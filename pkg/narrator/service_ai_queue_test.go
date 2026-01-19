package narrator

import (
	"testing"
)

func TestAIService_QueueConstraints(t *testing.T) {
	s := &AIService{
		queue: make([]*Narrative, 0),
	}

	// 1. Auto POI allowed when queue empty
	if !s.canEnqueue("poi", false) {
		t.Error("Auto POI should be allowed when queue is empty")
	}

	// 2. Add item manually
	s.enqueue(&Narrative{Type: "poi", Manual: true, Title: "Manual 1"}, true)

	// 3. Auto POI rejected when queue not empty
	if s.canEnqueue("poi", false) {
		t.Error("Auto POI should be rejected when queue is not empty")
	}

	// 4. Auto Essay rejected when queue not empty
	if s.canEnqueue("essay", false) {
		t.Error("Auto Essay should be rejected when queue is not empty")
	}

	// 5. Manual POI rejected if one already strictly queued (Limit 1 Manual POI)
	// Current queue: [Manual 1]
	// Count manual POIs = 1. Limit is 1. Should return false.
	if s.canEnqueue("poi", true) {
		t.Error("Second Manual POI should be rejected if one exists in queue")
	}

	// 6. Screenshot: Add one
	if !s.canEnqueue("screenshot", true) {
		t.Error("Screenshot should be allowed (count 0)")
	}
	s.enqueue(&Narrative{Type: "screenshot", Manual: true, Title: "Screen 1"}, true)
	// Queue: [Screen 1, Manual 1] (Priority prepend)

	// 7. Screenshot rejected if one exists
	if s.canEnqueue("screenshot", true) {
		t.Error("Second Screenshot should be rejected")
	}

	// 8. Debrief: Add one
	if !s.canEnqueue("debrief", true) {
		t.Error("Debrief should be allowed")
	}
	s.enqueue(&Narrative{Type: "debrief", Manual: true, Title: "Debrief 1"}, true)
	// Queue: [Debrief 1, Screen 1, Manual 1]

	// 9. Debrief rejected if one exists
	if s.canEnqueue("debrief", true) {
		t.Error("Second Debrief should be rejected")
	}
}

func TestAIService_QueuePriority(t *testing.T) {
	s := &AIService{
		queue: make([]*Narrative, 0),
	}

	// Auto Item (Low Priority)
	n1 := &Narrative{Type: "poi", Manual: false, Title: "Auto 1"}
	s.enqueue(n1, false)
	// Queue: [Auto 1]

	// Auto Item 2
	n2 := &Narrative{Type: "poi", Manual: false, Title: "Auto 2"}
	s.enqueue(n2, false)
	// Queue: [Auto 1, Auto 2]

	// Manual Item (High Priority)
	n3 := &Narrative{Type: "image", Manual: true, Title: "Manual 3"}
	s.enqueue(n3, true)
	// Queue: [Manual 3, Auto 1, Auto 2]

	// Check order
	if s.queue[0].Title != "Manual 3" {
		t.Errorf("Expected Manual 3 at head, got %s", s.queue[0].Title)
	}
	if s.queue[1].Title != "Auto 1" {
		t.Errorf("Expected Auto 1 at pos 1, got %s", s.queue[1].Title)
	}
}
