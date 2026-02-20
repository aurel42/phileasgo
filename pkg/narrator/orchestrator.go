package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"phileasgo/pkg/announcement"
	"phileasgo/pkg/audio"
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/model"
	"phileasgo/pkg/playback"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/session"
	"phileasgo/pkg/sim"
)

// Orchestrator manages the coordination between generation and playback.
type Orchestrator struct {
	gen        Generator
	audio      audio.Service
	q          *playback.Manager
	sessionMgr *session.Manager
	beaconSvc  BeaconProvider
	sim        sim.Client

	mu sync.RWMutex

	// Playback State
	active               bool
	currentPOI           *model.POI
	currentTitle         string
	currentType          model.NarrativeType
	currentImagePath     string
	currentThumbnailURL  string
	currentShowInfoPanel bool
	currentLat           float64
	currentLon           float64
	currentDuration      time.Duration

	// Replay State
	lastPOI       *model.POI
	lastImagePath string
	lastLat       float64
	lastLon       float64

	pacingDuration time.Duration
	skipCooldown   bool

	// Beacon Registry & Rotation
	beaconRegistry config.BeaconRegistry
	colorKeys      []string
	colorIndex     int
}

// NewOrchestrator creates a new narrator orchestrator.
func NewOrchestrator(
	gen Generator,
	audioMgr audio.Service,
	q *playback.Manager,
	sessionMgr *session.Manager,
	beaconSvc BeaconProvider,
	simClient sim.Client,
	beaconRegistry config.BeaconRegistry,
	beaconOrder []string,
) *Orchestrator {
	return &Orchestrator{
		gen:            gen,
		audio:          audioMgr,
		q:              q,
		sessionMgr:     sessionMgr,
		beaconSvc:      beaconSvc,
		sim:            simClient,
		beaconRegistry: beaconRegistry,
		colorKeys:      beaconOrder,
		pacingDuration: 3 * time.Second,
	}
}

func (o *Orchestrator) Start() {
	o.gen.ProcessGenerationQueue(context.Background())
}

func (o *Orchestrator) Stop() {
	o.audio.Shutdown()
}

func (o *Orchestrator) IsActive() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.active || o.gen.IsGenerating() || o.q.Count() > 0 || o.gen.HasPendingGeneration()
}

func (o *Orchestrator) IsGenerating() bool {
	return o.gen.IsGenerating()
}

func (o *Orchestrator) NarratedCount() int {
	return o.sessionMgr.NarratedCount()
}

func (o *Orchestrator) Stats() map[string]any {
	stats := o.gen.Stats()
	o.mu.RLock()
	defer o.mu.RUnlock()
	stats["playback_active"] = o.active
	stats["playback_queue_len"] = o.q.Count()
	return stats
}

func (o *Orchestrator) IsPlaying() bool {
	return o.audio.IsBusy()
}

func (o *Orchestrator) PlayPOI(ctx context.Context, poiID string, manual, enqueueIfBusy bool, tel *sim.Telemetry, strategy string) {
	// Immediate Visual Update (Marker Preview)
	if o.beaconSvc != nil {
		if pm := o.POIManager(); pm != nil {
			if p, err := pm.GetPOI(ctx, poiID); err == nil && p != nil {
				o.assignBeaconColor(p)
				entry := o.getRegistryEntryByColor(p.BeaconColor)
				if entry != nil {
					_ = o.beaconSvc.SetTarget(ctx, p.Lat, p.Lon, entry.Title, entry.Livery)
				}
			}
		}
	}

	// Deduplication/Promotion check
	if o.promoteInQueue(poiID, manual) {
		go o.ProcessPlaybackQueue(context.Background())
		return
	}

	// Delegate generation to generator
	// We might need to cast to *AIService for now or expand Generator interface
	if ai, ok := o.gen.(interface {
		PlayPOI(ctx context.Context, poiID string, manual, enqueueIfBusy bool, tel *sim.Telemetry, strategy string)
	}); ok {
		ai.PlayPOI(ctx, poiID, manual, enqueueIfBusy, tel, strategy)
	}
}

func (o *Orchestrator) PrepareNextNarrative(ctx context.Context, poiID, strategy string, tel *sim.Telemetry) error {
	if ai, ok := o.gen.(interface {
		PrepareNextNarrative(ctx context.Context, poiID, strategy string, tel *sim.Telemetry) error
	}); ok {
		return ai.PrepareNextNarrative(ctx, poiID, strategy, tel)
	}
	return fmt.Errorf("generator does not support pipeline preparation")
}

