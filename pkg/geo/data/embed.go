package data

import (
	_ "embed"
)

//go:embed geodata.bin
var GeoData []byte

//go:embed marine.geojson
var MarineGeoJSON []byte

//go:embed regions.geojson
var RegionsGeoJSON []byte
