package announcement

import (
	"context"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"testing"
	"time"
)

type mockProvider struct {
	enqueued chan bool
	done     chan bool
}

func (m *mockProvider) EnqueueAnnouncement(ctx context.Context, a Announcement, t *sim.Telemetry, onComplete func(*model.Narrative)) {
	m.enqueued <- true
	// Simulate async result
	go func() {
		if onComplete != nil {
			onComplete(&model.Narrative{ID: "test-narrative"})
			if m.done != nil {
				m.done <- true
			}
		}
	}()
}

func TestBaseAnnouncement(t *testing.T) {
	b := NewBaseAnnouncement("test", model.NarrativeTypePOI, false)
	if b.ID() != "test" {
		t.Errorf("expected ID test, got %s", b.ID())
	}
	if b.Status() != StatusIdle {
		t.Errorf("expected StatusIdle, got %s", b.Status())
	}

	b.SetStatus(StatusGenerating)
	if b.Status() != StatusGenerating {
		t.Errorf("expected StatusGenerating, got %s", b.Status())
	}

	n := &model.Narrative{ID: "nar"}
	b.SetHeldNarrative(n)
	if b.GetHeldNarrative() != n {
		t.Errorf("expected narrative to be set")
	}

	b.Reset()
	if b.Status() != StatusIdle {
		t.Errorf("expected StatusIdle after reset")
	}
	if b.GetHeldNarrative() != nil {
		t.Errorf("expected held narrative to be nil after reset")
	}
}

type testAnnouncement struct {
	*BaseAnnouncement
	gen  bool
	play bool
}

func (a *testAnnouncement) ShouldGenerate(t *sim.Telemetry) bool        { return a.gen }
func (a *testAnnouncement) ShouldPlay(t *sim.Telemetry) bool            { return a.play }
func (a *testAnnouncement) GetPromptData(t *sim.Telemetry) (any, error) { return nil, nil }

func TestManager_Lifecycle(t *testing.T) {
	provider := &mockProvider{
		enqueued: make(chan bool, 10),
		done:     make(chan bool, 10),
	}
	mgr := NewManager(provider)

	a := &testAnnouncement{
		BaseAnnouncement: NewBaseAnnouncement("a1", model.NarrativeTypePOI, false),
	}
	mgr.Register(a)

	tel := &sim.Telemetry{}

	// 1. Trigger Generation
	a.gen = true
	mgr.Tick(context.Background(), tel)

	if a.Status() != StatusGenerating {
		t.Errorf("expected StatusGenerating, got %s", a.Status())
	}
	<-provider.enqueued // Wait for Enqueue call

	// Wait for async completion
	select {
	case <-provider.done:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timed out waiting for generation result")
	}

	// Status should be Held now (but we might need a Tick to see it if it was processed via callback)
	// Actually onResult updates the status directly.
	if a.Status() != StatusHeld {
		t.Errorf("expected StatusHeld, got %s", a.Status())
	}

	// 2. Trigger Playback
	a.play = true
	mgr.Tick(context.Background(), tel)

	if a.Status() != StatusTriggered {
		t.Errorf("expected StatusTriggered, got %s", a.Status())
	}

	// 3. Repeatability (false) -> Should go to Done on next tick
	mgr.Tick(context.Background(), tel)
	if a.Status() != StatusDone {
		t.Errorf("expected StatusDone, got %s", a.Status())
	}
}

func TestManager_ImmediatePlay(t *testing.T) {
	provider := &mockProvider{
		enqueued: make(chan bool, 10),
		done:     make(chan bool, 10),
	}
	mgr := NewManager(provider)

	a := &testAnnouncement{
		BaseAnnouncement: NewBaseAnnouncement("a2", model.NarrativeTypePOI, false),
		gen:              true,
		play:             true,
	}
	mgr.Register(a)

	mgr.Tick(context.Background(), &sim.Telemetry{})
	<-provider.enqueued
	<-provider.done

	// Status should be Triggered immediately if ShouldPlay was true (immediate play optimization)
	if a.Status() != StatusTriggered {
		t.Errorf("expected StatusTriggered after result for immediate play, got %s", a.Status())
	}
}
