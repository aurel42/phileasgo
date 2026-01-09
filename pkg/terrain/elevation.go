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
