package rivers

import (
	_ "embed"
)

//go:embed data/rivers.geojson
var RiversGeoJSON []byte
