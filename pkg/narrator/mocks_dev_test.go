package narrator

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"phileasgo/pkg/announcement"
	"phileasgo/pkg/audio"
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/model"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"phileasgo/pkg/tts"
)

// --- Mocks ---

type MockLLM struct {
	mu                    sync.RWMutex
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
	m.mu.Lock()
	m.GenerateTextCalls++
	fn := m.GenerateTextFunc
	resp := m.Response
	err := m.Err
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, name, prompt)
	}
	return resp, err
}
func (m *MockLLM) GenerateJSON(ctx context.Context, name, prompt string, target any) error {
	return nil
}
func (m *MockLLM) HealthCheck(ctx context.Context) error    { return nil }
func (m *MockLLM) ValidateModels(ctx context.Context) error { return m.Err }
func (m *MockLLM) HasProfile(name string) bool {
	if m.HasProfileFunc != nil {
		return m.HasProfileFunc(name)
	}
	return m.HasProfileVal
}
func (m *MockLLM) GenerateImageText(ctx context.Context, name, prompt, imagePath string) (string, error) {
	m.mu.Lock()
	m.GenerateImageTextCalls++
	fn := m.GenerateImageTextFunc
	resp := m.Response
	err := m.Err
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, name, prompt, imagePath)
	}
	return resp, err
}

type MockTTS struct {
	Format          string
	Err             error
	SynthesizeFunc  func(ctx context.Context, text, voiceID, outputPath string) (string, error)
	SynthesizeCalls int
}

func (m *MockTTS) Voices(ctx context.Context) ([]tts.Voice, error) {
	return []tts.Voice{{ID: "voice-f", Name: "Female Voice"}}, nil
}
func (m *MockTTS) Synthesize(ctx context.Context, text, voiceID, outputPath string) (string, error) {
	m.SynthesizeCalls++
	if m.SynthesizeFunc != nil {
		return m.SynthesizeFunc(ctx, text, voiceID, outputPath)
	}

	ext := m.Format
	if ext == "" {
		ext = "mp3"
	}

	if m.Err == nil {
		fullPath := outputPath
		if !strings.HasSuffix(strings.ToLower(fullPath), "."+ext) {
			fullPath += "." + ext
		}
		_ = os.WriteFile(fullPath, make([]byte, tts.MinAudioSize+1), 0644)
	}
	return ext, m.Err
}

type MockAudio struct {
	mu              sync.RWMutex
	PlayCalls       int
	LastFile        string
	PlayErr         error
	IsPlayingVal    bool
	IsUserPausedVal bool
	CanReplay       bool
	Replayed        bool
	PlaySync        bool
}

func (m *MockAudio) Play(filepath string, startPaused bool, onComplete func()) error {
	m.mu.Lock()
	m.PlayCalls++
	m.LastFile = filepath
	m.IsPlayingVal = true
	err := m.PlayErr
	m.mu.Unlock()

	// Simulate async completion if callback provided
	if onComplete != nil {
		if m.PlaySync {
			onComplete()
		} else {
			go func() {
				time.Sleep(10 * time.Millisecond)
				onComplete()
			}()
		}
	}
	return err
}
func (m *MockAudio) Pause()                {}
func (m *MockAudio) Resume()               {}
func (m *MockAudio) Stop()                 {}
func (m *MockAudio) Shutdown()             {}
func (m *MockAudio) IsPlaying() bool       { m.mu.RLock(); defer m.mu.RUnlock(); return m.IsPlayingVal }
func (m *MockAudio) IsBusy() bool          { m.mu.RLock(); defer m.mu.RUnlock(); return m.IsPlayingVal }
func (m *MockAudio) IsPaused() bool        { m.mu.RLock(); defer m.mu.RUnlock(); return m.IsUserPausedVal }
func (m *MockAudio) SetVolume(vol float64) {}
func (m *MockAudio) Volume() float64       { return 1.0 }
func (m *MockAudio) SetUserPaused(paused bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.IsUserPausedVal = paused
}
func (m *MockAudio) IsUserPaused() bool        { m.mu.RLock(); defer m.mu.RUnlock(); return m.IsUserPausedVal }
func (m *MockAudio) ResetUserPause()           { m.mu.Lock(); defer m.mu.Unlock(); m.IsUserPausedVal = false }
func (m *MockAudio) LastNarrationFile() string { m.mu.RLock(); defer m.mu.RUnlock(); return m.LastFile }
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
	GetPOIsNearFunc            func(lat, lon, radiusMeters float64) []*model.POI
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

