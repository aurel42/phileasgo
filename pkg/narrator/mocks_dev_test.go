package narrator

import (
	"context"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"phileasgo/pkg/tts"
)

// --- Mocks ---

type MockLLM struct {
	Response              string
	Err                   error
	GenerateTextFunc      func(ctx context.Context, name, prompt string) (string, error)
	GenerateImageTextFunc func(ctx context.Context, name, prompt, imagePath string) (string, error)
	HasProfileVal         bool                   // Controls HasProfile return value (defaults to false)
	HasProfileFunc        func(name string) bool // Function to control HasProfile return value

	GenerateTextCalls      int
	GenerateImageTextCalls int
}

func (m *MockLLM) Name() string                         { return "MockLLM" }
func (m *MockLLM) Configure(cfg config.LLMConfig) error { return nil }
func (m *MockLLM) GenerateText(ctx context.Context, name, prompt string) (string, error) {
	m.GenerateTextCalls++
	if m.GenerateTextFunc != nil {
		return m.GenerateTextFunc(ctx, name, prompt)
	}
	return m.Response, m.Err
}
func (m *MockLLM) GenerateJSON(ctx context.Context, name, prompt string, target any) error {
	return nil
}
func (m *MockLLM) HealthCheck(ctx context.Context) error { return nil }
func (m *MockLLM) HasProfile(name string) bool {
	if m.HasProfileFunc != nil {
		return m.HasProfileFunc(name)
	}
	return m.HasProfileVal
}
func (m *MockLLM) GenerateImageText(ctx context.Context, name, prompt, imagePath string) (string, error) {
	m.GenerateImageTextCalls++
	if m.GenerateImageTextFunc != nil {
		return m.GenerateImageTextFunc(ctx, name, prompt, imagePath)
	}
	return m.Response, m.Err
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
	PlayCalls       int
	LastFile        string
	PlayErr         error
	IsPlayingVal    bool
	IsUserPausedVal bool
	CanReplay       bool
	Replayed        bool
}

func (m *MockAudio) Play(filepath string, startPaused bool, onComplete func()) error {
	m.PlayCalls++
	m.LastFile = filepath
	m.IsPlayingVal = true
	// Simulate async completion if callback provided
	if onComplete != nil {
		go func() {
			time.Sleep(10 * time.Millisecond)
			onComplete()
		}()
	}
	return m.PlayErr
}
func (m *MockAudio) Pause()                    {}
func (m *MockAudio) Resume()                   {}
func (m *MockAudio) Stop()                     {}
func (m *MockAudio) Shutdown()                 {}
func (m *MockAudio) IsPlaying() bool           { return m.IsPlayingVal }
func (m *MockAudio) IsBusy() bool              { return m.IsPlayingVal }
func (m *MockAudio) IsPaused() bool            { return m.IsUserPausedVal }
func (m *MockAudio) SetVolume(vol float64)     {}
func (m *MockAudio) Volume() float64           { return 1.0 }
func (m *MockAudio) SetUserPaused(paused bool) { m.IsUserPausedVal = paused }
func (m *MockAudio) IsUserPaused() bool        { return m.IsUserPausedVal }
func (m *MockAudio) ResetUserPause()           { m.IsUserPausedVal = false }
func (m *MockAudio) LastNarrationFile() string { return m.LastFile }
func (m *MockAudio) ReplayLastNarration(onComplete func()) bool {
	if m.CanReplay {
		m.Replayed = true
		if onComplete != nil {
			go func() {
				time.Sleep(10 * time.Millisecond)
				onComplete()
			}()
		}
		return true
	}
	return false
}
func (m *MockAudio) Position() time.Duration { return 0 }
func (m *MockAudio) Duration() time.Duration {
	return time.Second * 10
}

func (m *MockAudio) Remaining() time.Duration {
	return 0
}

func (m *MockAudio) AverageLatency() time.Duration {
	return 0
}

