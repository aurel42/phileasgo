// Script to strip unused properties from Natural Earth GeoJSON.
// Keeps only: NAME, QID, CATEGORY, and ISO codes.
// Filters out features without a QID (or ISO code for countries).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
	Name     string `json:"name"`
	QID      string `json:"qid,omitempty"`
	Category string `json:"category,omitempty"`
	ISOA2    string `json:"iso_a2,omitempty"`
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

	var slimFeatures []SlimFeature
	for _, f := range fc.Features {
		name := getAny(f.Properties, "NAME", "name")
		qid := getAny(f.Properties, "WIKIDATAID", "wikidataid", "QID", "qid")
		category := getAny(f.Properties, "FEATURECLA", "featurecla")
		iso := getAny(f.Properties, "ISO_A2", "iso_a2")

		// Filter: Must have QID (or ISO code if it's a country)
		if qid == "" && iso == "" {
			continue
		}

		slimFeatures = append(slimFeatures, SlimFeature{
			Type:     "Feature",
			Geometry: f.Geometry,
			Properties: SlimProperties{
				Name:     name,
				QID:      qid,
				Category: category,
				ISOA2:    iso,
			},
		})
	}

	slim := SlimFeatureCollection{
		Type:     "FeatureCollection",
		Features: slimFeatures,
	}

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

func getAny(props map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := props[key]; ok {
			if s, ok := val.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" && s != "-99" {
					return s
				}
			}
		}
	}
	return ""
}
