package wikidata

import "strconv"

// Article represents a Wikipedia article with geodata and metadata.
// It maps to the SPARQL result fields.
type Article struct {
	QID       string   `json:"qid"`
	Title     string   `json:"title"`
	TitleEn   string   `json:"title_en,omitempty"`
	TitleUser string   `json:"title_user,omitempty"`
	Lat       float64  `json:"lat"`
	Lon       float64  `json:"lon"`
	Dist      float64  `json:"dist_m"`
	Instances []string `json:"instances"`
	Sitelinks int      `json:"sitelinks"`
	Category  string   `json:"category,omitempty"`
	Ignored   bool     `json:"ignored,omitempty"`
	Icon      string   `json:"icon,omitempty"`

	// Physical Dimensions (from Wikidata properties)
	Area   *float64 `json:"wd_area,omitempty"`
	Height *float64 `json:"wd_height,omitempty"`
	Length *float64 `json:"wd_length,omitempty"`
	Width  *float64 `json:"wd_width,omitempty"`

	// Derived
	DimensionMultiplier float64 `json:"dimension_multiplier,omitempty"`
}

// HexTile represents a single hexagonal grid cell.
type HexTile struct {
	Row int
	Col int
}

// Key returns the cache key for this tile.
// Format: wd_hex_{row}_{col}
func (h HexTile) Key() string {
	return "wd_hex_" + strconv.Itoa(h.Row) + "_" + strconv.Itoa(h.Col)
}

// EntityMetadata contains raw Wikidata entity data (Labels and Claims).
type EntityMetadata struct {
	Labels map[string]string
	Claims map[string][]string
}
