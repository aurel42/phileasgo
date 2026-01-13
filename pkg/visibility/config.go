package visibility

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SizeType defines the size category of a POI
type SizeType string

const (
	SizeS  SizeType = "S"
	SizeM  SizeType = "M"
	SizeL  SizeType = "L"
	SizeXL SizeType = "XL"
)

// AltitudeRow defines visibility for a specific altitude
type AltitudeRow struct {
	AltAGL    float64              `yaml:"altitude"`
	Distances map[SizeType]float64 `yaml:"distances"`
}

// Config holds the visibility configuration
type Config struct {
	Visibility struct {
		Table []AltitudeRow `yaml:"table"`
	} `yaml:"visibility"`
}

// Manager handles loading and querying visibility configuration
type Manager struct {
	table []AltitudeRow
}

// NewManager creates a new Manager from a config file
func NewManager(path string) (*Manager, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read visibility config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse visibility config: %w", err)
	}

	return &Manager{
		table: cfg.Visibility.Table,
	}, nil
}

// NewManagerForTest creates a manager with injected table data for testing.
func NewManagerForTest(table []AltitudeRow) *Manager {
	return &Manager{
		table: table,
	}
}

// GetMaxVisibleDist returns the maximum visible distance for a given altitude and size.
// Interpolates between altitude levels and applies the dynamic boost factor.
func (m *Manager) GetMaxVisibleDist(altAGL float64, size SizeType, boostFactor float64) float64 {
	if len(m.table) == 0 {
		return 0
	}

	// 1. Find bracketing rows
	var lower, upper *AltitudeRow
	for i := range m.table {
		if m.table[i].AltAGL <= altAGL {
			lower = &m.table[i]
		}
		if m.table[i].AltAGL >= altAGL {
			upper = &m.table[i]
			break
		}
	}

	// Handle out of bounds
	// Handle out of bounds
	var baseDist float64
	if lower == nil {
		baseDist = getDist(m.table[0].Distances, size)
	} else if upper == nil {
		baseDist = getDist(m.table[len(m.table)-1].Distances, size)
	} else if lower == upper {
		baseDist = getDist(lower.Distances, size)
	} else {
		// Interpolate
		ratio := (altAGL - lower.AltAGL) / (upper.AltAGL - lower.AltAGL)
		d1 := getDist(lower.Distances, size)
		d2 := getDist(upper.Distances, size)
		baseDist = d1 + (d2-d1)*ratio
	}

	return baseDist * boostFactor
}

func getDist(distances map[SizeType]float64, size SizeType) float64 {
	if d, ok := distances[size]; ok {
		return d
	}
	// Fallback logic if size missing? Assume M or 0
	return distances[SizeM]
}
