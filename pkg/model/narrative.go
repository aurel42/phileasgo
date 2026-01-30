package model

import (
	"time"
)

// NarrativeType defines the type of content being narrated.
type NarrativeType string

const (
	NarrativeTypePOI        NarrativeType = "poi"
	NarrativeTypeEssay      NarrativeType = "essay"
	NarrativeTypeScreenshot NarrativeType = "screenshot"
	NarrativeTypeDebrief    NarrativeType = "debrief"
	NarrativeTypeBorder     NarrativeType = "border"
	NarrativeTypeLetsgo     NarrativeType = "letsgo"
	NarrativeTypeBriefing   NarrativeType = "briefing"
)

// Narrative represents a prepared narration ready for playback.
type Narrative struct {
	ID                string        `json:"id"`
	Type              NarrativeType `json:"type"` // "poi", "screenshot", "essay", "debrief", "border"
	Title             string        `json:"title"`
	Script            string        `json:"script"`
	AudioPath         string        `json:"audio_path"`
	Format            string        `json:"format"`   // e.g., "mp3"
	Duration          time.Duration `json:"duration"` // Actual audio duration
	GenerationLatency time.Duration `json:"generation_latency"`
	PredictedLatency  time.Duration `json:"predicted_latency"`
	RequestedWords    int           `json:"requested_words"`

	// Context Fields (Nullable/Optional)
	POI        *POI   `json:"poi,omitempty"`        // nil for non-POI narratives
	ImagePath  string `json:"image_path,omitempty"` // For screenshots
	EssayTopic string `json:"essay_topic,omitempty"`

	// Location Context (Snapshot)
	Lat float64 `json:"lat,omitempty"`
	Lon float64 `json:"lon,omitempty"`

	// Metadata
	Manual    bool      `json:"manual"` // True if manually requested by user
	CreatedAt time.Time `json:"created_at"`
}