func (m *MockPOIProvider) GetPOIsNear(lat, lon, radiusMeters float64) []*model.POI {
	if m.GetPOIsNearFunc != nil {
		return m.GetPOIsNearFunc(lat, lon, radiusMeters)
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

func (m *MockBeacon) SetTarget(ctx context.Context, lat, lon float64, title, livery string) error {
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
	Telemetry   sim.Telemetry
	Err         error
	PredWindow  time.Duration
	Transitions map[string]time.Time
}

func (m *MockSim) GetTelemetry(ctx context.Context) (sim.Telemetry, error) {
	return m.Telemetry, m.Err
}
func (m *MockSim) GetState() sim.State { return sim.StateActive }
func (m *MockSim) SetPredictionWindow(d time.Duration) {
	m.PredWindow = d
}
func (m *MockSim) GetLastTransition(stage string) time.Time {
	if m.Transitions == nil {
		return time.Time{}
	}
	return m.Transitions[stage]
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

func (m *MockSim) ExecuteCommand(ctx context.Context, cmd string, args map[string]any) error {
	return nil
}

func (m *MockSim) GetStageState() sim.StageState {
	return sim.StageState{
		Current:        "cruise",
		LastTransition: m.Transitions,
	}
}
func (m *MockSim) RestoreStageState(s sim.StageState) {
	m.Transitions = s.LastTransition
}

func (m *MockSim) SetEventRecorder(r sim.EventRecorder) {
	// No-op for mock
}

type MockStore struct {
	mu         sync.RWMutex
	SavedPOIs  []*model.POI
	Articles   map[string]*model.Article
	RecentPOIs []*model.POI
	State      map[string]string
}

func (m *MockStore) SavePOI(ctx context.Context, p *model.POI) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SavedPOIs = append(m.SavedPOIs, p)
	return nil
}
func (m *MockStore) GetPOI(ctx context.Context, qid string) (*model.POI, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Mock returns nil by default or we could add more logic here
	return nil, nil
}
func (m *MockStore) GetPOIsBatch(ctx context.Context, ids []string) (map[string]*model.POI, error) {
	return nil, nil
}
func (m *MockStore) GetRecentlyPlayedPOIs(ctx context.Context, since time.Time) ([]*model.POI, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.RecentPOIs, nil
}
func (m *MockStore) ResetLastPlayed(ctx context.Context, lat, lon, radius float64) error { return nil }
func (m *MockStore) SaveArticle(ctx context.Context, a *model.Article) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Articles == nil {
		m.Articles = make(map[string]*model.Article)
	}
	m.Articles[a.UUID] = a
	return nil
}
func (m *MockStore) GetArticle(ctx context.Context, uuid string) (*model.Article, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
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
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.State == nil {
		return "", false
	}
	val, ok := m.State[key]
	return val, ok
}

func (m *MockStore) SetState(ctx context.Context, key, val string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.State == nil {
		m.State = make(map[string]string)
	}
	m.State[key] = val
	return nil
}

func (m *MockStore) DeleteState(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.State != nil {
		delete(m.State, key)
	}
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

type MockAIService struct {
	GeneratingVal bool
	ManualVal     bool
	OnPlayback    func(n *model.Narrative, priority bool)
	PreparedPOI   *model.POI
}

func (m *MockAIService) Start()                                                      {}
func (m *MockAIService) Stop()                                                       {}
func (m *MockAIService) IsActive() bool                                              { return m.GeneratingVal }
func (m *MockAIService) IsGenerating() bool                                          { return m.GeneratingVal }
func (m *MockAIService) IsPlaying() bool                                             { return false }
func (m *MockAIService) ProcessPlaybackQueue(ctx context.Context)                    {}
func (m *MockAIService) PlayNarrative(ctx context.Context, n *model.Narrative) error { return nil }
func (m *MockAIService) SkipCooldown()                                               {}
func (m *MockAIService) ShouldSkipCooldown() bool                                    { return false }
func (m *MockAIService) ResetSkipCooldown()                                          {}
func (m *MockAIService) IsPaused() bool                                              { return false }
func (m *MockAIService) CurrentPOI() *model.POI                                      { return nil }
func (m *MockAIService) CurrentTitle() string                                        { return "" }
func (m *MockAIService) CurrentType() model.NarrativeType                            { return "" }
func (m *MockAIService) CurrentDuration() time.Duration                              { return 0 }
func (m *MockAIService) Remaining() time.Duration                                    { return 0 }
func (m *MockAIService) ReplayLast(ctx context.Context) bool                         { return false }
func (m *MockAIService) CurrentImagePath() string                                    { return "" }
func (m *MockAIService) CurrentThumbnailURL() string                                 { return "" }
func (m *MockAIService) ClearCurrentImage()                                          {}
func (m *MockAIService) Pause()                                                      {}
func (m *MockAIService) Resume()                                                     {}
func (m *MockAIService) Skip()                                                       {}
func (m *MockAIService) TriggerIdentAction()                                         {}
func (m *MockAIService) HasStagedAuto() bool                                         { return false }
func (m *MockAIService) HasPendingManualOverride() bool                              { return false }
func (m *MockAIService) GetPendingManualOverride() (string, string, bool)            { return "", "", false }
func (m *MockAIService) POIManager() POIProvider                                     { return nil }
func (m *MockAIService) LLMProvider() llm.Provider                                   { return nil }
func (m *MockAIService) AudioService() audio.Service                                 { return nil }
func (m *MockAIService) ProcessGenerationQueue(ctx context.Context)                  {}
func (m *MockAIService) HasPendingGeneration() bool                                  { return false }
func (m *MockAIService) ResetSession(ctx context.Context)                            {}

func (m *MockAIService) HasProfile(name string) bool   { return false }
func (m *MockAIService) NarratedCount() int            { return 0 }
func (m *MockAIService) Stats() map[string]any         { return nil }
func (m *MockAIService) AverageLatency() time.Duration { return 0 }

func (m *MockAIService) SetOnPlayback(cb func(n *model.Narrative, priority bool)) {
	m.OnPlayback = cb
}
func (m *MockAIService) PlayPOI(ctx context.Context, poiID string, manual, enqueueIfBusy bool, tel *sim.Telemetry, strategy string) {
}
func (m *MockAIService) PrepareNextNarrative(ctx context.Context, poiID, strategy string, tel *sim.Telemetry) error {
	return nil
}
func (m *MockAIService) GetPreparedPOI() *model.POI {
	return m.PreparedPOI
}
func (m *MockAIService) AssembleGeneric(ctx context.Context, t *sim.Telemetry) prompt.Data {
	return nil
}
func (m *MockAIService) RecordNarration(ctx context.Context, n *model.Narrative) {}

func (m *MockAIService) PlayEssay(ctx context.Context, tel *sim.Telemetry) bool {
	return false
}
func (m *MockAIService) IsPOIBusy(poiID string) bool {
	return false
}
func (m *MockAIService) GenerateNarrative(ctx context.Context, req *GenerationRequest) (*model.Narrative, error) {
	return &model.Narrative{Type: req.Type, Title: req.Title}, nil
}

func (m *MockAIService) EnqueueAnnouncement(ctx context.Context, a announcement.Item, t *sim.Telemetry, onComplete func(*model.Narrative)) {
}

func (m *MockAIService) Reset(ctx context.Context) {}
