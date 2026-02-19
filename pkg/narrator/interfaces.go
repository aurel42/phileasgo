package narrator

import (
	"context"
	"time"

	"phileasgo/pkg/announcement"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

// POIProvider defines the interface for POI management.
type POIProvider interface {
	GetPOI(ctx context.Context, qid string) (*model.POI, error)

	GetNarrationCandidates(limit int, minScore *float64) []*model.POI
	CountScoredAbove(threshold float64, limit int) int
	GetPOIsNear(lat, lon, radiusMeters float64) []*model.POI
	ClearBeaconColor(color string)

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
	GetArticleHTML(ctx context.Context, title, lang string) (string, error)
}

// LanguageResolver defines the interface for resolving language details.
type LanguageResolver interface {
	GetLanguageInfo(countryCode string) model.LanguageInfo
}

// BeaconProvider defines the interface for beacon/marker management.
type BeaconProvider interface {
	SetTarget(ctx context.Context, lat, lon float64, title, livery string) error
	Clear()
}

// Generator defines the interface for narration generation.
type Generator interface {
	GenerateNarrative(ctx context.Context, req *GenerationRequest) (*model.Narrative, error)
	ProcessGenerationQueue(ctx context.Context)
	HasPendingGeneration() bool
	IsGenerating() bool
	Stats() map[string]any
	// PlayEssay triggers a regional essay narration.
	PlayEssay(ctx context.Context, tel *sim.Telemetry) bool
	// IsPOIBusy returns true if the POI is currently generating or queued.
	IsPOIBusy(poiID string) bool
	EnqueueAnnouncement(ctx context.Context, a announcement.Item, t *sim.Telemetry, onComplete func(*model.Narrative))
	POIManager() POIProvider
	LLMProvider() llm.Provider
	AverageLatency() time.Duration
	RecordNarration(ctx context.Context, n *model.Narrative)
	Reset(ctx context.Context)
}

// AudioProvider alias for audio.Service to keep imports clean or use direct import.
// We can just use audio.Service directly.