type MockPOIProvider struct {
	GetPOIFunc func(ctx context.Context, qid string) (*model.POI, error)

	GetBestFunc               func(isOnGround bool) *model.POI
	CountScoredAboveFunc      func(threshold float64, limit int) int
	GetFilteredCandidatesFunc func(filterMode string, targetCount int, minScore float64, isOnGround bool) ([]*model.POI, float64)

	GetNarrationCandidatesFunc func(limit int, minScore *float64) []*model.POI
}

func (m *MockPOIProvider) GetPOI(ctx context.Context, qid string) (*model.POI, error) {
	if m.GetPOIFunc != nil {
		return m.GetPOIFunc(ctx, qid)
	}
	return nil, nil
}

func (m *MockPOIProvider) CountScoredAbove(threshold float64, limit int) int {
	if m.CountScoredAboveFunc != nil {
		return m.CountScoredAboveFunc(threshold, limit)
	}
	return 0 // default for tests
}

func (m *MockPOIProvider) LastScoredPosition() (lat, lon float64) {
	return 0, 0
}

func (m *MockPOIProvider) GetNarrationCandidates(limit int, minScore *float64) []*model.POI {
	if m.GetNarrationCandidatesFunc != nil {
		return m.GetNarrationCandidatesFunc(limit, minScore)
	}
	return []*model.POI{}
}

func (m *MockPOIProvider) GetFilteredCandidates(filterMode string, targetCount int, minScore float64, isOnGround bool) ([]*model.POI, float64) {
	return []*model.POI{}, 0.0
}

type MockGeo struct {
	Country string
	City    string
}

func (m *MockGeo) GetCountry(lat, lon float64) string {
	return m.Country
}

func (m *MockGeo) GetLocation(lat, lon float64) model.LocationInfo {
	city := m.City
	if city == "" {
		city = "MockCity"
	}
	return model.LocationInfo{
		CityName:    city,
		CountryCode: m.Country,
	}
}

type MockWikipedia struct {
	Content string
	Err     error
}

func (m *MockWikipedia) GetArticleContent(ctx context.Context, title, lang string) (string, error) {
	return m.Content, m.Err
}

func (m *MockWikipedia) GetArticleHTML(ctx context.Context, title, lang string) (string, error) {
	return m.Content, m.Err // Return same content as HTML for simplicity in tests
}

type MockBeacon struct {
	Cleared    bool
	TargetSet  bool
	LastTgtLat float64
	LastTgtLon float64
}

func (m *MockBeacon) SetTarget(ctx context.Context, lat, lon float64) error {
	m.TargetSet = true
	m.LastTgtLat = lat
	m.LastTgtLon = lon
	return nil
}

func (m *MockBeacon) Clear() {
	m.Cleared = true
	m.TargetSet = false
}

type MockSim struct {
	Telemetry  sim.Telemetry
	Err        error
	PredWindow time.Duration
}

func (m *MockSim) GetTelemetry(ctx context.Context) (sim.Telemetry, error) {
	return m.Telemetry, m.Err
}
func (m *MockSim) GetState() sim.State { return sim.StateActive }
func (m *MockSim) SetPredictionWindow(d time.Duration) {
	m.PredWindow = d
}
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
	State      map[string]string
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
func (m *MockStore) GetGeodataCache(ctx context.Context, key string) ([]byte, int, bool) {
	return nil, 0, false
}
func (m *MockStore) SetGeodataCache(ctx context.Context, key string, val []byte, radius int, lat, lon float64) error {
	return nil
}
func (m *MockStore) GetGeodataInBounds(ctx context.Context, minLat, maxLat, minLon, maxLon float64) ([]store.GeodataRecord, error) {
	return nil, nil
}
func (m *MockStore) ListGeodataCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}

func (m *MockStore) GetState(ctx context.Context, key string) (string, bool) {
	if m.State == nil {
		return "", false
	}
	val, ok := m.State[key]
	return val, ok
}

func (m *MockStore) SetState(ctx context.Context, key, val string) error {
	if m.State == nil {
		m.State = make(map[string]string)
	}
	m.State[key] = val
	return nil
}

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
