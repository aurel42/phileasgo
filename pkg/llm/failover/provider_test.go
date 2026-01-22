package failover

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"phileasgo/pkg/llm"
	"strings"
	"testing"
)

type mockProvider struct {
	responses []string
	errors    []error
	healthErr error
	callCount int
}

func (m *mockProvider) GenerateText(ctx context.Context, name, prompt string) (string, error) {
	idx := m.callCount
	m.callCount++
	if idx >= len(m.errors) {
		return "", fmt.Errorf("out of bounds")
	}
	return m.responses[idx], m.errors[idx]
}

func (m *mockProvider) GenerateJSON(ctx context.Context, name, prompt string, target any) error {
	_, err := m.GenerateText(ctx, name, prompt)
	return err
}

func (m *mockProvider) GenerateImageText(ctx context.Context, name, prompt, imagePath string) (string, error) {
	return m.GenerateText(ctx, name, prompt)
}

func (m *mockProvider) HealthCheck(ctx context.Context) error {
	return m.healthErr
}

func TestFailover_SuccessFirst(t *testing.T) {
	p1 := &mockProvider{responses: []string{"resp1"}, errors: []error{nil}}
	p2 := &mockProvider{responses: []string{"resp2"}, errors: []error{nil}}

	f, _ := New([]llm.Provider{p1, p2}, []string{"p1", "p2"}, "", nil)
	res, err := f.GenerateText(context.Background(), "test", "prompt")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "resp1" {
		t.Errorf("expected resp1, got %s", res)
	}
	if p2.callCount > 0 {
		t.Errorf("p2 should not have been called")
	}
}

func TestFailover_FailoverOnRetryable(t *testing.T) {
	p1 := &mockProvider{responses: []string{""}, errors: []error{fmt.Errorf("429 too many requests")}}
	p2 := &mockProvider{responses: []string{"resp2"}, errors: []error{nil}}

	f, _ := New([]llm.Provider{p1, p2}, []string{"p1", "p2"}, "", nil)
	res, err := f.GenerateText(context.Background(), "test", "prompt")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "resp2" {
		t.Errorf("expected resp2, got %s", res)
	}
	if p1.callCount != 1 {
		t.Errorf("p1 should have been called once")
	}
	if p2.callCount != 1 {
		t.Errorf("p2 should have been called once")
	}
}

func TestFailover_CircuitBreakerOnFatal(t *testing.T) {
	p1 := &mockProvider{responses: []string{""}, errors: []error{fmt.Errorf("401 unauthorized")}}
	p2 := &mockProvider{responses: []string{"resp2"}, errors: []error{nil}}

	f, _ := New([]llm.Provider{p1, p2}, []string{"p1", "p2"}, "", nil)

	// First call triggers circuit break
	_, err := f.GenerateText(context.Background(), "test", "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f.mu.RLock()
	disabled := f.disabled[0]
	f.mu.RUnlock()
	if !disabled {
		t.Errorf("p1 should be disabled")
	}

	// Second call should skip p1
	p1.callCount = 0
	p2.callCount = 0
	p2.responses = []string{"resp2_retry"}
	p2.errors = []error{nil}

	res, err := f.GenerateText(context.Background(), "test", "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "resp2_retry" {
		t.Errorf("expected resp2_retry, got %s", res)
	}
	if p1.callCount != 0 {
		t.Errorf("p1 should have been skipped")
	}
}

func TestFailover_NoDisableLastProvider(t *testing.T) {
	p1 := &mockProvider{responses: []string{""}, errors: []error{fmt.Errorf("401 unauthorized")}}

	f, _ := New([]llm.Provider{p1}, []string{"p1"}, "", nil)
	_, err := f.GenerateText(context.Background(), "test", "prompt")

	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("unexpected error: %v", err)
	}

	f.mu.RLock()
	disabled := f.disabled[0]
	f.mu.RUnlock()
	if disabled {
		t.Errorf("last provider should NOT be disabled")
	}
}

func TestFailover_RetryLast(t *testing.T) {
	// P1: Fail (429), Retry (429), Success
	p1 := &mockProvider{
		responses: []string{"", "", "resp_success"},
		errors:    []error{fmt.Errorf("429"), fmt.Errorf("429"), nil},
	}

	f, _ := New([]llm.Provider{p1}, []string{"p1"}, "", nil)
	res, err := f.GenerateText(context.Background(), "test", "prompt")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "resp_success" {
		t.Errorf("expected success on 3rd attempt, got %s", res)
	}
	if p1.callCount != 3 {
		t.Errorf("expected 3 calls, got %d", p1.callCount)
	}
}

