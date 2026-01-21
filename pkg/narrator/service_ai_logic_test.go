package narrator

import (
	"strings"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

func TestCalculateNavInstruction(t *testing.T) {
	// Helper to create mocked service
	makeSvc := func(units string) *AIService {
		return &AIService{
			cfg: &config.Config{
				Narrator: config.NarratorConfig{
					Units: units,
				},
			},
		}
	}

	// 4.5km = 4500m
	// 4.6km = 4600m
	// 4.4km = 4400m

	tests := []struct {
		name           string
		units          string
		userLat        float64
		userLon        float64
		userHdg        float64 // 0 = North
		poiLat         float64 // POI relative to user
		poiLon         float64
		onGround       bool
		expectContains []string
		expectEmpty    bool
	}{
		// GROUND RULES
		{
			name:    "Ground < 4.5km (Silence)",
			units:   "metric",
			userLat: 0, userLon: 0,
			poiLat: 0.03, poiLon: 0, // Approx 3.3km North
			onGround:    true,
			expectEmpty: true,
		},
		{
			name:    "Ground >= 4.5km (Cardinal + Dist)",
			units:   "metric",
			userLat: 0, userLon: 0,
			poiLat: 0.05, poiLon: 0, // Approx 5.5km North
			onGround:       true,
			expectContains: []string{"To the North", "about 6 kilometers"},
		},
		{
			name:    "Ground >= 4.5km (Imperial)",
			units:   "imperial",
			userLat: 0, userLon: 0,
			poiLat: 0.05, poiLon: 0, // Approx 5.5km
			onGround:       true,
			expectContains: []string{"To the North", "about 3 miles"},
		},

		// AIRBORNE RULES (< 4.5km) -> Relative, No Dist
		{
			name:    "Airborne < 4.5km (Straight Ahead)",
			units:   "metric",
			userLat: 0, userLon: 0, userHdg: 0, // Heading North
			poiLat: 0.03, poiLon: 0, // 3.3km North
			onGround:       false,
			expectContains: []string{"Straight ahead"},
			// MUST NOT contain distance
			expectEmpty: false,
		},
		{
			name:    "Airborne < 4.5km (On Right)",
			units:   "metric",
			userLat: 0, userLon: 0, userHdg: 0,
			poiLat: 0, poiLon: 0.03, // 3.3km East (Bearing 90)
			onGround:       false,
			expectContains: []string{"On your right"},
		},
		{
			name:    "Airborne < 4.5km (On Left)",
			units:   "metric",
			userLat: 0, userLon: 0, userHdg: 0,
			poiLat: 0, poiLon: -0.03, // 3.3km West (Bearing 270)
			onGround:       false,
			expectContains: []string{"On your left"},
		},

		// AIRBORNE RULES (>= 4.5km) -> Clock + Dist
		{
			name:    "Airborne >= 4.5km (Clock 12)",
			units:   "metric",
			userLat: 0, userLon: 0, userHdg: 0,
			poiLat: 0.05, poiLon: 0, // 5.5km North
			onGround:       false,
			expectContains: []string{"At your 12 o'clock", "about 6 kilometers"},
		},
		{
			name:    "Airborne >= 4.5km (Clock 3)",
			units:   "metric",
			userLat: 0, userLon: 0, userHdg: 0,
			poiLat: 0, poiLon: 0.05, // 5.5km East
			onGround:       false,
			expectContains: []string{"At your 3 o'clock", "about 6 kilometers"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := makeSvc(tt.units)
			poi := &model.POI{Lat: tt.poiLat, Lon: tt.poiLon}
			tel := &sim.Telemetry{
				Latitude:   tt.userLat,
				Longitude:  tt.userLon,
				Heading:    tt.userHdg,
				IsOnGround: tt.onGround,
			}

			// Pre-calc distance to verify test assumption
			p1 := geo.Point{Lat: tt.userLat, Lon: tt.userLon}
			p2 := geo.Point{Lat: tt.poiLat, Lon: tt.poiLon}
			dist := geo.Distance(p1, p2) / 1000.0
			// Sanity check valid test cases
			if strings.Contains(tt.name, "< 4.5") && dist >= 4.5 {
				t.Fatalf("Test setup error: distance %.2f km is not < 4.5km", dist)
			}
			if strings.Contains(tt.name, ">= 4.5") && dist < 4.5 {
				t.Fatalf("Test setup error: distance %.2f km is not >= 4.5km", dist)
			}

			got := svc.calculateNavInstruction(poi, tel)

			if tt.expectEmpty {
				if got != "" {
					t.Errorf("Expected empty instruction, got: %q", got)
				}
				return
			}

			if got == "" {
				t.Errorf("Expected instruction, got empty")
			}

			for _, want := range tt.expectContains {
				if !strings.Contains(got, want) {
					t.Errorf("Result %q missing substring %q", got, want)
				}
			}

			// Negative checks for specific logic rules
			if strings.Contains(tt.name, "< 4.5") && !tt.onGround {
				// Should NO dist
				if strings.Contains(got, "kilometer") || strings.Contains(got, "mile") {
					t.Errorf("Result %q should NOT contain distance info for airborne < 4.5km", got)
				}
			}
		})
	}
}

func TestHumanRound(t *testing.T) {
	tests := []struct {
		input float64
		want  float64
	}{
		{4.2, 4.0},
		{4.7, 5.0},
		{9.4, 9.0},
		{11.0, 10.0}, // 11 -> 10 (nearest 5)
		{12.4, 10.0},
		{12.6, 15.0}, // 12.6 -> 15 (nearest 5)
		{23.0, 25.0},
		{98.0, 100.0},
		{102.0, 100.0},
		{106.0, 110.0},
		{123.0, 120.0},
	}

	for _, tt := range tests {
		got := humanRound(tt.input)
		if got != tt.want {
			t.Errorf("humanRound(%.1f) = %.1f, want %.1f", tt.input, got, tt.want)
		}
	}
}

func TestCapitalizeStart(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "Hello"},
		{"World", "World"},
		{"", ""},
		{"a", "A"},
		{"123", "123"},
	}

	for _, tt := range tests {
		got := capitalizeStart(tt.input)
		if got != tt.want {
			t.Errorf("capitalizeStart(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
