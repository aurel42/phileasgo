package data

import (
	_ "embed"
)

//go:embed geodata.bin
var GeoData []byte
