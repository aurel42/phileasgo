package config

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// MockStateStore implements store.StateStore for testing.
type MockStateStore struct {
	data map[string]string
}

func NewMockStateStore() *MockStateStore {
	return &MockStateStore{data: make(map[string]string)}
}

func (m *MockStateStore) GetState(ctx context.Context, key string) (string, bool) {
	val, ok := m.data[key]
	return val, ok
}

func (m *MockStateStore) SetState(ctx context.Context, key, val string) error {
	m.data[key] = val
	return nil
}

func (m *MockStateStore) DeleteState(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func TestUnifiedProvider(t *testing.T) {
	ctx := context.Background()
	baseCfg := &Config{}
	baseCfg.Sim.Provider = "test-sim"
	baseCfg.Sim.TeleportThreshold = 100
	baseCfg.Narrator.Units = "metric"
	baseCfg.Ticker.TelemetryLoop = Duration(1 * time.Second)
	baseCfg.Narrator.AutoNarrate = true
	baseCfg.Narrator.MinScoreThreshold = 0.5
	baseCfg.Narrator.Frequency = 10
	baseCfg.Narrator.RepeatTTL = Duration(3600 * time.Second)
	baseCfg.Narrator.TargetLanguage = "en-US"
	baseCfg.Narrator.ActiveTargetLanguage = "en-US"
	baseCfg.Narrator.TargetLanguageLibrary = []string{"en-US", "de-DE"}
	baseCfg.Sim.Mock.StartLat = 45.0
	baseCfg.Sim.Mock.StartLon = 5.0
	baseCfg.Sim.Mock.StartAlt = 1000.0
	h := 90.0
	baseCfg.Sim.Mock.StartHeading = &h
	baseCfg.Sim.Mock.DurationParked = Duration(60 * time.Second)
	baseCfg.Sim.Mock.DurationTaxi = Duration(30 * time.Second)
	baseCfg.Sim.Mock.DurationHold = Duration(15 * time.Second)
	baseCfg.Narrator.PauseDuration = Duration(5 * time.Second)
	baseCfg.Terrain.LineOfSight = true
	baseCfg.Narrator.Essay.Enabled = true
	baseCfg.Narrator.Essay.DelayBetweenEssays = Duration(300 * time.Second)
	baseCfg.Narrator.Essay.DelayBeforeEssay = Duration(60 * time.Second)
	baseCfg.Narrator.StyleLibrary = []string{"style1", "style2"}
	baseCfg.Narrator.ActiveStyle = "style1"
	baseCfg.Narrator.SecretWordLibrary = []string{"word1", "word2"}
	baseCfg.Narrator.ActiveSecretWord = "word1"

	store := NewMockStateStore()
	p := NewProvider(baseCfg, store)

	t.Run("Defaults_And_Fallbacks", func(t *testing.T) {
		if p.SimProvider(ctx) != "test-sim" {
			t.Errorf("expected test-sim, got %s", p.SimProvider(ctx))
		}
		if p.TeleportDistance(ctx) != 100.0 {
			t.Errorf("expected 100.0, got %f", p.TeleportDistance(ctx))
		}
		if p.Units(ctx) != "metric" {
			t.Errorf("expected metric, got %s", p.Units(ctx))
		}
		if p.TelemetryLoop(ctx) != 1000*time.Millisecond {
			t.Errorf("expected 1s, got %v", p.TelemetryLoop(ctx))
		}
		if p.AutoNarrate(ctx) != true {
			t.Error("expected auto narrate true")
		}
		if p.MinScoreThreshold(ctx) != 0.5 {
			t.Errorf("expected 0.5, got %f", p.MinScoreThreshold(ctx))
		}
		if p.NarrationFrequency(ctx) != 10 {
			t.Errorf("expected 10, got %d", p.NarrationFrequency(ctx))
		}
		if p.RepeatTTL(ctx) != 3600*time.Second {
			t.Errorf("expected 3600s, got %v", p.RepeatTTL(ctx))
		}
		if p.TargetLanguage(ctx) != "en-US" {
			t.Errorf("expected en-US, got %s", p.TargetLanguage(ctx))
		}
		if p.ActiveTargetLanguage(ctx) != "en-US" {
			t.Errorf("expected en-US, got %s", p.ActiveTargetLanguage(ctx))
		}
		if len(p.TargetLanguageLibrary(ctx)) != 2 {
			t.Errorf("expected 2 languages, got %d", len(p.TargetLanguageLibrary(ctx)))
		}
		if p.TextLengthScale(ctx) != 3 {
			t.Errorf("expected 3, got %d", p.TextLengthScale(ctx))
		}
		if p.MockStartLat(ctx) != 45.0 {
			t.Errorf("expected 45.0, got %f", p.MockStartLat(ctx))
		}
		if p.MockStartLon(ctx) != 5.0 {
			t.Errorf("expected 5.0, got %f", p.MockStartLon(ctx))
		}
		if p.MockStartAlt(ctx) != 1000.0 {
			t.Errorf("expected 1000.0, got %f", p.MockStartAlt(ctx))
		}
		if *p.MockStartHeading(ctx) != 90.0 {
			t.Errorf("expected 90.0, got %f", *p.MockStartHeading(ctx))
		}
		if p.MockDurationParked(ctx) != 60*time.Second {
			t.Errorf("expected 60s, got %v", p.MockDurationParked(ctx))
		}
		if p.MockDurationTaxi(ctx) != 30*time.Second {
			t.Errorf("expected 30s, got %v", p.MockDurationTaxi(ctx))
		}
		if p.MockDurationHold(ctx) != 15*time.Second {
			t.Errorf("expected 15s, got %v", p.MockDurationHold(ctx))
		}
		if p.ShowCacheLayer(ctx) != false {
			t.Error("expected false")
		}
		if p.ShowVisibilityLayer(ctx) != false {
			t.Error("expected false")
		}
		if p.FilterMode(ctx) != "fixed" {
			t.Errorf("expected fixed, got %s", p.FilterMode(ctx))
		}
		if p.TargetPOICount(ctx) != 5 {
			t.Errorf("expected 5, got %d", p.TargetPOICount(ctx))
		}
		if p.PauseDuration(ctx) != 5*time.Second {
			t.Errorf("expected 5s, got %v", p.PauseDuration(ctx))
		}
		if p.LineOfSight(ctx) != true {
			t.Error("expected true")
		}
		if p.EssayEnabled(ctx) != true {
			t.Error("expected true")
		}
		if p.EssayDelayBetweenEssays(ctx) != 300*time.Second {
			t.Errorf("expected 300s, got %v", p.EssayDelayBetweenEssays(ctx))
		}
		if p.EssayDelayBeforeEssay(ctx) != 60*time.Second {
			t.Errorf("expected 60s, got %v", p.EssayDelayBeforeEssay(ctx))
		}
		if len(p.StyleLibrary(ctx)) != 2 {
			t.Errorf("expected 2 styles, got %d", len(p.StyleLibrary(ctx)))
		}
		if p.ActiveStyle(ctx) != "style1" {
			t.Errorf("expected style1, got %s", p.ActiveStyle(ctx))
		}
		if len(p.SecretWordLibrary(ctx)) != 2 {
			t.Errorf("expected 2 words, got %d", len(p.SecretWordLibrary(ctx)))
		}
		if p.ActiveSecretWord(ctx) != "word1" {
			t.Errorf("expected word1, got %s", p.ActiveSecretWord(ctx))
		}
		if p.AppConfig() != baseCfg {
			t.Error("expected baseCfg")
		}
	})

	t.Run("Store_Overrides", func(t *testing.T) {
		store.SetState(ctx, KeySimSource, "mock")
		store.SetState(ctx, KeyTeleportDistance, "200.5")
		store.SetState(ctx, KeyUnits, "imperial")
		store.SetState(ctx, KeyMinPOIScore, "0.8")
		store.SetState(ctx, KeyNarrationFrequency, "20")
		store.SetState(ctx, KeyTextLength, "5")
		store.SetState(ctx, KeyMockLat, "50.0")
		store.SetState(ctx, KeyMockLon, "10.0")
		store.SetState(ctx, KeyMockAlt, "2000.0")
		store.SetState(ctx, KeyMockHeading, "180.0")
		store.SetState(ctx, KeyMockDurParked, "120s")
		store.SetState(ctx, KeyShowCacheLayer, "true")
		store.SetState(ctx, KeyShowVisibility, "true")
		store.SetState(ctx, KeyFilterMode, "dynamic")
		store.SetState(ctx, KeyTargetPOICount, "10")
		store.SetState(ctx, KeyActiveStyle, "style2")
		store.SetState(ctx, KeyActiveSecretWord, "word2")
		store.SetState(ctx, KeyActiveTargetLanguage, "fr-FR")

		styles, _ := json.Marshal([]string{"s1", "s2", "s3"})
		store.SetState(ctx, KeyStyleLibrary, string(styles))
		langs, _ := json.Marshal([]string{"fr-FR", "pl-PL"})
		store.SetState(ctx, KeyTargetLanguageLibrary, string(langs))

		if p.SimProvider(ctx) != "mock" {
			t.Errorf("expected mock, got %s", p.SimProvider(ctx))
		}
		if p.TeleportDistance(ctx) != 200.5 {
			t.Errorf("expected 200.5, got %f", p.TeleportDistance(ctx))
		}
		if p.Units(ctx) != "imperial" {
			t.Errorf("expected imperial, got %s", p.Units(ctx))
		}
		if p.MinScoreThreshold(ctx) != 0.8 {
			t.Errorf("expected 0.8, got %f", p.MinScoreThreshold(ctx))
		}
		if p.NarrationFrequency(ctx) != 20 {
			t.Errorf("expected 20, got %d", p.NarrationFrequency(ctx))
		}
		if p.TextLengthScale(ctx) != 5 {
			t.Errorf("expected 5, got %d", p.TextLengthScale(ctx))
		}
		if p.MockStartLat(ctx) != 50.0 {
			t.Errorf("expected 50.0, got %f", p.MockStartLat(ctx))
		}
		if p.MockStartLon(ctx) != 10.0 {
			t.Errorf("expected 10.0, got %f", p.MockStartLon(ctx))
		}
		if p.MockStartAlt(ctx) != 2000.0 {
			t.Errorf("expected 2000.0, got %f", p.MockStartAlt(ctx))
		}
		if *p.MockStartHeading(ctx) != 180.0 {
			t.Errorf("expected 180.0, got %f", *p.MockStartHeading(ctx))
		}
		if p.MockDurationParked(ctx) != 120*time.Second {
			t.Errorf("expected 120s, got %v", p.MockDurationParked(ctx))
		}
		if p.ShowCacheLayer(ctx) != true {
			t.Error("expected true")
		}
		if p.ShowVisibilityLayer(ctx) != true {
			t.Error("expected true")
		}
		if p.FilterMode(ctx) != "dynamic" {
			t.Errorf("expected dynamic, got %s", p.FilterMode(ctx))
		}
		if p.TargetPOICount(ctx) != 10 {
			t.Errorf("expected 10, got %d", p.TargetPOICount(ctx))
		}
		if p.ActiveStyle(ctx) != "style2" {
			t.Errorf("expected style2, got %s", p.ActiveStyle(ctx))
		}
		if p.ActiveSecretWord(ctx) != "word2" {
			t.Errorf("expected word2, got %s", p.ActiveSecretWord(ctx))
		}
		if p.ActiveTargetLanguage(ctx) != "fr-FR" {
			t.Errorf("expected fr-FR, got %s", p.ActiveTargetLanguage(ctx))
		}
		if len(p.TargetLanguageLibrary(ctx)) != 2 {
			t.Errorf("expected 2 languages, got %d", len(p.TargetLanguageLibrary(ctx)))
		}
		if len(p.StyleLibrary(ctx)) != 3 {
			t.Errorf("expected 3 styles, got %d", len(p.StyleLibrary(ctx)))
		}
	})

	t.Run("Conversion_Errors_Fallbacks", func(t *testing.T) {
		store.SetState(ctx, KeyTeleportDistance, "invalid")
		store.SetState(ctx, KeyMinPOIScore, "invalid")
		store.SetState(ctx, KeyNarrationFrequency, "invalid")
		store.SetState(ctx, KeyMockLat, "invalid")
		store.SetState(ctx, KeyMockHeading, "invalid")
		store.SetState(ctx, KeyMockDurParked, "invalid")
		store.SetState(ctx, KeyStyleLibrary, "invalid-json")

		if p.TeleportDistance(ctx) != 100.0 {
			t.Errorf("expected fallback 100.0, got %f", p.TeleportDistance(ctx))
		}
		if p.MinScoreThreshold(ctx) != 0.5 {
			t.Errorf("expected fallback 0.5, got %f", p.MinScoreThreshold(ctx))
		}
		if p.NarrationFrequency(ctx) != 10 {
			t.Errorf("expected fallback 10, got %d", p.NarrationFrequency(ctx))
		}
		if p.MockStartLat(ctx) != 45.0 {
			t.Errorf("expected fallback 45.0, got %f", p.MockStartLat(ctx))
		}
		if *p.MockStartHeading(ctx) != 90.0 {
			t.Errorf("expected fallback 90.0, got %f", *p.MockStartHeading(ctx))
		}
		if p.MockDurationParked(ctx) != 60*time.Second {
			t.Errorf("expected fallback 60s, got %v", p.MockDurationParked(ctx))
		}
		if len(p.StyleLibrary(ctx)) != 2 {
			t.Errorf("expected fallback 2 styles, got %d", len(p.StyleLibrary(ctx)))
		}
	})

	t.Run("Empty_Store_Handle", func(t *testing.T) {
		pNone := NewProvider(baseCfg, nil)
		if pNone.SimProvider(ctx) != "test-sim" {
			t.Errorf("expected test-sim, got %s", pNone.SimProvider(ctx))
		}
		if pNone.MockStartHeading(ctx) == nil || *pNone.MockStartHeading(ctx) != 90.0 {
			t.Error("expected fallback heading 90.0")
		}
	})
}
