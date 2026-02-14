package config

import (
	"fmt"
	"os"
	"sort"

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

// beaconsFile is the top-level YAML structure supporting an optional order array.
type beaconsFile struct {
	Order   []string                       `yaml:"order"`
	Entries map[string]BeaconRegistryEntry  `yaml:",inline"`
}

// LoadBeacons loads the beacon registry from a YAML file.
// Returns the registry map and an ordered list of color keys.
// If the YAML contains an "order" array, that order is used;
// otherwise keys are sorted alphabetically.
func LoadBeacons(path string) (BeaconRegistry, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read beacons config: %w", err)
	}

	var file beaconsFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, nil, fmt.Errorf("failed to parse beacons config: %w", err)
	}

	// Validation
	for name, entry := range file.Entries {
		if entry.Title == "" || entry.Livery == "" || entry.MapColor == "" {
			return nil, nil, fmt.Errorf("invalid beacon entry '%s': must have title, livery, and map_color", name)
		}
	}

	// Build ordered key list
	var keys []string
	if len(file.Order) > 0 {
		// Validate that all order entries exist in the registry
		for _, k := range file.Order {
			if _, ok := file.Entries[k]; !ok {
				return nil, nil, fmt.Errorf("order references unknown beacon '%s'", k)
			}
		}
		keys = file.Order
	} else {
		keys = make([]string, 0, len(file.Entries))
		for k := range file.Entries {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}

	return file.Entries, keys, nil
}
