package wikidata

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"sync"
	"time"

	"phileasgo/pkg/cache"
	"phileasgo/pkg/model"
	"phileasgo/pkg/request"
)

const (
	langMapCacheKey = "sys_lang_map_v3"
	refreshInterval = 30 * 24 * time.Hour // Refresh monthly
)

// LanguageMapper handles dynamic resolution of Country Code -> Official Languages.
type LanguageMapper struct {
	cache   cache.Cacher
	client  *request.Client // Use Request Client directly for fetching map
	logger  *slog.Logger
	mu      sync.RWMutex
	mapping map[string][]model.LanguageInfo // CountryCode (ISO 2) -> List of official languages
}

// NewLanguageMapper creates a new mapper.
func NewLanguageMapper(c cache.Cacher, rc *request.Client, logger *slog.Logger) *LanguageMapper {
	return &LanguageMapper{
		cache:   c,
		client:  rc,
		logger:  logger,
		mapping: make(map[string][]model.LanguageInfo),
	}
}

// Start initializes the mapper by loading from cache or fetching from source.
func (m *LanguageMapper) Start(ctx context.Context) error {
	m.logger.Info("Starting LanguageMapper")

	// 1. Try Load from Store
	if err := m.load(ctx); err != nil {
		m.logger.Warn("Failed to load language map from store", "error", err)
	}

	// 2. If empty, fetch immediately
	m.mu.RLock()
	isEmpty := len(m.mapping) == 0
	m.mu.RUnlock()

	if isEmpty {
		m.logger.Info("Language map empty, fetching from Wikidata...")
		if err := m.refresh(ctx); err != nil {
			return fmt.Errorf("initial language map fetch failed: %w", err)
		}
	}

	return nil
}

// GetLanguages returns all official languages for a given country code.
// Returns []{Code: "en", Name: "English"} if not found or empty.
func (m *LanguageMapper) GetLanguages(countryCode string) []model.LanguageInfo {
	fallback := []model.LanguageInfo{{Code: "en", Name: "English"}}
	if countryCode == "" {
		return fallback
	}

	m.mu.RLock()
	langs, ok := m.mapping[countryCode]
	m.mu.RUnlock()

	if ok && len(langs) > 0 {
		return langs
	}
	return fallback
}

func (m *LanguageMapper) load(ctx context.Context) error {
	data, ok := m.cache.GetCache(ctx, langMapCacheKey)
	if !ok {
		return nil // Not found is fine
	}

	var loaded map[string][]model.LanguageInfo
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	m.mu.Lock()
	m.mapping = loaded
	m.mu.Unlock()
	return nil
}

func (m *LanguageMapper) save(ctx context.Context) error {
	m.mu.RLock()
	data, err := json.Marshal(m.mapping)
	m.mu.RUnlock()
	if err != nil {
		return err
	}
	return m.cache.SetCache(ctx, langMapCacheKey, data)
}

func (m *LanguageMapper) refresh(ctx context.Context) error {
	query := `
	SELECT DISTINCT ?countryCode ?langCode ?officialLangLabel WHERE {
	  ?c wdt:P297 ?countryCode ;
		 wdt:P37 ?officialLang .
	  ?officialLang wdt:P424 ?langCode .
	  SERVICE wikibase:label { bd:serviceParam wikibase:language "en". }
	}
	`
	// Simple wrapper for SPARQL request
	// We can reuse the same endpoint constant from client.go if exported, or duplicate
	endpoint := "https://query.wikidata.org/sparql"

	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Add("query", query)
	q.Add("format", "json")
	u.RawQuery = q.Encode()

	headers := map[string]string{
		"Accept": "application/sparql-results+json",
	}

	body, err := m.client.GetWithHeaders(ctx, u.String(), headers, "")
	if err != nil {
		return err
	}

	// Parse
	var result sparqlResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}

	newMap := make(map[string][]model.LanguageInfo)
	for _, b := range result.Results.Bindings {
		cc := val(b, "countryCode")
		lc := val(b, "langCode")
		ln := val(b, "officialLangLabel") // Label Service provides this

		if cc != "" && lc != "" {
			if ln == "" {
				ln = "Unknown"
			}
			newMap[cc] = append(newMap[cc], model.LanguageInfo{Code: lc, Name: ln})
		}
	}

	m.logger.Info("Refreshed Language Map", "count", len(newMap))
	if len(newMap) == 0 {
		return fmt.Errorf("fetched 0 mappings")
	}

	m.mu.Lock()
	m.mapping = newMap
	m.mu.Unlock()

	return m.save(ctx)
}
