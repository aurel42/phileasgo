// Script to strip unused properties from Natural Earth GeoJSON.
// Keeps only: ISO_A2, ISO_A2_EH, NAME, and geometry.
// This dramatically reduces file size for embedding.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

type Feature struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Geometry   json.RawMessage        `json:"geometry"`
}

type SlimFeature struct {
	Type       string          `json:"type"`
	Properties SlimProperties  `json:"properties"`
	Geometry   json.RawMessage `json:"geometry"`
}

type SlimProperties struct {
	Name    string `json:"NAME"`
	ISOA2   string `json:"ISO_A2"`
	ISOA2EH string `json:"ISO_A2_EH"`
}

type SlimFeatureCollection struct {
	Type     string        `json:"type"`
	Features []SlimFeature `json:"features"`
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input.geojson> <output.geojson>\n", os.Args[0])
		os.Exit(1)
	}

	inputPath := os.Args[1]
	outputPath := os.Args[2]

	// Read input
	data, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read input: %v\n", err)
		os.Exit(1)
	}

	var fc FeatureCollection
	if err := json.Unmarshal(data, &fc); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse GeoJSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Input: %d features, %d bytes\n", len(fc.Features), len(data))

	// Build slim version
	slim := SlimFeatureCollection{
		Type:     "FeatureCollection",
		Features: make([]SlimFeature, len(fc.Features)),
	}

	for i, f := range fc.Features {
		slim.Features[i] = SlimFeature{
			Type:     "Feature",
			Geometry: f.Geometry,
			Properties: SlimProperties{
				Name:    getString(f.Properties, "NAME"),
				ISOA2:   getString(f.Properties, "ISO_A2"),
				ISOA2EH: getString(f.Properties, "ISO_A2_EH"),
			},
		}
	}

	// Write output (compact JSON)
	outData, err := json.Marshal(slim)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outputPath, outData, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write output: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Output: %d features, %d bytes (%.1f%% reduction)\n",
		len(slim.Features), len(outData), 100*(1-float64(len(outData))/float64(len(data))))
}

func getString(props map[string]interface{}, key string) string {
	if val, ok := props[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}
