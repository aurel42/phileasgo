package wikidata

import (
	"strings"
	"testing"

	"log/slog"
	"os"
	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
)

func TestValidator_ValidateBatch(t *testing.T) {
	// We need a real requester to test the API or a mock.
	// Since we are in planning/exec, I'll use a stubbed client if possible or just check logic.

	// For now, let's just make sure it compiles and doesn't panic with nil client (well it will panic)
	// I'll skip the actual network test here unless I have a good mock.
	t.Skip("Skipping live validator test")
}

func TestValidator_Logic(t *testing.T) {
	// Stub test to verify compilation
	tr := tracker.New()
	req := request.New(nil, tr)
	client := NewClient(req, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	v := NewValidator(client)

	if v == nil {
		t.Fatal("Validator should not be nil")
	}
}

// TestTryDirectMatch tests the direct match logic for QID validation.
func TestTryDirectMatch(t *testing.T) {
	tr := tracker.New()
	req := request.New(nil, tr)
	client := NewClient(req, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	v := NewValidator(client)

	tests := []struct {
		name         string
		inputName    string
		inputQID     string
		actualLabels map[string]string
		wantMatch    bool
	}{
		{
			name:         "exact match lowercase",
			inputName:    "castle",
			inputQID:     "Q23413",
			actualLabels: map[string]string{"Q23413": "castle"},
			wantMatch:    true,
		},
		{
			name:         "contains match",
			inputName:    "castle",
			inputQID:     "Q23413",
			actualLabels: map[string]string{"Q23413": "medieval castle"},
			wantMatch:    true,
		},
		{
			name:         "reverse contains match",
			inputName:    "medieval castle",
			inputQID:     "Q23413",
			actualLabels: map[string]string{"Q23413": "castle"},
			wantMatch:    true,
		},
		{
			name:         "no match",
			inputName:    "castle",
			inputQID:     "Q23413",
			actualLabels: map[string]string{"Q23413": "palace"},
			wantMatch:    false,
		},
		{
			name:         "empty QID",
			inputName:    "castle",
			inputQID:     "",
			actualLabels: map[string]string{},
			wantMatch:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, ok := v.tryDirectMatch(tc.inputName, tc.inputQID, tc.actualLabels)
			if ok != tc.wantMatch {
				t.Errorf("tryDirectMatch(%q, %q) = %v, want %v", tc.inputName, tc.inputQID, ok, tc.wantMatch)
			}
			if ok && result.QID != tc.inputQID {
				t.Errorf("Expected QID %s, got %s", tc.inputQID, result.QID)
			}
		})
	}
}

// TestExtractQIDs tests the QID extraction from suggestions.
func TestExtractQIDs(t *testing.T) {
	tr := tracker.New()
	req := request.New(nil, tr)
	client := NewClient(req, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	v := NewValidator(client)

	suggestions := map[string]string{
		"castle":   "Q23413",
		"palace":   "Q16560",
		"invalid":  "not-a-qid",
		"empty":    "",
		"numbered": "Q12345",
	}

	qids := v.extractQIDs(suggestions)

	// Should only extract valid QIDs starting with Q
	if len(qids) != 3 {
		t.Errorf("Expected 3 valid QIDs, got %d: %v", len(qids), qids)
	}

	// Verify all extracted are valid
	for _, qid := range qids {
		if !strings.HasPrefix(qid, "Q") {
			t.Errorf("Invalid QID extracted: %s", qid)
		}
	}
}
