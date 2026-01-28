package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jonas-p/go-shp"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
)

func main() {
	inputPath := flag.String("input", "", "Path to input .shp file")
	outputPath := flag.String("output", "", "Path to output .geojson file")
	flag.Parse()

	if *inputPath == "" || *outputPath == "" {
		flag.Usage()
		log.Fatal("Input and output paths are required")
	}

	if err := run(*inputPath, *outputPath); err != nil {
		log.Fatal(err)
	}
}

func run(inputPath, outputPath string) error {
	// Open Shapefile
	shape, err := shp.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open shapefile: %w", err)
	}
	defer shape.Close()

	// Prepare fields
	fields := shape.Fields()
	fieldNames := make([]string, len(fields))
	for i, f := range fields {
		fieldNames[i] = f.String()
	}

	fc := geojson.NewFeatureCollection()

	// iterate through all shapes
	for shape.Next() {
		n, p := shape.Shape()

		var geometry orb.Geometry

		switch s := p.(type) {
		case *shp.Null:
			continue
		case *shp.PolyLine:
			geometry = convertPolyLine(s)
		case *shp.Polygon:
			geometry = convertPolygon(s)
		case *shp.Point:
			geometry = orb.Point{s.X, s.Y}
		default:
			log.Printf("Skipping unsupported shape type: %T", p)
			continue
		}

		f := geojson.NewFeature(geometry)

		// Read attributes
		for i, name := range fieldNames {
			val := shape.ReadAttribute(n, i)
			f.Properties[name] = val
		}

		fc.Append(f)
	}

	if err := shape.Err(); err != nil {
		return fmt.Errorf("error iterating shapes: %w", err)
	}

	data, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal GeoJSON: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("Successfully converted %d features to %s\n", len(fc.Features), outputPath)
	return nil
}

func convertPolyLine(s *shp.PolyLine) orb.MultiLineString {
	var multiline orb.MultiLineString

	for i := 0; i < int(s.NumParts); i++ {
		start := s.Parts[i]
		end := s.NumPoints
		if i < int(s.NumParts)-1 {
			end = s.Parts[i+1]
		}

		var line orb.LineString
		for j := start; j < end; j++ {
			line = append(line, orb.Point{s.Points[j].X, s.Points[j].Y})
		}
		multiline = append(multiline, line)
	}
	return multiline
}

func convertPolygon(s *shp.Polygon) orb.Polygon {
	// Simple conversion treating all parts as rings of a single polygon
	var poly orb.Polygon

	for i := 0; i < int(s.NumParts); i++ {
		start := s.Parts[i]
		end := s.NumPoints
		if i < int(s.NumParts)-1 {
			end = s.Parts[i+1]
		}

		var ring orb.Ring
		for j := start; j < end; j++ {
			ring = append(ring, orb.Point{s.Points[j].X, s.Points[j].Y})
		}
		poly = append(poly, ring)
	}
	return poly
}
