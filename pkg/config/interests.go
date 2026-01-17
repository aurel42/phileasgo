package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// InterestsConfig holds interests configuration loaded from YAML.
type InterestsConfig struct {
	Interests []string `yaml:"interests"`
	Avoid     []string `yaml:"avoid"`
}

// LoadInterests loads interests from the given YAML file.
func LoadInterests(path string) (*InterestsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read interests config: %w", err)
	}

	var cfg InterestsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse interests config: %w", err)
	}

	return &cfg, nil
}
