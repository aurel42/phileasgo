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
	responses         []string
	errors            []error
	healthErr         error
	callCount         int
	supportedProfiles map[string]bool
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

func (m *mockProvider) HasProfile(name string) bool {
	if m.supportedProfiles != nil {
		return m.supportedProfiles[name]
	}
	return true
}

func TestFailover_SuccessFirst(t *testing.T) {
	p1 := &mockProvider{responses: []string{"resp1"}, errors: []error{nil}}
	p2 := &mockProvider{responses: []string{"resp2"}, errors: []error{nil}}

	f, _ := New([]llm.Provider{p1, p2}, []string{"p1", "p2"}, "", true, nil)
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

	f, _ := New([]llm.Provider{p1, p2}, []string{"p1", "p2"}, "", true, nil)
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

	f, _ := New([]llm.Provider{p1, p2}, []string{"p1", "p2"}, "", true, nil)

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

	f, _ := New([]llm.Provider{p1}, []string{"p1"}, "", true, nil)
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

	f, _ := New([]llm.Provider{p1}, []string{"p1"}, "", true, nil)
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

	f, _ := New([]llm.Provider{p1, p2}, []string{"p1", "p2"}, "", true, nil)
	_, err := f.GenerateText(context.Background(), "test", "prompt")

	if !strings.Contains(err.Error(), "exhausted after 3 retries") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFailover_JSON_Success(t *testing.T) {
	p1 := &mockProvider{responses: []string{"{}"}, errors: []error{nil}}
	f, _ := New([]llm.Provider{p1}, []string{"p1"}, "", true, nil)
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
	f, _ := New([]llm.Provider{p1}, []string{"p1"}, "", true, nil)
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

	f, _ := New([]llm.Provider{p1, p2}, []string{"p1", "p2"}, "", true, nil)

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
	_, err := New(nil, nil, "", true, nil)
	if err == nil {
		t.Error("expected error for nil providers")
	}

	_, err = New([]llm.Provider{&mockProvider{}}, []string{"p1", "p2"}, "", true, nil)
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
		{fmt.Errorf("400 bad request"), false},
		{fmt.Errorf("429 too many requests"), false},
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
	f, _ := New([]llm.Provider{p1}, []string{"p1"}, logPath, true, nil)

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
	f2, _ := New([]llm.Provider{p2}, []string{"p2"}, logPath, true, nil)
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

func TestFailover_ProfileSparse(t *testing.T) {
	// P1: Default provider (but MISSING "vision")
	p1 := &mockProvider{
		responses:         []string{"default_resp"},
		errors:            []error{nil},
		supportedProfiles: map[string]bool{"narration": true},
	}
	// P2: Vision provider (Has "vision")
	p2 := &mockProvider{
		responses:         []string{"vision_resp"},
		errors:            []error{nil},
		supportedProfiles: map[string]bool{"vision": true},
	}

	// Global chain: P1 -> P2
	f, _ := New([]llm.Provider{p1, p2}, []string{"p1", "p2"}, "", true, nil)

	// Call narration (supported by P1)
	res, err := f.GenerateText(context.Background(), "narration", "text prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "default_resp" {
		t.Errorf("expected default_resp, got %s", res)
	}
	if p1.callCount != 1 {
		t.Errorf("p1 should be called for narration")
	}

	// Call vision (NOT supported by P1, should fallback to P2)
	// IMPORTANT: P1 should NOT be called at all (HashProfile check)
	p1CallsInit := p1.callCount
	res, err = f.GenerateText(context.Background(), "vision", "vision prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "vision_resp" {
		t.Errorf("expected vision_resp, got %s", res)
	}
	if p1.callCount != p1CallsInit {
		t.Errorf("p1 should NOT be called for vision (unsupported profile)")
	}
	if p2.callCount != 1 {
		t.Errorf("p2 should be called for vision")
	}
}

func TestFailover_SmartBackoff(t *testing.T) {
	// P1 will fail twice with 429, then eventually succeed
	// Sequence of calls we expect to P1:
	// 1. Fail (sf=1, sr=0) -> Skip next
	// 2. [Skipped]
	// 3. Fail (sf=2, sr=0) -> Skip next 2
	// 4. [Skipped]
	// 5. [Skipped]
	// 6. Success -> Reset
	p1 := &mockProvider{
		responses: []string{"", "", "p1_success"},
		errors:    []error{fmt.Errorf("429"), fmt.Errorf("429"), nil},
	}
	// P2 will always succeed
	p2 := &mockProvider{
		responses: []string{"p2_1", "p2_2", "p2_3", "p2_4", "p2_5", "p2_6"},
		errors:    []error{nil, nil, nil, nil, nil, nil},
	}

	f, _ := New([]llm.Provider{p1, p2}, []string{"p1", "p2"}, "", true, nil)

	// Call 1: P1 tried, fails. P2 called. (P1 calls: 1, P2 calls: 1)
	res, _ := f.GenerateText(context.Background(), "narration", "p")
	if res != "p2_1" {
		t.Errorf("Call 1: expected p2_1, got %s", res)
	}

	// Call 2: P1 skipped (sr:0 < sf:1). P2 called. (P1 calls: 1, P2 calls: 2)
	res, _ = f.GenerateText(context.Background(), "narration", "p")
	if res != "p2_2" {
		t.Errorf("Call 2: expected p2_2, got %s", res)
	}
	if p1.callCount != 1 {
		t.Errorf("Call 2: expected p1 count 1 (skipped), got %d", p1.callCount)
	}

	// Call 3: P1 tried (sr:1 < sf:1 is false), fails. P2 called. (P1 calls: 2, P2 calls: 3)
	// sf becomes 2, sr reset to 0.
	res, _ = f.GenerateText(context.Background(), "narration", "p")
	if res != "p2_3" {
		t.Errorf("Call 3: expected p2_3, got %s", res)
	}
	if p1.callCount != 2 {
		t.Errorf("Call 3: expected p1 count 2, got %d", p1.callCount)
	}

	// Call 4: P1 skipped (sr:0 < sf:2). P2 called. (P1 calls: 2, P2 calls: 4)
	res, _ = f.GenerateText(context.Background(), "narration", "p")
	if res != "p2_4" {
		t.Errorf("Call 4: expected p2_4, got %s", res)
	}

	// Call 5: P1 skipped (sr:1 < sf:2). P2 called. (P1 calls: 2, P2 calls: 5)
	res, _ = f.GenerateText(context.Background(), "narration", "p")
	if res != "p2_5" {
		t.Errorf("Call 5: expected p2_5, got %s", res)
	}

	// Call 6: P1 tried (sr:2 < sf:2 false), succeeds. (P1 calls: 3, P2 calls: 5)
	res, _ = f.GenerateText(context.Background(), "narration", "p")
	if res != "p1_success" {
		t.Errorf("Call 6: expected p1_success, got %s", res)
	}
	if p2.callCount != 5 {
		t.Errorf("Call 6: expected p2 count 5, got %d", p2.callCount)
	}

	// Call 7: P1 tried immediately (reset).
	res, _ = f.GenerateText(context.Background(), "narration", "p")
	// Since P1 is out of mocked responses, it might fail or we should add more.
	// But we just want to see it was called.
	if p1.callCount != 4 {
		t.Errorf("Call 7: expected p1 count 4, got %d", p1.callCount)
	}
}
