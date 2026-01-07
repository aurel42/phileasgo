package narrator

import (
	"context"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/tts"
)

// --- Mocks ---

type MockLLM struct {
	Response         string
	Err              error
	GenerateTextFunc func(ctx context.Context, name, prompt string) (string, error)
}

func (m *MockLLM) Name() string                         { return "MockLLM" }
func (m *MockLLM) Configure(cfg config.LLMConfig) error { return nil }
func (m *MockLLM) GenerateText(ctx context.Context, name, prompt string) (string, error) {
	if m.GenerateTextFunc != nil {
		return m.GenerateTextFunc(ctx, name, prompt)
	}
	return m.Response, m.Err
}
func (m *MockLLM) GenerateJSON(ctx context.Context, name, prompt string, target any) error {
	return nil
}

type MockTTS struct {
	Format string
	Err    error
}

func (m *MockTTS) Voices(ctx context.Context) ([]tts.Voice, error) {
	return []tts.Voice{{ID: "voice-f", Name: "Female Voice"}}, nil
}
func (m *MockTTS) Synthesize(ctx context.Context, text, voiceID, outputPath string) (string, error) {
	return m.Format, m.Err
}

type MockAudio struct {
	PlayCalls    int
	LastFile     string
	PlayErr      error
	IsPlayingVal bool
}

func (m *MockAudio) Play(filepath string, startPaused bool) error {
	m.PlayCalls++
	m.LastFile = filepath
	return m.PlayErr
}
func (m *MockAudio) Pause()                    {}
func (m *MockAudio) Resume()                   {}
func (m *MockAudio) Stop()                     {}
func (m *MockAudio) IsPlaying() bool           { return m.IsPlayingVal }
func (m *MockAudio) IsBusy() bool              { return m.IsPlayingVal }
func (m *MockAudio) IsPaused() bool            { return false }
func (m *MockAudio) SetVolume(vol float64)     {}
func (m *MockAudio) Volume() float64           { return 1.0 }
func (m *MockAudio) SetUserPaused(paused bool) {}
func (m *MockAudio) IsUserPaused() bool        { return false }
func (m *MockAudio) ResetUserPause()           {}
func (m *MockAudio) LastNarrationFile() string { return m.LastFile }
func (m *MockAudio) ReplayLastNarration() bool { return m.PlayErr == nil }
func (m *MockAudio) Position() time.Duration   { return 0 }
func (m *MockAudio) Duration() time.Duration   { return 0 }

type MockPOIProvider struct {
	GetPOIFunc  func(ctx context.Context, qid string) (*model.POI, error)
	GetBestFunc func() *model.POI
}

func (m *MockPOIProvider) GetPOI(ctx context.Context, qid string) (*model.POI, error) {
	if m.GetPOIFunc != nil {
		return m.GetPOIFunc(ctx, qid)
	}
	return nil, nil
}
func (m *MockPOIProvider) GetBestCandidate() *model.POI {
	return nil
}

func (m *MockPOIProvider) CountScoredAbove(threshold float64, limit int) int {
	return 0 // default for tests
}

type MockGeo struct {
	Country string
}

func (m *MockGeo) GetCountry(lat, lon float64) string {
	return m.Country
}

func (m *MockGeo) GetLocation(lat, lon float64) model.LocationInfo {
	return model.LocationInfo{
		CityName:    "MockCity",
		CountryCode: m.Country,
	}
}

type MockWiki struct {
	Content string
	Err     error
}

func (m *MockWiki) GetArticleContent(ctx context.Context, title, lang string) (string, error) {
	return m.Content, m.Err
}

type MockBeacon struct {
	Cleared bool
}

func (m *MockBeacon) SetTarget(ctx context.Context, lat, lon float64) error {
	return nil
}

func (m *MockBeacon) Clear() {
	m.Cleared = true
}

type MockSim struct {
	Telemetry sim.Telemetry
	Err       error
}

