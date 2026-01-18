package wikidata

import (
	"context"
	"encoding/json"
	"phileasgo/pkg/model"
	"testing"
)

// Reuse mockCache logic locally or import?
// mockCache definition in client_test.go is not exported.
// I will define a local mock for simplicity.

type mockCacher struct {
	data map[string][]byte
}

func (m *mockCacher) GetCache(ctx context.Context, key string) ([]byte, bool) {
	if m.data == nil {
		return nil, false
	}
	val, ok := m.data[key]
	return val, ok
}
func (m *mockCacher) SetCache(ctx context.Context, key string, val []byte) error {
	if m.data == nil {
		m.data = make(map[string][]byte)
	}
	m.data[key] = val
	return nil
}
func (m *mockCacher) GetGeodataCache(ctx context.Context, key string) ([]byte, int, bool) {
	return nil, 0, false
}
func (m *mockCacher) SetGeodataCache(ctx context.Context, key string, val []byte, radiusM int, lat, lon float64) error {
	return nil
}

func TestLanguageMapper_LoadSave(t *testing.T) {
	// Mock Cache
	mc := &mockCacher{data: make(map[string][]byte)}

	// data to save
	dataMap := map[string][]model.LanguageInfo{
		"CZ": {{Code: "cs", Name: "Czech"}},
		"DE": {{Code: "de", Name: "German"}},
	}

	lm := &LanguageMapper{
		cache:   mc,
		mapping: dataMap,
	}

	// Test Save (to cache)
	if err := lm.save(context.Background()); err != nil {
		t.Errorf("save() failed: %v", err)
	}

	// Verify cache content
	cachedData, ok := mc.data[langMapCacheKey]
	if !ok {
		t.Fatal("Cache missing key")
	}

	var loadedData map[string][]model.LanguageInfo
	if err := json.Unmarshal(cachedData, &loadedData); err != nil {
		t.Fatal(err)
	}
	if loadedData["CZ"][0].Code != "cs" {
		t.Errorf("Saved content mismatch")
	}

	// Test Load (from cache)
	lm2 := &LanguageMapper{
		cache:   mc,
		mapping: make(map[string][]model.LanguageInfo),
	}
	if err := lm2.load(context.Background()); err != nil {
		t.Errorf("load() failed: %v", err)
	}
	if lm2.GetLanguages("CZ")[0].Code != "cs" {
		t.Errorf("GetLanguages(CZ) = %v, want cs", lm2.GetLanguages("CZ"))
	}
}

func TestLanguageMapper_GetLanguage(t *testing.T) {
	lm := &LanguageMapper{
		mapping: map[string][]model.LanguageInfo{
			"US": {{Code: "en", Name: "English"}},
			"FR": {{Code: "fr", Name: "French"}},
		},
	}

	tests := []struct {
		country string
		want    string
	}{
		{"US", "en"},
		{"FR", "fr"},
		{"UNKNOWN", "en"}, // Default fallback
		{"", "en"},
	}

	for _, tt := range tests {
		if got := lm.GetLanguages(tt.country); got[0].Code != tt.want {
			t.Errorf("GetLanguages(%s) = %v, want %s", tt.country, got, tt.want)
		}
	}
}