func (o *Orchestrator) GetPreparedPOI() *model.POI {
	if next := o.q.Peek(); next != nil && next.POI != nil {
		return next.POI
	}
	// Fallback to what generator is doing
	if ai, ok := o.gen.(interface{ GetPreparedPOI() *model.POI }); ok {
		return ai.GetPreparedPOI()
	}
	return nil
}

func (o *Orchestrator) HasStagedAuto() bool {
	if o.q.HasAuto() {
		return true
	}
	if ai, ok := o.gen.(interface{ IsGenerating() bool }); ok {
		return ai.IsGenerating()
	}
	return false
}

func (o *Orchestrator) HasPendingManualOverride() bool {
	return o.gen.HasPendingGeneration()
}

func (o *Orchestrator) GetPendingManualOverride() (poiID, strategy string, ok bool) {
	if ai, ok := o.gen.(interface {
		GetPendingManualOverride() (poiID, strategy string, ok bool)
	}); ok {
		return ai.GetPendingManualOverride()
	}
	return "", "", false
}

func (o *Orchestrator) GenerateNarrative(ctx context.Context, req *GenerationRequest) (*model.Narrative, error) {
	return o.gen.GenerateNarrative(ctx, req)
}

func (o *Orchestrator) ProcessPlaybackQueue(ctx context.Context) {
	if o.IsPaused() {
		return
	}

	o.mu.Lock()
	if o.active {
		o.mu.Unlock()
		return
	}
	if o.q.Count() == 0 {
		o.mu.Unlock()
		return
	}
	o.mu.Unlock()

	next := o.q.Pop()
	if next == nil {
		return
	}

	if err := o.PlayNarrative(ctx, next); err != nil {
		slog.Error("Orchestrator: Playback failed", "error", err)
		go o.ProcessPlaybackQueue(ctx)
	}
}

func (o *Orchestrator) PlayNarrative(ctx context.Context, n *model.Narrative) error {
	o.mu.Lock()
	if o.active {
		o.mu.Unlock()
		return fmt.Errorf("already active")
	}
	o.mu.Unlock()

	audioFile := o.setPlaybackState(n)

	if err := o.audio.Play(audioFile, false, o.finalizePlayback); err != nil {
		o.mu.Lock()
		o.active = false
		o.mu.Unlock()
		return err
	}

	// Post-play logic (session, state, logging)
	if n.POI != nil {
		n.POI.LastPlayed = time.Now()
		// Persist to DB so cooldown survives eviction/teleport/restart
		if pm := o.POIManager(); pm != nil {
			go pm.SaveLastPlayed(context.Background(), n.POI.WikidataID, n.POI.LastPlayed)
		}
		// Spawn colored beacon in MSFS
		o.assignBeaconColor(n.POI)
		if o.beaconSvc != nil {
			entry := o.getRegistryEntryByColor(n.POI.BeaconColor)
			if entry != nil {
				go func() {
					if err := o.beaconSvc.SetTarget(context.Background(), n.POI.Lat, n.POI.Lon, entry.Title, entry.Livery); err != nil {
						slog.Error("Orchestrator: Failed to spawn beacon", "error", err)
					}
				}()
			}
		}
	}
	// Record the event
	o.gen.RecordNarration(ctx, n)

	return nil
}