func (m *MockSim) GetTelemetry(ctx context.Context) (sim.Telemetry, error) {
	return m.Telemetry, m.Err
}
func (m *MockSim) GetState() sim.State                 { return sim.StateActive }
func (m *MockSim) SetPredictionWindow(d time.Duration) {}
func (m *MockSim) SetObjectPosition(id uint32, lat, lon, alt, pitch, bank, heading float64) error {
	return nil
}
func (m *MockSim) SpawnAirTraffic(reqID uint32, title, tailNum string, lat, lon, alt, heading float64) (uint32, error) {
	return 0, nil
}
func (m *MockSim) RemoveObject(id, reqID uint32) error                   { return nil }
func (m *MockSim) SubscribeToData(defineID uint32, structType any) error { return nil }
func (m *MockSim) RequestData(defineID, reqID uint32) error              { return nil }
func (m *MockSim) Close() error                                          { return nil }

type MockStore struct {
	SavedPOIs  []*model.POI
	Articles   map[string]*model.Article
	RecentPOIs []*model.POI
}

func (m *MockStore) SavePOI(ctx context.Context, p *model.POI) error {
	m.SavedPOIs = append(m.SavedPOIs, p)
	return nil
}
func (m *MockStore) GetPOI(ctx context.Context, qid string) (*model.POI, error) { return nil, nil }
func (m *MockStore) GetPOIsBatch(ctx context.Context, ids []string) (map[string]*model.POI, error) {
	return nil, nil
}
func (m *MockStore) GetRecentlyPlayedPOIs(ctx context.Context, since time.Time) ([]*model.POI, error) {
	return m.RecentPOIs, nil
}
func (m *MockStore) ResetLastPlayed(ctx context.Context, lat, lon, radius float64) error { return nil }
func (m *MockStore) SaveArticle(ctx context.Context, a *model.Article) error             { return nil }
func (m *MockStore) GetArticle(ctx context.Context, uuid string) (*model.Article, error) {
	if m.Articles != nil {
		if a, ok := m.Articles[uuid]; ok {
			return a, nil
		}
	}
	return nil, nil
}
func (m *MockStore) GetConfig(ctx context.Context) (map[string]string, error)    { return nil, nil }
func (m *MockStore) SaveConfig(ctx context.Context, cfg map[string]string) error { return nil }
func (m *MockStore) GetCache(ctx context.Context, key string) ([]byte, bool)     { return nil, false }
func (m *MockStore) HasCache(ctx context.Context, key string) (bool, error)      { return false, nil }
func (m *MockStore) SetCache(ctx context.Context, key string, val []byte) error  { return nil }
func (m *MockStore) ListCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}

func (m *MockStore) GetState(ctx context.Context, key string) (string, bool) { return "", false }
func (m *MockStore) SetState(ctx context.Context, key, val string) error     { return nil }

func (m *MockStore) GetMSFSPOI(ctx context.Context, id int64) (*model.MSFSPOI, error) {
	return nil, nil
}
func (m *MockStore) SaveMSFSPOI(ctx context.Context, poi *model.MSFSPOI) error { return nil }
func (m *MockStore) CheckMSFSPOI(ctx context.Context, lat, lon, radius float64) (bool, error) {
	return false, nil
}

func (m *MockStore) GetHierarchy(ctx context.Context, qid string) (*model.WikidataHierarchy, error) {
	return nil, nil
}
func (m *MockStore) SaveHierarchy(ctx context.Context, h *model.WikidataHierarchy) error { return nil }
func (m *MockStore) GetClassification(ctx context.Context, qid string) (category string, found bool, err error) {
	return "", false, nil
}
func (m *MockStore) SaveClassification(ctx context.Context, qid, category string, parents []string, label string) error {
	return nil
}

func (m *MockStore) GetSeenEntitiesBatch(ctx context.Context, qids []string) (map[string][]string, error) {
	return nil, nil
}
func (m *MockStore) MarkEntitiesSeen(ctx context.Context, entities map[string][]string) error {
	return nil
}

func (m *MockStore) Close() error { return nil }
