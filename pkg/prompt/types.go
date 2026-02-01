package prompt

import (
	"context"
	"phileasgo/pkg/model"
	"time"
)

// Data represents the gathered context for an AI prompt.
type Data map[string]any

// ImageResult represents a selected image for a POI.
type ImageResult struct {
	Title string
	URL   string
}

// SessionState holds transient flight session context.
type SessionState struct {
	TripSummary  string
	Events       []model.TripEvent
	LastSentence string
}

// Interfaces for dependencies

type GeoProvider interface {
	GetLocation(lat, lon float64) model.LocationInfo
}

type WikipediaProvider interface {
	GetArticleHTML(ctx context.Context, title, lang string) (string, error)
}

type Store interface {
	GetArticle(ctx context.Context, uuid string) (*model.Article, error)
	SaveArticle(ctx context.Context, art *model.Article) error
	GetRecentlyPlayedPOIs(ctx context.Context, since time.Time) ([]*model.POI, error)
	GetState(ctx context.Context, key string) (string, bool)
}

type LLMProvider interface {
	HasProfile(name string) bool
	GenerateText(ctx context.Context, profile, prompt string) (string, error)
}

type POIProvider interface {
	CountScoredAbove(threshold float64, limit int) int
}

type POIAnalyzer interface {
	POIProvider
}

type LanguageResolver interface {
	GetLanguageInfo(code string) model.LanguageInfo
}

type Renderer interface {
	Render(name string, data any) (string, error)
}

const (
	StrategyMinSkew = "min_skew"
	StrategyMaxSkew = "max_skew"
	StrategyUniform = "uniform"
	StrategyFixed   = "fixed"
)
