package narrator

import (
	"context"

	"phileasgo/pkg/model"
)

// POIProvider defines the interface for POI management.
type POIProvider interface {
	GetPOI(ctx context.Context, qid string) (*model.POI, error)
	GetBestCandidate() *model.POI
	GetCandidates(limit int) []*model.POI
	CountScoredAbove(threshold float64, limit int) int
	LastScoredPosition() (lat, lon float64)
}

// GeoProvider defines the interface for geographic services.
type GeoProvider interface {
	GetCountry(lat, lon float64) string
	GetLocation(lat, lon float64) model.LocationInfo
}

// WikipediaProvider defines the interface for Wikipedia access.
type WikipediaProvider interface {
	GetArticleContent(ctx context.Context, title, lang string) (string, error)
}

// LanguageResolver defines the interface for resolving language details.
type LanguageResolver interface {
	GetLanguageInfo(countryCode string) model.LanguageInfo
}

// BeaconProvider defines the interface for beacon/marker management.
type BeaconProvider interface {
	SetTarget(ctx context.Context, lat, lon float64) error
	Clear()
}

// AudioProvider alias for audio.Service to keep imports clean or use direct import.
// We can just use audio.Service directly.
