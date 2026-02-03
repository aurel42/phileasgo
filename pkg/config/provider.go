package config

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"phileasgo/pkg/store"
)

// Provider defines the interface for accessing unified configuration.
type Provider interface {
	// General
	SimProvider(ctx context.Context) string
	Units(ctx context.Context) string

	// Narrator
	AutoNarrate(ctx context.Context) bool
	MinScoreThreshold(ctx context.Context) float64
	NarrationFrequency(ctx context.Context) int
	RepeatTTL(ctx context.Context) time.Duration
	TargetLanguage(ctx context.Context) string
	TextLengthScale(ctx context.Context) int

	// Mock Sim
	MockStartLat(ctx context.Context) float64
	MockStartLon(ctx context.Context) float64
	MockStartAlt(ctx context.Context) float64
	MockStartHeading(ctx context.Context) *float64
	MockDurationParked(ctx context.Context) time.Duration
	MockDurationTaxi(ctx context.Context) time.Duration
	MockDurationHold(ctx context.Context) time.Duration

	// UI / Overlay
	ShowCacheLayer(ctx context.Context) bool
	ShowVisibilityLayer(ctx context.Context) bool
	FilterMode(ctx context.Context) string
	TargetPOICount(ctx context.Context) int

	// Raw access (for components that need deep access)
	AppConfig() *Config
}

// UnifiedProvider implements Provider by bridging static Config and persistent Store.
type UnifiedProvider struct {
	base  *Config
	store store.StateStore
}

// NewProvider creates a new UnifiedProvider.
func NewProvider(base *Config, st store.StateStore) *UnifiedProvider {
	return &UnifiedProvider{
		base:  base,
		store: st,
	}
}

func (p *UnifiedProvider) AppConfig() *Config { return p.base }

// --- Implementations ---

func (p *UnifiedProvider) SimProvider(ctx context.Context) string {
	fallback := p.base.Sim.Provider
	if fallback == "" {
		fallback = "simconnect"
	}
	return p.getString(ctx, KeySimSource, fallback)
}

func (p *UnifiedProvider) Units(ctx context.Context) string {
	return p.getString(ctx, KeyUnits, p.base.Narrator.Units)
}

func (p *UnifiedProvider) AutoNarrate(ctx context.Context) bool {
	return p.base.Narrator.AutoNarrate
}

func (p *UnifiedProvider) MinScoreThreshold(ctx context.Context) float64 {
	return p.getFloat64(ctx, KeyMinPOIScore, p.base.Narrator.MinScoreThreshold)
}

func (p *UnifiedProvider) NarrationFrequency(ctx context.Context) int {
	return p.getInt(ctx, KeyNarrationFrequency, p.base.Narrator.Frequency)
}

func (p *UnifiedProvider) RepeatTTL(ctx context.Context) time.Duration {
	return time.Duration(p.base.Narrator.RepeatTTL)
}

func (p *UnifiedProvider) TargetLanguage(ctx context.Context) string {
	return p.base.Narrator.TargetLanguage
}

func (p *UnifiedProvider) TextLengthScale(ctx context.Context) int {
	return p.getInt(ctx, KeyTextLength, 3) // Default 3 (Normal)
}

func (p *UnifiedProvider) MockStartLat(ctx context.Context) float64 {
	return p.getFloat64(ctx, KeyMockLat, p.base.Sim.Mock.StartLat)
}

func (p *UnifiedProvider) MockStartLon(ctx context.Context) float64 {
	return p.getFloat64(ctx, KeyMockLon, p.base.Sim.Mock.StartLon)
}

func (p *UnifiedProvider) MockStartAlt(ctx context.Context) float64 {
	return p.getFloat64(ctx, KeyMockAlt, p.base.Sim.Mock.StartAlt)
}

func (p *UnifiedProvider) MockStartHeading(ctx context.Context) *float64 {
	val, ok := p.store.GetState(ctx, KeyMockHeading)
	if !ok || val == "" {
		return p.base.Sim.Mock.StartHeading
	}
	var h float64
	if _, err := fmt.Sscanf(val, "%f", &h); err != nil {
		return p.base.Sim.Mock.StartHeading
	}
	return &h
}

func (p *UnifiedProvider) MockDurationParked(ctx context.Context) time.Duration {
	return p.getDuration(ctx, KeyMockDurParked, time.Duration(p.base.Sim.Mock.DurationParked))
}

func (p *UnifiedProvider) MockDurationTaxi(ctx context.Context) time.Duration {
	return p.getDuration(ctx, KeyMockDurTaxi, time.Duration(p.base.Sim.Mock.DurationTaxi))
}

func (p *UnifiedProvider) MockDurationHold(ctx context.Context) time.Duration {
	return p.getDuration(ctx, KeyMockDurHold, time.Duration(p.base.Sim.Mock.DurationHold))
}

func (p *UnifiedProvider) ShowCacheLayer(ctx context.Context) bool {
	return p.getBool(ctx, KeyShowCacheLayer, false)
}

func (p *UnifiedProvider) ShowVisibilityLayer(ctx context.Context) bool {
	return p.getBool(ctx, KeyShowVisibility, false)
}

func (p *UnifiedProvider) FilterMode(ctx context.Context) string {
	return p.getString(ctx, KeyFilterMode, "adaptive")
}

func (p *UnifiedProvider) TargetPOICount(ctx context.Context) int {
	return p.getInt(ctx, KeyTargetPOICount, 5)
}

// --- Helpers ---

func (p *UnifiedProvider) getString(ctx context.Context, key, fallback string) string {
	if p.store != nil {
		if val, ok := p.store.GetState(ctx, key); ok && val != "" {
			return val
		}
	}
	return fallback
}

func (p *UnifiedProvider) getInt(ctx context.Context, key string, fallback int) int {
	if p.store != nil {
		if val, ok := p.store.GetState(ctx, key); ok && val != "" {
			if i, err := strconv.Atoi(val); err == nil {
				return i
			}
		}
	}
	return fallback
}

func (p *UnifiedProvider) getFloat64(ctx context.Context, key string, fallback float64) float64 {
	if p.store != nil {
		if val, ok := p.store.GetState(ctx, key); ok && val != "" {
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				return f
			}
		}
	}
	return fallback
}

func (p *UnifiedProvider) getBool(ctx context.Context, key string, fallback bool) bool {
	if p.store != nil {
		if val, ok := p.store.GetState(ctx, key); ok && val != "" {
			return val == "true"
		}
	}
	return fallback
}

func (p *UnifiedProvider) getDuration(ctx context.Context, key string, fallback time.Duration) time.Duration {
	if p.store != nil {
		if val, ok := p.store.GetState(ctx, key); ok && val != "" {
			if dur, err := ParseDuration(val); err == nil {
				return dur
			}
		}
	}
	return fallback
}