func TestFailover_ExhaustAll(t *testing.T) {
	p1 := &mockProvider{responses: []string{""}, errors: []error{fmt.Errorf("429")}}
	p2 := &mockProvider{responses: []string{"", "", "", ""}, errors: []error{fmt.Errorf("429"), fmt.Errorf("429"), fmt.Errorf("429"), fmt.Errorf("429")}}

	f, _ := New([]llm.Provider{p1, p2}, []string{"p1", "p2"}, "", nil)
	_, err := f.GenerateText(context.Background(), "test", "prompt")

	if !strings.Contains(err.Error(), "exhausted after 3 retries") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFailover_JSON_Success(t *testing.T) {
	p1 := &mockProvider{responses: []string{"{}"}, errors: []error{nil}}
	f, _ := New([]llm.Provider{p1}, []string{"p1"}, "", nil)
	err := f.GenerateJSON(context.Background(), "test", "prompt", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p1.callCount != 1 {
		t.Errorf("expected 1 call, got %d", p1.callCount)
	}
}

func TestFailover_ImageText_Success(t *testing.T) {
	p1 := &mockProvider{responses: []string{"image desc"}, errors: []error{nil}}
	f, _ := New([]llm.Provider{p1}, []string{"p1"}, "", nil)
	res, err := f.GenerateImageText(context.Background(), "test", "prompt", "path/to/img")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "image desc" {
		t.Errorf("expected 'image desc', got %s", res)
	}
}

func TestFailover_HealthCheck(t *testing.T) {
	p1 := &mockProvider{healthErr: fmt.Errorf("failed")}
	p2 := &mockProvider{healthErr: nil}

	f, _ := New([]llm.Provider{p1, p2}, []string{"p1", "p2"}, "", nil)

	// Scenario: p1 fails healthcheck, p2 succeeds
	err := f.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck should succeed if p2 is healthy: %v", err)
	}

	// Scenario: Both fail
	p2.healthErr = fmt.Errorf("also failed")
	err = f.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("HealthCheck should fail if all providers are unhealthy")
	}

	// Scenario: One is disabled (Circuit Breaker)
	p1.healthErr = nil                  // p1 is now healthy
	p2.healthErr = fmt.Errorf("failed") // p2 is unhealthy
	f.disabled[0] = true                // p1 is disabled

	err = f.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("HealthCheck should fail if only healthy provider is disabled")
	}
}

func TestFailover_New_Errors(t *testing.T) {
	_, err := New(nil, nil, "", nil)
	if err == nil {
		t.Error("expected error for nil providers")
	}

	_, err = New([]llm.Provider{&mockProvider{}}, []string{"p1", "p2"}, "", nil)
	if err == nil {
		t.Error("expected error for mismatched counts")
	}
}

func TestIsUnrecoverable(t *testing.T) {
	tests := []struct {
		err      error
		expected bool
	}{
		{nil, false},
		{fmt.Errorf("401 unauthorized"), true},
		{fmt.Errorf("403 forbidden"), true},
		{fmt.Errorf("400 bad request"), true},
		{fmt.Errorf("random error"), false},
		{fmt.Errorf("invalid_api_key"), true},
	}

	for _, tt := range tests {
		if got := isUnrecoverable(tt.err); got != tt.expected {
			t.Errorf("isUnrecoverable(%v) = %v, want %v", tt.err, got, tt.expected)
		}
	}
}

func TestFailover_Logging(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "llm_log_test")
	defer os.RemoveAll(tmpDir)
	logPath := filepath.Join(tmpDir, "llm.log")

	p1 := &mockProvider{responses: []string{"success_resp"}, errors: []error{nil}}
	f, _ := New([]llm.Provider{p1}, []string{"p1"}, logPath, nil)

	// Test Successful Log
	_, _ = f.GenerateText(context.Background(), "SuccessCall", "Prompt text")

	content, _ := os.ReadFile(logPath)
	if !strings.Contains(string(content), "PROMPT: SuccessCall") {
		t.Errorf("log should contain prompt name, got %s", string(content))
	}
	if !strings.Contains(string(content), "Prompt text") {
		t.Errorf("log should contain prompt text")
	}
	if !strings.Contains(string(content), "success_resp") {
		t.Errorf("log should contain response text")
	}

	// Test Error Log
	p2 := &mockProvider{responses: []string{""}, errors: []error{fmt.Errorf("fatal 401")}}
	f2, _ := New([]llm.Provider{p2}, []string{"p2"}, logPath, nil)
	_, _ = f2.GenerateText(context.Background(), "FailCall", "Fail Prompt")

	content, _ = os.ReadFile(logPath)
	if !strings.Contains(string(content), "ERROR: FailCall - fatal 401") {
		t.Errorf("log should contain error entry, got %s", string(content))
	}
	// Failures should NOT contain the prompt text according to requirements
	if strings.Contains(string(content), "Fail Prompt") {
		// Wait, let's re-read requirements: "for unsuccessful requests, we log in llm.log only the fact that they happened and the reason why they failed."
		// My implementation DOES NOT log prompt for errors, so it shouldn't be there.
		parts := strings.Split(string(content), "FailCall")
		if len(parts) > 1 && strings.Contains(parts[len(parts)-1], "Fail Prompt") {
			t.Errorf("error log should NOT contain prompt text")
		}
	}
}