func (o *Orchestrator) setPlaybackState(n *model.Narrative) string {
	ext := "." + n.Format
	audioFile := n.AudioPath
	if !strings.HasSuffix(strings.ToLower(audioFile), ext) {
		audioFile += ext
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	o.active = true
	o.currentTitle = n.Title
	o.currentType = n.Type
	o.currentDuration = n.Duration
	o.currentThumbnailURL = n.ThumbnailURL
	o.currentShowInfoPanel = n.ShowInfoPanel
	o.currentImagePath = n.ImagePath
	o.currentPOI = n.POI
	o.currentLat = n.Lat
	o.currentLon = n.Lon

	if n.POI != nil {
		o.lastPOI = n.POI
	}
	if n.ImagePath != "" {
		o.lastImagePath = n.ImagePath
		o.lastLat = n.Lat
		o.lastLon = n.Lon
	}

	return audioFile
}

func (o *Orchestrator) finalizePlayback() {
	// If Skip was called, audio.Stop() should have triggered finalizePlayback
	// via the onComplete callback. We just need to make sure we don't sleep
	// if we're skipping.
	if !o.ShouldSkipCooldown() {
		time.Sleep(o.pacingDuration)
	}

	o.mu.Lock()
	o.active = false
	o.currentPOI = nil
	o.currentImagePath = ""
	o.currentType = ""
	o.currentThumbnailURL = ""
	o.currentTitle = ""
	o.currentShowInfoPanel = false
	o.currentDuration = 0
	o.mu.Unlock()

	// Beacon Check (Switch to next target)
	if o.beaconSvc != nil {
		next := o.q.Peek()
		// If next in queue is a POI, point the beacon there
		if next != nil && next.POI != nil {
			slog.Info("Orchestrator: Switching marker to next queued POI", "qid", next.POI.WikidataID)
			o.assignBeaconColor(next.POI)
			entry := o.getRegistryEntryByColor(next.POI.BeaconColor)
			if entry != nil {
				_ = o.beaconSvc.SetTarget(context.Background(), next.POI.Lat, next.POI.Lon, entry.Title, entry.Livery)
			}
		} else if ai, ok := o.gen.(interface {
			GetPreparedPOI() *model.POI
		}); ok {
			generating := ai.GetPreparedPOI()
			if generating != nil {
				slog.Info("Orchestrator: Switching marker to currently generating POI", "qid", generating.WikidataID)
				o.assignBeaconColor(generating)
				entry := o.getRegistryEntryByColor(generating.BeaconColor)
				if entry != nil {
					_ = o.beaconSvc.SetTarget(context.Background(), generating.Lat, generating.Lon, entry.Title, entry.Livery)
				}
			}
		}
	}

	go o.ProcessPlaybackQueue(context.Background())
}

func (o *Orchestrator) SkipCooldown()            { o.skipCooldown = true }
func (o *Orchestrator) ShouldSkipCooldown() bool { return o.skipCooldown }
func (o *Orchestrator) ResetSkipCooldown()       { o.skipCooldown = false }
func (o *Orchestrator) IsPaused() bool           { return o.audio.IsUserPaused() }
func (o *Orchestrator) CurrentPOI() *model.POI {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.currentPOI
}
func (o *Orchestrator) CurrentTitle() string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// For announcements/special types, prioritize the specific title if set
	if o.currentType != "" && o.currentType != model.NarrativeTypePOI {
		if o.currentTitle != "" {
			return o.currentTitle
		}
	}

	if o.currentPOI != nil {
		return o.currentPOI.DisplayName()
	}
	if o.active {
		if o.currentTitle != "" {
			return o.currentTitle
		}
		if o.currentType == model.NarrativeTypeScreenshot {
			return "Photograph"
		}
	}
	return ""
}
func (o *Orchestrator) CurrentType() model.NarrativeType {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.currentType
}
func (o *Orchestrator) Remaining() time.Duration { return o.audio.Remaining() }
func (o *Orchestrator) CurrentImagePath() string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.currentImagePath
}
func (o *Orchestrator) CurrentThumbnailURL() string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.currentThumbnailURL
}
func (o *Orchestrator) CurrentShowInfoPanel() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.currentShowInfoPanel
}
func (o *Orchestrator) ClearCurrentImage() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.currentImagePath = ""
}
func (o *Orchestrator) CurrentDuration() time.Duration {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.currentDuration
}

// IsPOIBusy returns true if the POI is currently generating or queued.
func (o *Orchestrator) IsPOIBusy(poiID string) bool {
	return o.q.HasPOI(poiID) || o.gen.IsPOIBusy(poiID) || (o.currentPOI != nil && o.currentPOI.WikidataID == poiID)
}

func (o *Orchestrator) Pause()  { o.audio.SetUserPaused(true); o.audio.Pause() }
func (o *Orchestrator) Resume() { o.audio.ResetUserPause(); o.audio.Resume() }
func (o *Orchestrator) Skip() {
	o.mu.Lock()
	if !o.active {
		o.mu.Unlock()
		return
	}
	o.mu.Unlock()

	slog.Info("Orchestrator: Skipping current narration", "title", o.currentTitle)
	o.SkipCooldown()
	o.audio.Stop()
	// audio.Stop() will trigger finalizePlayback via the onComplete callback
}
func (o *Orchestrator) TriggerIdentAction() {
	// Implement based on what we see in AIService
}

func (o *Orchestrator) promoteInQueue(poiID string, manual bool) bool {
	if manual {
		return o.q.Promote(poiID)
	}
	return o.q.HasPOI(poiID)
}

