package narrator

import (
	"context"
	"os"
	"path/filepath"
	"phileasgo/pkg/announcement"
	"phileasgo/pkg/config"
	"phileasgo/pkg/generation"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/playback"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/session"
	"phileasgo/pkg/sim"
	"testing"
)

func TestOrchestrator_QueueManagement(t *testing.T) {
	mockGen := &MockAIService{}
	pbQ := playback.NewManager()
	sess := session.NewManager(nil)
	o := NewOrchestrator(mockGen, &MockAudio{}, pbQ, sess, nil, nil)

	// 1. Enqueue via EnqueuePlayback
	// Force active state to prevent auto-pop
	o.mu.Lock()
	o.active = true
	o.mu.Unlock()

	o.EnqueuePlayback(&model.Narrative{Title: "Auto", Manual: false, Type: model.NarrativeTypePOI}, false)
	if pbQ.Count() != 1 {
		t.Errorf("expected 1 item in queue, got %d", pbQ.Count())
	}

	// 2. Enqueue Priority Manual POI
	o.EnqueuePlayback(&model.Narrative{Title: "Manual", Manual: true, Type: model.NarrativeTypePOI}, true)
	if pbQ.Count() != 2 || pbQ.Peek().Title != "Manual" {
		t.Error("priority item should be at the front")
	}

	// 3. Reset Session
	o.ResetSession(context.Background())
	if pbQ.Count() != 0 {
		t.Error("expected empty queue after reset")
	}

	// 4. promoteInQueue - Boost case
	pbQ.Enqueue(&model.Narrative{POI: &model.POI{WikidataID: "Q1"}, Type: model.NarrativeTypePOI}, false)
	pbQ.Enqueue(&model.Narrative{POI: &model.POI{WikidataID: "Q2"}, Type: model.NarrativeTypePOI}, false)

	if !o.promoteInQueue("Q2", true) {
		t.Error("expected true (found Q2)")
	}
	if pbQ.Peek().POI.WikidataID != "Q2" {
		t.Error("Q2 should be boosted to front")
	}
}

func TestAIService_QueueCallback(t *testing.T) {
	svc := &AIService{}
	var called bool
	svc.SetOnPlayback(func(n *model.Narrative, priority bool) {
		called = true
	})

	svc.enqueuePlayback(&model.Narrative{Title: "Test"}, false)
	if !called {
		t.Error("expected onPlayback callback to be called")
	}
}

func TestAIService_ManualOverrides(t *testing.T) {
	svc := &AIService{
		genQ: generation.NewManager(),
	}

	svc.pendingManualID = "Q1"
	svc.pendingManualStrategy = "short"
	if !svc.HasPendingManualOverride() {
		t.Error("expected pending override")
	}
	id, _, ok := svc.GetPendingManualOverride()
	if !ok || id != "Q1" {
		t.Errorf("expected Q1, got %s (ok=%v)", id, ok)
	}
}

func TestAIService_RecordNarration(t *testing.T) {
	// Setup Prompts Dir
	tmpDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpDir, "narrator"), 0o755)
	_ = os.WriteFile(filepath.Join(tmpDir, "narrator", "event_summary.tmpl"), []byte("Summary: {{.LastTitle}}"), 0o644)
	pm, _ := prompts.NewManager(tmpDir)

	sess := session.NewManager(nil)
	svc := &AIService{
		sessionMgr: sess,
		cfg:        config.DefaultConfig(),
		prompts:    pm,
		llm:        &MockLLM{Response: "Summary: Test"},
	}
	svc.promptAssembler = prompt.NewAssembler(svc.cfg, nil, svc.prompts, nil, nil, nil, svc.llm, nil, nil)

	n := &model.Narrative{
		Title:  "Test Title",
		Script: "Test Script",
		Type:   model.NarrativeTypePOI,
		POI:    &model.POI{WikidataID: "Q123"},
	}

	// This calls AddNarration and starts a goroutine for summarizeAndLogEvent
	svc.RecordNarration(context.Background(), n)

	// Since we can't easily wait for the goroutine in a unit test without mocks,
	// we just verify the narration was added to the session history via events.
	// Actually, session.Manager.AddNarration just sets LastSentence.
	// AddEvent is called in a goroutine.

	// Let's check state
	state := sess.GetState()
	if state.LastSentence == "" {
		t.Error("expected LastSentence to be updated")
	}
}

func TestAIService_HandleAnnouncementJob(t *testing.T) {
	// Setup Prompts Dir (reuse same pattern as RecordNarration if needed, or just mock)
	tmpDir := t.TempDir()
	pm, _ := prompts.NewManager(tmpDir)

	svc := &AIService{
		prompts: pm,
	}
	// Mock announcement
	a := &mockAnnouncement{
		itemType: model.NarrativeTypeLetsgo,
		title:    "Let's Go",
	}

	job := &generation.Job{
		Type:         model.NarrativeTypeLetsgo,
		Announcement: a,
	}

	// handleAnnouncementJob expects a prompt Data return from GetPromptData
	// and a template to be available. We'll verify it handles nil data/missing templates gracefully
	// or returns nil since we didn't setup the whole environment.
	req := svc.handleAnnouncementJob(context.Background(), job)
	if req != nil {
		// It should fail because service.prompts is nil
		t.Error("expected nil request due to missing prompts manager")
	}
}

type mockAnnouncement struct {
	itemType model.NarrativeType
	title    string
	held     *model.Narrative
}

func (m *mockAnnouncement) Type() model.NarrativeType { return m.itemType }
func (m *mockAnnouncement) ID() string                { return "mock" }
func (m *mockAnnouncement) Title() string             { return m.title }
func (m *mockAnnouncement) Summary() string           { return "summary" }
func (m *mockAnnouncement) ImagePath() string         { return "" }
func (m *mockAnnouncement) POI() *model.POI           { return nil }
func (m *mockAnnouncement) GetPromptData(t *sim.Telemetry) (any, error) {
	return map[string]any{"MaxWords": 100}, nil
}
func (m *mockAnnouncement) ShouldGenerate(t *sim.Telemetry) bool { return true }
func (m *mockAnnouncement) ShouldPlay(t *sim.Telemetry) bool     { return true }
func (m *mockAnnouncement) Status() announcement.Status          { return announcement.StatusIdle }
func (m *mockAnnouncement) Reset()                               {}
func (m *mockAnnouncement) SetStatus(s announcement.Status)      {}
func (m *mockAnnouncement) GetHeldNarrative() *model.Narrative   { return m.held }
func (m *mockAnnouncement) SetHeldNarrative(n *model.Narrative)  { m.held = n }
func (m *mockAnnouncement) IsRepeatable() bool                   { return true }
