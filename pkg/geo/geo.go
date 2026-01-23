package geo

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"phileasgo/pkg/model"
)

// Point represents a geographic coordinate.
type Point struct {
	Lat float64
	Lon float64
}

// Distance calculates the Haversine distance between two points in meters.
func Distance(p1, p2 Point) float64 {
	const R = 6371000 // Earth radius in meters
	dLat := (p2.Lat - p1.Lat) * (math.Pi / 180.0)
	dLon := (p2.Lon - p1.Lon) * (math.Pi / 180.0)
	lat1 := p1.Lat * (math.Pi / 180.0)
	lat2 := p2.Lat * (math.Pi / 180.0)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(lat1)*math.Cos(lat2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// DestinationPoint calculates the destination point from a start point, given distance (in meters) and bearing (in degrees).
func DestinationPoint(start Point, distMeters, bearing float64) Point {
	const R = 6371000 // Earth radius in meters
	lat1 := start.Lat * (math.Pi / 180.0)
	lon1 := start.Lon * (math.Pi / 180.0)
	brng := bearing * (math.Pi / 180.0)

	lat2 := math.Asin(math.Sin(lat1)*math.Cos(distMeters/R) +
		math.Cos(lat1)*math.Sin(distMeters/R)*math.Cos(brng))
	lon2 := lon1 + math.Atan2(math.Sin(brng)*math.Sin(distMeters/R)*math.Cos(lat1),
		math.Cos(distMeters/R)-math.Sin(lat1)*math.Sin(lat2))

	return Point{
		Lat: lat2 * (180.0 / math.Pi),
		Lon: lon2 * (180.0 / math.Pi),
	}
}

// Bearing calculates the initial bearing (forward azimuth) from p1 to p2 in degrees.
func Bearing(p1, p2 Point) float64 {
	lat1 := p1.Lat * (math.Pi / 180.0)
	lat2 := p2.Lat * (math.Pi / 180.0)
	dLon := (p2.Lon - p1.Lon) * (math.Pi / 180.0)

	y := math.Sin(dLon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) -
		math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)
	brng := math.Atan2(y, x)

	return math.Mod(brng*(180.0/math.Pi)+360.0, 360.0)
}

// NormalizeAngle normalizes an angle difference to the range [-180, 180].
func NormalizeAngle(angleDeg float64) float64 {
	for angleDeg > 180 {
		angleDeg -= 360
	}
	for angleDeg < -180 {
		angleDeg += 360
	}
	return angleDeg
}

// City represents a city from cities1000.txt
type City struct {
	Name        string
	Lat         float64
	Lon         float64
	CountryCode string
	Admin1Code  string
	Admin1Name  string
}

// Service provides reverse geocoding.
type Service struct {
	grid map[int][]City
}

// NewService loads cities and builds the spatial index.
func NewService(citiesPath, admin1Path string) (*Service, error) {
	// 1. Load Admin1 Codes (Code -> Name)
	adminMap := make(map[string]string)
	adminFile, err := os.Open(admin1Path)
	if err == nil {
		scanner := bufio.NewScanner(adminFile)
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.Split(line, "\t")
			// format: code <tab> name <tab> nameAscii <tab> ...
			// We use name (index 1) which is UTF-8
			if len(parts) >= 2 {
				adminMap[parts[0]] = parts[1]
			}
		}
		adminFile.Close()

	}
	// 2. Load Cities
	file, err := os.Open(citiesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open cities file: %w", err)
	}
	defer file.Close()

	s := &Service{
		grid: make(map[int][]City),
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) < 19 {
			continue
		}

		// Parse Lat/Lon
		lat, _ := strconv.ParseFloat(parts[4], 64)
		lon, _ := strconv.ParseFloat(parts[5], 64)
		country := parts[8]
		admin1 := parts[10]

		city := City{
			Name:        parts[1],
			Lat:         lat,
			Lon:         lon,
			CountryCode: country,
			Admin1Code:  admin1,
			Admin1Name:  adminMap[country+"."+admin1], // Lookup full name
		}

		// Add to Grid
		key := s.getGridKey(lat, lon)
		s.grid[key] = append(s.grid[key], city)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return s, nil
}

// GetLocation returns the nearest city and country information.
func (s *Service) GetLocation(lat, lon float64) model.LocationInfo {
	// Search current cell and 8 neighbors
	originLatKey := int(math.Floor(lat))
	originLonKey := int(math.Floor(lon))

	var bestCity City
	minDistSq := math.MaxFloat64

	for dLat := -2; dLat <= 2; dLat++ {
		for dLon := -2; dLon <= 2; dLon++ {
			key := s.makeKey(originLatKey+dLat, originLonKey+dLon)
			cities, ok := s.grid[key]
			if !ok {
				continue
			}

			for _, city := range cities {
				// Euclidean distance squared is enough for comparison
				dLat := city.Lat - lat
				dLon := city.Lon - lon
				distSq := dLat*dLat + dLon*dLon

				if distSq < minDistSq {
					minDistSq = distSq
					bestCity = city
				}
			}
		}
	}

	if minDistSq == math.MaxFloat64 {
		return model.LocationInfo{
			CityName:    "Unknown",
			CountryCode: "US", // Default fallback
		}
	}

	return model.LocationInfo{
		CityName:    bestCity.Name,
		CountryCode: bestCity.CountryCode,
		Admin1Code:  bestCity.Admin1Code,
		Admin1Name:  bestCity.Admin1Name,
	}
}

// GetCountry returns the country code for the nearest city to the given coordinates.
func (s *Service) GetCountry(lat, lon float64) string {
	loc := s.GetLocation(lat, lon)
	return loc.CountryCode
}

func (s *Service) getGridKey(lat, lon float64) int {
	latKey := int(math.Floor(lat))
	lonKey := int(math.Floor(lon))
	return s.makeKey(latKey, lonKey)
}

func (s *Service) makeKey(lat, lon int) int {
	// Combine two ints into one.
	// Offset lat to be positive (Lat -90 to 90 -> 0 to 180)
	// Offset lon to be positive (Lon -180 to 180 -> 0 to 360)
	// Key = (Lat+90) * 360 + (Lon+180)
	return (lat+90)*360 + (lon + 180)
}
