package terrain

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

const (
	// ETOPO1 Constants (cell-registered: 10801 rows × 21601 cols)
	etopo1Rows = 10801
	etopo1Cols = 21601
	etopo1Size = etopo1Rows * etopo1Cols * 2 // 16-bit signed integers
)

// ElevationGetter defines the efficient retrieval of terrain elevation.
type ElevationGetter interface {
	GetElevation(lat, lon float64) (int16, error)
	GetLowestElevation(lat, lon, radiusKM float64) (int16, error)
}

// ElevationProvider reads elevation data from ETOPO1.
type ElevationProvider struct {
	file *os.File
}

// NewElevationProvider opens the ETOPO1 binary file.
func NewElevationProvider(path string) (*ElevationProvider, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	if info.Size() != int64(etopo1Size) {
		f.Close()
		return nil, fmt.Errorf("invalid ETOPO1 file size: expected %d, got %d", etopo1Size, info.Size())
	}

	return &ElevationProvider{
		file: f,
	}, nil
}

// Close closes the file handle.
func (e *ElevationProvider) Close() error {
	return e.file.Close()
}

// GetElevation returns the elevation in meters at the given lat/lon.
func (e *ElevationProvider) GetElevation(lat, lon float64) (int16, error) {
	if lat > 90 || lat < -90 || lon > 180 || lon < -180 {
		return 0, fmt.Errorf("coordinates out of bounds: %f, %f", lat, lon)
	}

	row := int(math.Round((90.0 - lat) * 60.0))
	col := int(math.Round((lon + 180.0) * 60.0))

	if row < 0 {
		row = 0
	}
	if row >= etopo1Rows {
		row = etopo1Rows - 1
	}
	if col < 0 {
		col = 0
	}
	if col >= etopo1Cols {
		col %= etopo1Cols
	}

	offset := int64(row*etopo1Cols+col) * 2

	b := make([]byte, 2)
	_, err := e.file.ReadAt(b, offset)
	if err != nil {
		return 0, err
	}

	val := int16(binary.LittleEndian.Uint16(b))
	return val, nil
}

// GetLowestElevation returns the minimum altitude (meters) within radiusKM.
// It uses an efficient buffered scanning approach to minimize I/O operations.
func (e *ElevationProvider) GetLowestElevation(lat, lon, radiusKM float64) (int16, error) {
	if radiusKM < 0 {
		return 0, fmt.Errorf("negative radius")
	}

	// 1. Calculate Grid Bounds
	// ETOPO1 resolution is 1 arc-minute.
	// 1 arc-minute latitude approx 1.852 km (1 NM).
	radiusNM := radiusKM / 1.852
	radiusRows := int(math.Ceil(radiusNM))

	// Adjust for longitude convergence: radiusCols = radiusRows / cos(lat)
	// We use the maximum latitude in the range to be safe (closest to pole = widest relative longitude coverage needed? No, cos(90)=0, 1/0=inf.
	// Actually, we need to cover a physical distance. At high latitudes, 1 degree longitude is very short.
	// So we need MORE columns to cover the same KM distance.
	// radiusCols = radiusRows / cos(lat) correct.
	// We should use the latitude closest to the pole for the worst-case width calc, or just center lat?
	// Center lat is good enough as long as we aren't literally at the pole.
	// We clamp cosLat to avoid division by zero.
	cosLat := math.Cos(lat * math.Pi / 180.0)
	if math.Abs(cosLat) < 0.01 {
		cosLat = 0.01
	}
	radiusCols := int(math.Ceil(float64(radiusRows) / cosLat))

	centerRow := int(math.Round((90.0 - lat) * 60.0))
	centerCol := int(math.Round((lon + 180.0) * 60.0))

	minElev := int16(math.MaxInt16)
	startRow, endRow := centerRow-radiusRows, centerRow+radiusRows
	startCol, endCol := centerCol-radiusCols, centerCol+radiusCols
	width := endCol - startCol + 1

	// 2. Scan Grid
	// We scan row by row.
	for r := startRow; r <= endRow; r++ {
		// Clamp Latitude (if we go off the map north/south, we just clamp to edge)
		row := r
		if row < 0 {
			row = 0
		} else if row >= etopo1Rows {
			row = etopo1Rows - 1
		}

		// Scan row (handling wrap-around)
		if err := e.scanRowSegment(row, startCol, width, &minElev); err != nil {
			return 0, err
		}
	}

	if minElev == math.MaxInt16 {
		// Fallback if somehow nothing was scanned (e.g. 0 radius)
		var err error
		minElev, err = e.GetElevation(lat, lon)
		if err != nil {
			return 0, err
		}
	}

	// Cap at MSL (0). We don't care about underwater or below-sea-level depressions.
	if minElev < 0 {
		return 0, nil
	}
	return minElev, nil
}

// scanRowSegment scans a portion of a row, handling longitude wrapping (Date Line).
func (e *ElevationProvider) scanRowSegment(row, startCol, width int, minElev *int16) error {
	// Normalize start column to [0, etopo1Cols)
	// Initial col could be negative (e.g. -10) -> wraps to end
	// or > cols.
	// Go's % operator returning negative for negative numbers: -10 % 21601 = -10.
	// So we need proper modulus.
	normStart := (startCol%etopo1Cols + etopo1Cols) % etopo1Cols

	if normStart+width <= etopo1Cols {
		// Contiguous read (no wrapping needed for this segment)
		return e.scanChunk(row, normStart, width, minElev)
	}

	// Wrap around the Date Line
	firstLen := etopo1Cols - normStart // Read until the end of the row
	remaining := width - firstLen      // Remainder to read from start of row

	if err := e.scanChunk(row, normStart, firstLen, minElev); err != nil {
		return err
	}
	return e.scanChunk(row, 0, remaining, minElev)
}

// scanChunk reads a contiguous chunk of elevation data from the file.
func (e *ElevationProvider) scanChunk(row, colStart, count int, minElev *int16) error {
	if count <= 0 {
		return nil
	}
	offset := int64(row*etopo1Cols+colStart) * 2
	// Buffer size: 2 bytes per sample
	b := make([]byte, count*2)

	if _, err := e.file.ReadAt(b, offset); err != nil {
		return err
	}

	// Iterate over the buffer and find min
	for i := 0; i < count; i++ {
		val := int16(binary.LittleEndian.Uint16(b[i*2 : i*2+2]))
		if val < *minElev {
			*minElev = val
		}
	}
	return nil
}
