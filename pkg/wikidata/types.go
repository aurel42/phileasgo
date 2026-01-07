package wikidata

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
	Label     string   `json:"label,omitempty"`
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

// HexTile represents a single H3 grid cell.
type HexTile struct {
	Index string
}

// Key returns the cache key for this tile.
// Format: wd_h3_{index}
func (h HexTile) Key() string {
	return "wd_h3_" + h.Index
}

// EntityMetadata contains raw Wikidata entity data (Labels and Claims).
type EntityMetadata struct {
	Labels map[string]string
	Claims map[string][]string
}