func (o *Orchestrator) ReplayLast(ctx context.Context) bool {
	if !o.audio.ReplayLastNarration(o.finalizePlayback) {
		return false
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	if o.lastPOI != nil {
		o.currentPOI = o.lastPOI
		o.active = true
		return true
	}
	if o.lastImagePath != "" {
		o.currentImagePath = o.lastImagePath
		o.active = true
		return true
	}
	return true
}

func (o *Orchestrator) AverageLatency() time.Duration {
	// Average latency is a generational metric
	if ai, ok := o.gen.(interface{ AverageLatency() time.Duration }); ok {
		return ai.AverageLatency()
	}
	return 60 * time.Second
}

func (o *Orchestrator) Reset(ctx context.Context) {
	o.mu.Lock()
	o.q.Clear()
	o.currentPOI = nil
	o.lastPOI = nil
	o.active = false
	o.mu.Unlock()

	if ai, ok := o.gen.(interface{ Reset(ctx context.Context) }); ok {
		ai.Reset(ctx)
	}
}

func (o *Orchestrator) Play(n *model.Narrative) {
	o.EnqueuePlayback(n, true)
}

func (o *Orchestrator) AudioService() audio.Service {
	return o.audio
}

func (o *Orchestrator) POIManager() POIProvider {
	// Orchestrator needs to expose POIMgr for some callers
	if ai, ok := o.gen.(interface{ POIManager() POIProvider }); ok {
		return ai.POIManager()
	}
	return nil
}

func (o *Orchestrator) LLMProvider() llm.Provider {
	return o.gen.LLMProvider()
}

func (o *Orchestrator) ProcessGenerationQueue(ctx context.Context) {
	o.gen.ProcessGenerationQueue(ctx)
}

func (o *Orchestrator) HasPendingGeneration() bool {
	return o.gen.HasPendingGeneration()
}

func (o *Orchestrator) ResetSession(ctx context.Context) {
	o.Reset(ctx)
}

func (o *Orchestrator) PlayEssay(ctx context.Context, tel *sim.Telemetry) bool {
	return o.gen.PlayEssay(ctx, tel)
}

// DataProvider Implementation (Delegated to Generator)
func (o *Orchestrator) GetLocation(lat, lon float64) model.LocationInfo {
	if ai, ok := o.gen.(announcement.DataProvider); ok {
		return ai.GetLocation(lat, lon)
	}
	return model.LocationInfo{}
}

func (o *Orchestrator) IsUserPaused() bool {
	return o.audio.IsUserPaused()
}

func (o *Orchestrator) GetPOIsNear(lat, lon, radius float64) []*model.POI {
	if ai, ok := o.gen.(announcement.DataProvider); ok {
		return ai.GetPOIsNear(lat, lon, radius)
	}
	return nil
}

func (o *Orchestrator) GetRepeatTTL() time.Duration {
	if ai, ok := o.gen.(announcement.DataProvider); ok {
		return ai.GetRepeatTTL()
	}
	return 0
}

func (o *Orchestrator) GetLastTransition(stage string) time.Time {
	if ai, ok := o.gen.(interface{ GetLastTransition(string) time.Time }); ok {
		return ai.GetLastTransition(stage)
	}
	return time.Time{}
}

func (o *Orchestrator) AssemblePOI(ctx context.Context, p *model.POI, t *sim.Telemetry, strategy string) prompt.Data {
	if ai, ok := o.gen.(announcement.DataProvider); ok {
		return ai.AssemblePOI(ctx, p, t, strategy)
	}
	return nil
}

func (o *Orchestrator) AssembleGeneric(ctx context.Context, t *sim.Telemetry) prompt.Data {
	if ai, ok := o.gen.(announcement.DataProvider); ok {
		return ai.AssembleGeneric(ctx, t)
	}
	return nil
}

func (o *Orchestrator) assignBeaconColor(p *model.POI) {
	if p.BeaconColor != "" {
		return // Already assigned
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	if len(o.colorKeys) == 0 {
		return
	}

	key := o.colorKeys[o.colorIndex%len(o.colorKeys)]
	o.colorIndex++

	entry := o.beaconRegistry[key]

	// Reclaim: clear this color from any POI that currently holds it
	if pm := o.POIManager(); pm != nil {
		pm.ClearBeaconColor(entry.MapColor)
	}

	p.BeaconColor = entry.MapColor
	slog.Info("Orchestrator: Assigned beacon color to POI", "poi", p.WikidataID, "color", key, "hex", p.BeaconColor)
}

func (o *Orchestrator) getRegistryEntryByColor(hexColor string) *config.BeaconRegistryEntry {
	for _, entry := range o.beaconRegistry {
		if entry.MapColor == hexColor {
			return &entry
		}
	}
	return nil
}

// Internal methods for AIService to call
func (o *Orchestrator) EnqueuePlayback(n *model.Narrative, priority bool) {
	o.q.Enqueue(n, priority)
	go o.ProcessPlaybackQueue(context.Background())
}
