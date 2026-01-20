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
)

// Narrative represents a prepared narration ready for playback.
type Narrative struct {
	ID             string        `json:"id"`
	Type           NarrativeType `json:"type"` // "poi", "screenshot", "essay", "debrief"
	Title          string        `json:"title"`
	Script         string        `json:"script"`
	AudioPath      string        `json:"audio_path"`
	Format         string        `json:"format"`   // e.g., "mp3"
	Duration       time.Duration `json:"duration"` // Actual audio duration
	RequestedWords int           `json:"requested_words"`

	// Context Fields (Nullable/Optional)
	POI        *POI   `json:"poi,omitempty"`        // nil for non-POI narratives
	ImagePath  string `json:"image_path,omitempty"` // For screenshots
	EssayTopic string `json:"essay_topic,omitempty"`

	// Metadata
	Manual    bool      `json:"manual"` // True if manually requested by user
	CreatedAt time.Time `json:"created_at"`
}
