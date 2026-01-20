package narrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"

	"github.com/stretchr/testify/assert"
)

func TestPriorityQueue_Basics(t *testing.T) {
	// Setup minimalist service (mocks not needed for pure queue logic if we are careful)
	// deeper mocks might be needed if we actually run ProcessPriorityQueue,
	// but here we test the Queue mechanism itself.
	s := &AIService{
		priorityGenQueue: make([]*GenerationJob, 0),
	}

	// 1. Initially Empty
	assert.False(t, s.HasPendingPriority(), "Should be empty initially")

	// 2. Enqueue Manual POI
	job1 := &GenerationJob{
		Type:      "poi",
		POIID:     "Q123",
		Manual:    true,
		CreatedAt: time.Now(),
	}
	s.enqueuePriority(job1)

	assert.True(t, s.HasPendingPriority(), "Should have pending priority")
	assert.Equal(t, 1, len(s.priorityGenQueue))

	// 3. Enqueue Screenshot
	job2 := &GenerationJob{
		Type:      "screenshot",
		ImagePath: "/tmp/foo.jpg",
		CreatedAt: time.Now(),
	}
	s.enqueuePriority(job2)
	assert.Equal(t, 2, len(s.priorityGenQueue))

	// 4. Pop (FIFO for now, or just check next)
	next := s.peekPriority()
	assert.NotNil(t, next)
	assert.Equal(t, "Q123", next.POIID)

	popped := s.popPriority()
	assert.Equal(t, job1, popped)
	assert.Equal(t, 1, len(s.priorityGenQueue))

	assert.True(t, s.HasPendingPriority())

	popped2 := s.popPriority()
	assert.Equal(t, job2, popped2)
	assert.False(t, s.HasPendingPriority())
}

func TestPriorityQueue_GenerationJob_Context(t *testing.T) {
	// Verify GenerationJob holds necessary context
	job := &GenerationJob{
		Type:     model.NarrativeTypePOI,
		POIID:    "Q123",
		Strategy: "funny",
		Manual:   true,
	}

	assert.Equal(t, "poi", string(job.Type))
	assert.Equal(t, "Q123", job.POIID)
	assert.True(t, job.Manual)
}

func TestAIService_ProcessPriorityQueue(t *testing.T) {
	// 1. Setup Service with Mocks
	mockLLM := &MockLLM{Response: "Script"}
	mockPOI := &MockPOIProvider{
		GetPOIFunc: func(_ context.Context, qid string) (*model.POI, error) {
			return &model.POI{WikidataID: qid, NameEn: "TestPOI"}, nil
		},
	}

	tempDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempDir, "narrator"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "narrator", "script.tmpl"), []byte("Msg"), 0o644)
	_ = os.WriteFile(filepath.Join(tempDir, "narrator", "summary_update.tmpl"), []byte("Summary"), 0o644)
	pm, _ := prompts.NewManager(tempDir) // Dummy manager with template

	svc := NewAIService(&config.Config{}, mockLLM, &MockTTS{Format: "mp3"}, pm, &MockAudio{}, mockPOI, &MockBeacon{}, &MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil, nil)

	// 2. Enqueue a Manual POI Job
	job := &GenerationJob{
		Type:     model.NarrativeTypePOI,
		POIID:    "QProcess",
		Manual:   true,
		Strategy: "uniform",
	}
	svc.enqueuePriority(job)

	// 3. Process the Queue (Synchronously for test)
	// We mocked ProcessPriorityQueue to run in the same goroutine or we wait?
	// The method doesn't exist yet, but we expect it to be blocking if we call it directly?
	// "ProcessPriorityQueue(ctx)" should ideally drain the queue.

	ctx := context.Background()
	svc.ProcessPriorityQueue(ctx)

	// 4. Verification
	// The queue should be empty
	assert.False(t, svc.HasPendingPriority())

	// The Playback Queue should have the generated narrative
	svc.mu.Lock()
	defer svc.mu.Unlock()
	assert.Equal(t, 1, len(svc.queue))
	if len(svc.queue) > 0 {
		assert.Equal(t, "QProcess", svc.queue[0].POI.WikidataID)
		assert.True(t, svc.queue[0].Manual)
	}
}
