package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"io"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Paths
var (
	citiesPath = flag.String("cities", "data/cities1000.txt", "Path to cities1000.txt")
	adminPath  = flag.String("admin", "data/admin1CodesASCII.txt", "Path to admin1CodesASCII.txt")
	outPath    = flag.String("out", "pkg/geo/data/geodata.bin", "Output binary file")
)

// City represents the data we want to keep
type City struct {
	Name        string
	Lat         float32 // float32 is enough for cities (precision ~111m / 65000 -> meters) actually float32 has 7 decimal digits, ample.
	Lon         float32
	CountryCode string
	Admin1Code  string
	Population  int32
}

func main() {
	flag.Parse()

	log.Println("Loading Admin1 Codes...")
	adminMap, err := loadAdmin1(*adminPath)
	if err != nil {
		log.Fatalf("Failed to load admin codes: %v", err)
	}
	log.Printf("Loaded %d admin codes", len(adminMap))

	log.Println("Loading Cities...")
	cities, grid, err := loadCities(*citiesPath)
	if err != nil {
		log.Fatalf("Failed to load cities: %v", err)
	}
	log.Printf("Loaded %d cities in %d grid cells", len(cities), len(grid))

	log.Println("Writing binary data...")
	if err := writeBinary(*outPath, adminMap, grid); err != nil {
		log.Fatalf("Failed to write binary: %v", err)
	}
	log.Println("Done!")
}

func loadAdmin1(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) >= 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m, scanner.Err()
}

func loadCities(path string) ([]City, map[int][]City, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var allCities []City
	grid := make(map[int][]City)
	scanner := bufio.NewScanner(f)

	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) < 19 {
			continue
		}

		lat, _ := strconv.ParseFloat(parts[4], 64)
		lon, _ := strconv.ParseFloat(parts[5], 64)
		pop, _ := strconv.Atoi(parts[14])

		c := City{
			Name:        parts[1],
			Lat:         float32(lat),
			Lon:         float32(lon),
			CountryCode: parts[8],
			Admin1Code:  parts[10],
			Population:  int32(pop),
		}

		allCities = append(allCities, c)

		key := getGridKey(lat, lon)
		grid[key] = append(grid[key], c)
		count++
	}

	return allCities, grid, scanner.Err()
}

func getGridKey(lat, lon float64) int {
	latKey := int(math.Floor(lat))
	lonKey := int(math.Floor(lon))
	return (latKey+90)*360 + (lonKey + 180)
}

func writeBinary(path string, adminMap map[string]string, grid map[int][]City) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	if err := writeHeader(w); err != nil {
		return err
	}

	if err := writeAdminSection(w, adminMap); err != nil {
		return err
	}

	// 3. Grid Index Placeholder
	w.Flush()
	idxStart, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	// Get grid keys sorted
	var gridKeys []int
	for k := range grid {
		gridKeys = append(gridKeys, k)
	}
	sort.Ints(gridKeys)

	// Write placeholder for Count
	if err := binary.Write(f, binary.LittleEndian, uint32(len(gridKeys))); err != nil {
		return err
	}

	// Calculate size of one index entry: 4 (key) + 4 (offset) + 2 (count) = 10 bytes
	indexSize := int64(len(gridKeys)) * 10
	citiesStart := idxStart + 4 + indexSize

	// Seek past index
	if _, err := f.Seek(citiesStart, io.SeekStart); err != nil {
		return err
	}
	w.Reset(f)

	// 4. Cities Data
	gridOffsets, err := writeCitiesSection(w, f, grid, gridKeys, citiesStart)
	if err != nil {
		return err
	}
	w.Flush()

	// 5. Fill Index
	if err := fillGridIndex(f, idxStart, grid, gridKeys, gridOffsets); err != nil {
		return err
	}

	return nil
}

func writeHeader(w *bufio.Writer) error {
	// Magic: PHGO (4 bytes)
	// Version: 1 (uint16)
	if _, err := w.WriteString("PHGO"); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	return nil
}

func writeAdminSection(w *bufio.Writer, adminMap map[string]string) error {
	// Count (uint32)
	// [KeyLen(u8)][Key...][NameLen(u8)][Name...]
	// Sorting keys for deterministic output
	var adminKeys []string
	for k := range adminMap {
		adminKeys = append(adminKeys, k)
	}
	sort.Strings(adminKeys)

	if err := binary.Write(w, binary.LittleEndian, uint32(len(adminKeys))); err != nil {
		return err
	}

	// We need to store offsets? No, we load the whole map into memory at startup.
	// It's small (3800 entries).
	for _, k := range adminKeys {
		if err := writeString(w, k); err != nil {
			return err
		}
		if err := writeString(w, adminMap[k]); err != nil {
			return err
		}
	}
	return nil
}

func writeCitiesSection(w *bufio.Writer, f *os.File, grid map[int][]City, gridKeys []int, citiesStart int64) (map[int]uint32, error) {
	// Track offsets
	gridOffsets := make(map[int]uint32)

	for _, k := range gridKeys {
		// Verify we are where we think we are (relative to citiesStart)

		// Bufio writer might have buffered data, so "pos" is only accurate if flushed.
		// However, we are writing linearly. We can track bytes written.
		// A safer way: Flush before recording offset.
		w.Flush()
		currentPos, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, err
		}

		gridOffsets[k] = uint32(currentPos - citiesStart)

		cities := grid[k]
		for _, c := range cities {
			// Lat/Lon (float32)
			if err := binary.Write(w, binary.LittleEndian, c.Lat); err != nil {
				return nil, err
			}
			if err := binary.Write(w, binary.LittleEndian, c.Lon); err != nil {
				return nil, err
			}
			// Pop (int32)
			if err := binary.Write(w, binary.LittleEndian, c.Population); err != nil {
				return nil, err
			}

			// Country (2 bytes fixed)
			cc := c.CountryCode
			if len(cc) > 2 {
				cc = cc[:2]
			}
			if len(cc) < 2 {
				cc += "  "
			} // padding
			if _, err := w.WriteString(cc[:2]); err != nil {
				return nil, err
			}

			// Admin1Code (string)
			if err := writeString(w, c.Admin1Code); err != nil {
				return nil, err
			}
			// Name (string)
			if err := writeString(w, c.Name); err != nil {
				return nil, err
			}
		}
	}
	return gridOffsets, nil
}

func fillGridIndex(f *os.File, idxStart int64, grid map[int][]City, gridKeys []int, gridOffsets map[int]uint32) error {
	// [GridKey(int32)][Offset(uint32)][Count(uint16)]
	if _, err := f.Seek(idxStart+4, io.SeekStart); err != nil {
		return err
	} // Skip count

	for _, k := range gridKeys {
		offset := gridOffsets[k]
		count := len(grid[k])
		// Check overflow
		if count > 65535 {
			log.Printf("Warning: Grid cell %d has %d cities, truncating to 65535", k, count)
			count = 65535
		}

		if err := binary.Write(f, binary.LittleEndian, int32(k)); err != nil {
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, offset); err != nil {
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, uint16(count)); err != nil {
			return err
		}
	}
	return nil
}

func writeString(w io.Writer, s string) error {
	b := []byte(s)
	if len(b) > 255 {
		b = b[:255]
	}
	if _, err := w.Write([]byte{uint8(len(b))}); err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	return nil
}
