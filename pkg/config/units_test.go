package config

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"10s", 10 * time.Second, false},
		{"1m", 1 * time.Minute, false},
		{"1.5h", 90 * time.Minute, false},
		{"1d", 24 * time.Hour, false},
		{"1w", 168 * time.Hour, false},
		{"2d2h", 50 * time.Hour, false},
		{"100ms", 100 * time.Millisecond, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseDuration(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.expected {
			t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestParseDistance(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
		wantErr  bool
	}{
		{"100m", 100, false},
		{"1.5km", 1500, false},
		{"1nm", 1852, false},
		{"500", 500, false}, // Unitless fallback
		{"10x", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseDistance(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseDistance(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.expected {
			t.Errorf("ParseDistance(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestYAMLUnmarshal(t *testing.T) {
	type TestConfig struct {
		Time Duration `yaml:"time"`
		Dist Distance `yaml:"dist"`
	}

	yamlData := `
time: 2d
dist: 5km
`
	var cfg TestConfig
	if err := yaml.Unmarshal([]byte(yamlData), &cfg); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if time.Duration(cfg.Time) != 48*time.Hour {
		t.Errorf("Expected 48h, got %v", time.Duration(cfg.Time))
	}
	if float64(cfg.Dist) != 5000 {
		t.Errorf("Expected 5000m, got %v", cfg.Dist)
	}
}
