package model

import (
	"time"
)

// SizeResolver defines the interface for dynamically determining POI size.
type SizeResolver interface {
	GetSize(category string) string
}

// POI represents a Point of Interest from Wikidata/Wikipedia.
// Note: The Size field is ephemeral and used primarily for UI/lookahead.
// Dynamic size resolution should be preferred via a SizeResolver.
type POI struct {
	WikidataID       string `json:"wikidata_id"`       // Primary Key
	Source           string `json:"source"`            // "wikidata"
	Category         string `json:"category"`          // e.g. "Landmark"
	SpecificCategory string `json:"specific_category"` // More precise label from Gemini (e.g. "Chalk Formation")
	Icon             string `json:"icon"`              // e.g. "castle.png"
	IconArtistic     string `json:"icon_artistic"`     // Override for artistic map

	// Coordinates
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`

	// Metadata
	Sitelinks int `json:"sitelinks"`

	// Display / Scoring Data
	NameEn    string `json:"name_en"`    // Canonical English Name
	NameLocal string `json:"name_local"` // Name in local language
	NameUser  string `json:"name_user"`  // Name in user's language (default en)

	WPURL           string `json:"wp_url"`            // URL of the *longest* article
	WPArticleLength int    `json:"wp_article_length"` // Length of the *longest* article
	ThumbnailURL    string `json:"thumbnail_url"`     // Wikipedia thumbnail URL (fetched on-demand)

	// Technical
	TriggerQID string    `json:"trigger_qid"`
	LastPlayed time.Time `json:"last_played"`
	CreatedAt  time.Time `json:"created_at"`

	// Scorer Data
	Size                string    `json:"size"`                 // S, M, L, XL
	DimensionMultiplier float64   `json:"dimension_multiplier"` // Multiplier from physical dimensions
	Score               float64   `json:"score"`                // Intrinsic score (content-based, position-agnostic)
	ScoreDetails        string    `json:"score_details"`        // Explainer for debug
	IsVisible           bool      `json:"is_visible"`
	IsHiddenFeature     bool      `json:"is_hidden_feature"`
	IsDeferred          bool      `json:"is_deferred"` // Hard filter: don't pick this POI now
	Visibility          float64   `json:"visibility"`  // Raw visibility score (0.0 - 1.0+)
	Badges              []string  `json:"badges"`      // Ephemeral state (deferred, msfs, etc.)
	LOSStatus           LOSStatus `json:"los_status"`  // 0=unknown, 1=visible, 2=blocked
	// MSFS
	IsMSFSPOI bool `json:"is_msfs_poi"`
	// Narration
	NarrationStrategy string  `json:"narration_strategy"` // uniform, min_skew, max_skew
	BeaconColor       string  `json:"beacon_color"`       // Assigned color for the beacon
	TimeToBehind      float64 `json:"time_to_behind"`     // Seconds until it leaves forward view
	TimeToCPA         float64 `json:"time_to_cpa"`        // Seconds until closest point of approach
	InCooldown        bool    `json:"is_on_cooldown"`     // Calculated on-the-fly for UI

	// Session persistence (in-memory only)
	Script string `json:"-"`

	// River Context (transient, not persisted)
	RiverContext *RiverContext `json:"-"`

	// Deprecated/Removed fields: ID (int), ArticleIDs
}

// LOSStatus represents the Line-of-Sight status of a POI.
type LOSStatus int

const (
	LOSUnknown LOSStatus = iota // 0 (Default)
	LOSVisible                  // 1
	LOSBlocked                  // 2
)

// RiverContext holds ephemeral state for rivers detected by the Sentinel.
type RiverContext struct {
	IsActive      bool    // True if this POI is currently the "active" river near the aircraft
	IsFlyingAlong bool    // True if aircraft is roughly following the river course
	Direction     string  // "upstream", "downstream", "crossing"
	DistanceM     float64 // Lateral distance in meters
	ClosestLat    float64 // Dynamic closest point (for UI)
	ClosestLon    float64
}

// RiverCandidate represents a potential river match found by the Sentinel.
type RiverCandidate struct {
	Name       string
	WikidataID string
	ClosestLat float64
	ClosestLon float64
	Distance   float64 // meters
	IsAhead    bool
	MouthLat   float64
	MouthLon   float64
	SourceLat  float64
	SourceLon  float64
}

// LocationInfo represents rich geographic context.
type LocationInfo struct {
	CityName    string `json:"city_name"`
	CountryCode string `json:"country_code"` // Legal (from boundary maps)
	CountryName string `json:"country_name"` // Legal (from boundary maps)
	Admin1Code  string `json:"admin1_code"`
	Admin1Name  string `json:"admin1_name"`
	RegionName  string `json:"region_name"` // For future use
	Zone        string `json:"zone"`        // "land", "territorial", "eez", "international"

	// Nearest City Context (if different from Legal)
	CityCountryCode string `json:"city_country_code,omitempty"`
	CityCountryName string `json:"city_country_name,omitempty"`
	CityAdmin1Name  string `json:"city_admin1_name,omitempty"`
}

// DisplayName returns the best available name for the POI.
// Priority: NameUser > NameEn > NameLocal > WikidataID
func (p *POI) DisplayName() string {
	if p.NameUser != "" {
		return p.NameUser
	}
	if p.NameEn != "" {
		return p.NameEn
	}
	if p.NameLocal != "" {
		return p.NameLocal
	}
	return p.WikidataID
}

// IsOnCooldown returns true if the POI was played recently and is still within the cooldown period.
func (p *POI) IsOnCooldown(ttl time.Duration) bool {
	if p.LastPlayed.IsZero() {
		return false
	}
	return time.Since(p.LastPlayed) < ttl
}

// MSFSPOI represents a POI from Microsoft Flight Simulator.
type MSFSPOI struct {
	ID        int64   `json:"id"`
	Type      string  `json:"type"`
	Name      string  `json:"name"`
	Ident     string  `json:"ident"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Elevation float64 `json:"elevation"`
}

// WikidataHierarchy represents the parent structure of an entity.
type WikidataHierarchy struct {
	QID       string    `json:"qid"`
	Name      string    `json:"name"`
	Parents   []string  `json:"parents"` // Stored as JSON list of QIDs
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Article represents a Wikipedia article.
type Article struct {
	UUID         string            `json:"uuid"`
	Title        string            `json:"title"`
	URL          string            `json:"url"`
	Names        map[string]string `json:"names"` // JSON: {"en": "...", "fr": "..."}
	Text         string            `json:"text"`
	Lengths      map[string]int    `json:"lengths"` // JSON: {"en": 100, "fr": 120}
	ThumbnailURL string            `json:"thumbnail_url"`
	CreatedAt    time.Time         `json:"created_at"`
}

// ClassificationResult represents the outcome of a classification.
type ClassificationResult struct {
	Category string `json:"category"`
	Size     string `json:"size"`
	Ignored  bool   `json:"ignored"` // True = article should be dropped (in ignored_categories)
}

// TripEvent represents a structured event in the flight log.
type TripEvent struct {
	Timestamp time.Time         `json:"timestamp"`
	Type      string            `json:"type"`               // "transition", "narration", "activity"
	Category  NarrativeType     `json:"category,omitempty"` // for narrations
	Title     string            `json:"title,omitempty"`
	Summary   string            `json:"summary,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Lat       float64           `json:"lat,omitempty"`
	Lon       float64           `json:"lon,omitempty"`
}
