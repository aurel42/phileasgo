package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// BeaconRegistryEntry defines a single beacon's livery and color mapping.
type BeaconRegistryEntry struct {
	Title    string `yaml:"title"`
	Livery   string `yaml:"livery"`
	MapColor string `yaml:"map_color"`
}

// BeaconRegistry is a map of color name (e.g. "red") to its details.
type BeaconRegistry map[string]BeaconRegistryEntry

// LoadBeacons loads the beacon registry from a YAML file.
func LoadBeacons(path string) (BeaconRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read beacons config: %w", err)
	}

	var registry BeaconRegistry
	if err := yaml.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("failed to parse beacons config: %w", err)
	}

	// Validation
	for name, entry := range registry {
		if entry.Title == "" || entry.Livery == "" || entry.MapColor == "" {
			return nil, fmt.Errorf("invalid beacon entry '%s': must have title, livery, and map_color", name)
		}
	}

	return registry, nil
}
