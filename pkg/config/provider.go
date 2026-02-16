package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"phileasgo/pkg/store"
)

// Provider defines the interface for accessing unified configuration.
type Provider interface {
	// General
	SimProvider(ctx context.Context) string
	TeleportDistance(ctx context.Context) float64
	Units(ctx context.Context) string          // Prompt template units (imperial/hybrid/metric)
	RangeRingUnits(ctx context.Context) string // Map display units (km/nm)
	TelemetryLoop(ctx context.Context) time.Duration

	// Narrator
	AutoNarrate(ctx context.Context) bool
	MinScoreThreshold(ctx context.Context) float64
	NarrationFrequency(ctx context.Context) int
	RepeatTTL(ctx context.Context) time.Duration
	TargetLanguage(ctx context.Context) string
	ActiveTargetLanguage(ctx context.Context) string
	TargetLanguageLibrary(ctx context.Context) []string
	NarrationLengthShort(ctx context.Context) int
	NarrationLengthLong(ctx context.Context) int
	TextLengthScale(ctx context.Context) int
	TwoPassScriptGeneration(ctx context.Context) bool

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
	SettlementLabelLimit(ctx context.Context) int
	SettlementTier(ctx context.Context) int
	FilterMode(ctx context.Context) string
	TargetPOICount(ctx context.Context) int
	PauseDuration(ctx context.Context) time.Duration
	LineOfSight(ctx context.Context) bool
	DeferralProximityBoostPower(ctx context.Context) float64
	DeferralThreshold(ctx context.Context) float64

	// Essay
	EssayEnabled(ctx context.Context) bool
	EssayDelayBetweenEssays(ctx context.Context) time.Duration
	EssayDelayBeforeEssay(ctx context.Context) time.Duration

	// Style Library
	StyleLibrary(ctx context.Context) []string
	ActiveStyle(ctx context.Context) string
	ActiveMapStyle(ctx context.Context) string

	// Secret Word (Trip Theme)
	SecretWordLibrary(ctx context.Context) []string
	ActiveSecretWord(ctx context.Context) string

	// Beacon
	BeaconEnabled(ctx context.Context) bool
	BeaconFormationEnabled(ctx context.Context) bool
	BeaconFormationDistance(ctx context.Context) Distance
	BeaconFormationCount(ctx context.Context) int
	BeaconMinSpawnAltitude(ctx context.Context) Distance
	BeaconAltitudeFloor(ctx context.Context) Distance
	BeaconSinkDistanceFar(ctx context.Context) Distance
	BeaconSinkDistanceClose(ctx context.Context) Distance
	BeaconTargetFloorAGL(ctx context.Context) Distance
	BeaconMaxTargets(ctx context.Context) int

	// Aircraft
	AircraftIcon(ctx context.Context) string
	AircraftSize(ctx context.Context) int
	AircraftColorMain(ctx context.Context) string
	AircraftColorAccent(ctx context.Context) string

	// Aesthetics
	PaperOpacityClear(ctx context.Context) float64
	PaperOpacityFog(ctx context.Context) float64
	ParchmentSaturation(ctx context.Context) float64

	// Audio
	Volume(ctx context.Context) float64

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
func (p *UnifiedProvider) TeleportDistance(ctx context.Context) float64 {
	return p.getFloat64(ctx, KeyTeleportDistance, float64(p.base.Sim.TeleportThreshold))
}

func (p *UnifiedProvider) Units(ctx context.Context) string {
	return p.getString(ctx, KeyUnits, p.base.Narrator.Units)
}

// RangeRingUnits returns the map display units (km or nm) for the frontend.
// This is stored separately from the prompt template units.
func (p *UnifiedProvider) RangeRingUnits(ctx context.Context) string {
	return p.getString(ctx, KeyRangeRingUnits, "km")
}

func (p *UnifiedProvider) TelemetryLoop(ctx context.Context) time.Duration {
	return time.Duration(p.base.Ticker.TelemetryLoop)
}

func (p *UnifiedProvider) AutoNarrate(ctx context.Context) bool {
	return p.getBool(ctx, KeyAutoNarrate, p.base.Narrator.AutoNarrate)
}

func (p *UnifiedProvider) MinScoreThreshold(ctx context.Context) float64 {
	return p.getFloat64(ctx, KeyMinPOIScore, p.base.Narrator.MinScoreThreshold)
}

func (p *UnifiedProvider) NarrationFrequency(ctx context.Context) int {
	return p.getInt(ctx, KeyNarrationFrequency, p.base.Narrator.Frequency)
}

func (p *UnifiedProvider) NarrationLengthShort(ctx context.Context) int {
	return p.getInt(ctx, KeyNarrationLengthShort, p.base.Narrator.NarrationLengthShortWords)
}

func (p *UnifiedProvider) NarrationLengthLong(ctx context.Context) int {
	return p.getInt(ctx, KeyNarrationLengthLong, p.base.Narrator.NarrationLengthLongWords)
}

func (p *UnifiedProvider) RepeatTTL(ctx context.Context) time.Duration {
	return p.getDuration(ctx, KeyRepeatTTL, time.Duration(p.base.Narrator.RepeatTTL))
}

func (p *UnifiedProvider) TargetLanguage(ctx context.Context) string {
	return p.base.Narrator.TargetLanguage
}

func (p *UnifiedProvider) ActiveTargetLanguage(ctx context.Context) string {
	return p.getString(ctx, KeyActiveTargetLanguage, p.base.Narrator.ActiveTargetLanguage)
}

func (p *UnifiedProvider) TargetLanguageLibrary(ctx context.Context) []string {
	return p.getStringSlice(ctx, KeyTargetLanguageLibrary, p.base.Narrator.TargetLanguageLibrary)
}

func (p *UnifiedProvider) TextLengthScale(ctx context.Context) int {
	return p.getInt(ctx, KeyTextLength, 3) // Default 3 (Normal)
}

func (p *UnifiedProvider) TwoPassScriptGeneration(ctx context.Context) bool {
	return p.getBool(ctx, KeyTwoPassScriptGeneration, p.base.Narrator.TwoPassScriptGeneration)
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
	if p.store != nil {
		if val, ok := p.store.GetState(ctx, KeyMockHeading); ok && val != "" {
			var h float64
			if _, err := fmt.Sscanf(val, "%f", &h); err == nil {
				return &h
			}
		}
	}
	return p.base.Sim.Mock.StartHeading
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

func (p *UnifiedProvider) SettlementLabelLimit(ctx context.Context) int {
	return p.getInt(ctx, KeySettlementLabelLimit, p.base.Overlay.SettlementLabelLimit)
}

func (p *UnifiedProvider) SettlementTier(ctx context.Context) int {
	// Default to 3 (Village/All) if not set
	return p.getInt(ctx, KeySettlementTier, 3)
}

func (p *UnifiedProvider) FilterMode(ctx context.Context) string {
	return p.getString(ctx, KeyFilterMode, "fixed")
}

func (p *UnifiedProvider) TargetPOICount(ctx context.Context) int {
	return p.getInt(ctx, KeyTargetPOICount, 5)
}

func (p *UnifiedProvider) PauseDuration(ctx context.Context) time.Duration {
	return p.getDuration(ctx, KeyPauseDuration, time.Duration(p.base.Narrator.PauseDuration))
}

func (p *UnifiedProvider) LineOfSight(ctx context.Context) bool {
	return p.base.Terrain.LineOfSight
}

func (p *UnifiedProvider) DeferralProximityBoostPower(ctx context.Context) float64 {
	return p.getFloat64(ctx, KeyDeferralProximityBoostPower, p.base.Scorer.DeferralProximityBoostPower)
}

func (p *UnifiedProvider) DeferralThreshold(ctx context.Context) float64 {
	return p.getFloat64(ctx, KeyDeferralThreshold, p.base.Scorer.DeferralThreshold)
}

func (p *UnifiedProvider) EssayEnabled(ctx context.Context) bool {
	return p.base.Narrator.Essay.Enabled
}

func (p *UnifiedProvider) EssayDelayBetweenEssays(ctx context.Context) time.Duration {
	return time.Duration(p.base.Narrator.Essay.DelayBetweenEssays)
}

func (p *UnifiedProvider) EssayDelayBeforeEssay(ctx context.Context) time.Duration {
	return time.Duration(p.base.Narrator.Essay.DelayBeforeEssay)
}

func (p *UnifiedProvider) StyleLibrary(ctx context.Context) []string {
	return p.getStringSlice(ctx, KeyStyleLibrary, p.base.Narrator.StyleLibrary)
}

func (p *UnifiedProvider) ActiveStyle(ctx context.Context) string {
	return p.getOptionalString(ctx, KeyActiveStyle, p.base.Narrator.ActiveStyle)
}

func (p *UnifiedProvider) ActiveMapStyle(ctx context.Context) string {
	return p.getString(ctx, KeyActiveMapStyle, p.base.Narrator.ActiveMapStyle)
}

func (p *UnifiedProvider) SecretWordLibrary(ctx context.Context) []string {
	return p.getStringSlice(ctx, KeySecretWordLibrary, p.base.Narrator.SecretWordLibrary)
}

func (p *UnifiedProvider) ActiveSecretWord(ctx context.Context) string {
	return p.getOptionalString(ctx, KeyActiveSecretWord, p.base.Narrator.ActiveSecretWord)
}

func (p *UnifiedProvider) BeaconEnabled(ctx context.Context) bool {
	return p.getBool(ctx, KeyBeaconEnabled, p.base.Beacon.Enabled)
}

func (p *UnifiedProvider) BeaconFormationEnabled(ctx context.Context) bool {
	return p.getBool(ctx, KeyBeaconFormationEnabled, p.base.Beacon.FormationEnabled)
}

func (p *UnifiedProvider) BeaconFormationDistance(ctx context.Context) Distance {
	return p.getDistance(ctx, KeyBeaconFormationDistance, p.base.Beacon.FormationDistance)
}

func (p *UnifiedProvider) BeaconFormationCount(ctx context.Context) int {
	return p.getInt(ctx, KeyBeaconFormationCount, p.base.Beacon.FormationCount)
}

func (p *UnifiedProvider) BeaconMinSpawnAltitude(ctx context.Context) Distance {
	return p.getDistance(ctx, KeyBeaconMinSpawnAltitude, p.base.Beacon.MinSpawnAltitude)
}

func (p *UnifiedProvider) BeaconAltitudeFloor(ctx context.Context) Distance {
	return p.getDistance(ctx, KeyBeaconAltitudeFloor, p.base.Beacon.AltitudeFloor)
}

func (p *UnifiedProvider) BeaconSinkDistanceFar(ctx context.Context) Distance {
	return p.getDistance(ctx, KeyBeaconSinkDistanceFar, p.base.Beacon.TargetSinkDistanceFar)
}

func (p *UnifiedProvider) BeaconSinkDistanceClose(ctx context.Context) Distance {
	return p.getDistance(ctx, KeyBeaconSinkDistanceClose, p.base.Beacon.TargetSinkDistanceClose)
}

func (p *UnifiedProvider) BeaconTargetFloorAGL(ctx context.Context) Distance {
	return p.getDistance(ctx, KeyBeaconTargetFloorAGL, p.base.Beacon.TargetFloorAGL)
}

func (p *UnifiedProvider) BeaconMaxTargets(ctx context.Context) int {
	return p.getInt(ctx, KeyBeaconMaxTargets, p.base.Beacon.MaxTargets)
}

func (p *UnifiedProvider) AircraftIcon(ctx context.Context) string {
	return p.getString(ctx, KeyAircraftIcon, p.base.Scorer.AircraftIcon)
}

func (p *UnifiedProvider) AircraftSize(ctx context.Context) int {
	return p.getInt(ctx, KeyAircraftSize, p.base.Scorer.AircraftSize)
}

func (p *UnifiedProvider) AircraftColorMain(ctx context.Context) string {
	return p.getString(ctx, KeyAircraftColorMain, p.base.Scorer.AircraftColorMain)
}

func (p *UnifiedProvider) AircraftColorAccent(ctx context.Context) string {
	return p.getString(ctx, KeyAircraftColorAccent, p.base.Scorer.AircraftColorAccent)
}

func (p *UnifiedProvider) PaperOpacityClear(ctx context.Context) float64 {
	return p.getFloat64(ctx, KeyPaperOpacityClear, 0.1)
}

func (p *UnifiedProvider) PaperOpacityFog(ctx context.Context) float64 {
	return p.getFloat64(ctx, KeyPaperOpacityFog, 0.7)
}

func (p *UnifiedProvider) ParchmentSaturation(ctx context.Context) float64 {
	return p.getFloat64(ctx, KeyParchmentSaturation, 1.0)
}

func (p *UnifiedProvider) Volume(ctx context.Context) float64 {
	return p.getFloat64(ctx, KeyVolume, 1.0)
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

// getOptionalString returns the stored value even if empty, only falling back if not set.
func (p *UnifiedProvider) getOptionalString(ctx context.Context, key, fallback string) string {
	if p.store != nil {
		if val, ok := p.store.GetState(ctx, key); ok {
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

func (p *UnifiedProvider) getDistance(ctx context.Context, key string, fallback Distance) Distance {
	if p.store != nil {
		if val, ok := p.store.GetState(ctx, key); ok && val != "" {
			if d, err := ParseDistance(val); err == nil {
				return Distance(d)
			}
		}
	}
	return fallback
}

func (p *UnifiedProvider) getStringSlice(ctx context.Context, key string, fallback []string) []string {
	if p.store != nil {
		if val, ok := p.store.GetState(ctx, key); ok && val != "" {
			var result []string
			if err := json.Unmarshal([]byte(val), &result); err == nil {
				return result
			}
		}
	}
	return fallback
}
