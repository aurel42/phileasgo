package narrator

import (
	"context"
	"os"
	"path/filepath"
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/session"
	"phileasgo/pkg/tts"
	"strings"
	"testing"
	"time"
)

func TestAIService_ExtractTitleFromScript(t *testing.T) {
	s := &AIService{}

	tests := []struct {
		name       string
		script     string
		wantTitle  string
		wantScript string
	}{
		{
			name:       "Standard Title",
			script:     "TITLE: The Great Land\nThis is the content.",
			wantTitle:  "The Great Land",
			wantScript: "This is the content.",
		},
		{
			name:       "Markdown Bold Title",
			script:     "**TITLE:** Mount McKinley\n**The mountains are high.**",
			wantTitle:  "Mount McKinley",
			wantScript: "**The mountains are high.**",
		},
		{
			name:       "Case Insensitive Title",
			script:     "Title : Flying over Alaska\nLow clouds today.",
			wantTitle:  "Flying over Alaska",
			wantScript: "Low clouds today.",
		},
		{
			name:       "No Title",
			script:     "Just some narration\nwithout a title.",
			wantTitle:  "",
			wantScript: "Just some narration\nwithout a title.",
		},
		{
			name:       "Title Only",
			script:     "TITLE: Only Title",
			wantTitle:  "Only Title",
			wantScript: "",
		},
		{
			name:       "Indented Title",
			script:     "  **TITLE: ** Indented\nNext line",
			wantTitle:  "Indented",
			wantScript: "Next line",
		},
		{
			name:       "Title with asterisk suffix",
			script:     "**TITLE: Bold Title**\nStory starts here",
			wantTitle:  "Bold Title",
			wantScript: "Story starts here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTitle, gotScript := s.extractTitleFromScript(tt.script)
			if gotTitle != tt.wantTitle {
				t.Errorf("extractTitleFromScript() gotTitle = %v, want %v", gotTitle, tt.wantTitle)
			}
			if strings.TrimSpace(gotScript) != strings.TrimSpace(tt.wantScript) {
				t.Errorf("extractTitleFromScript() gotScript = %v, want %v", gotScript, tt.wantScript)
			}
		})
	}
}

func TestAIService_PerformRescueIfNeeded(t *testing.T) {
	// Setup Prompts
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "context")
	_ = os.MkdirAll(promptsDir, 0o755)
	_ = os.WriteFile(filepath.Join(promptsDir, "rescue_script.tmpl"), []byte("Rescue: {{.Script}}"), 0o644)
	pm, _ := prompts.NewManager(tmpDir)

	mockLLM := &MockLLM{}

	s := &AIService{
		prompts:    pm,
		llm:        mockLLM,
		cfg:        config.NewProvider(config.DefaultConfig(), nil),
		sessionMgr: session.NewManager(nil),
	}
	s.promptAssembler = prompt.NewAssembler(s.cfg, nil, s.prompts, nil, nil, nil, s.llm, nil, nil, nil, nil, nil)

	tests := []struct {
		name       string
		maxWords   int
		script     string
		llmResp    string
		wantResult string
	}{
		{
			name:       "No Rescue Needed",
			maxWords:   100,
			script:     "Short script.",
			wantResult: "Short script.",
		},
		{
			name:       "Rescue Not Possible (No limit)",
			maxWords:   0,
			script:     "Long script that won't be rescued because maxWords is 0.",
			wantResult: "Long script that won't be rescued because maxWords is 0.",
		},
		{
			name:       "Rescue Successful",
			maxWords:   10,
			script:     "This is a very long script that definitely needs to be rescued because it is far too long for the target of ten words.",
			llmResp:    "Clean rescued script.",
			wantResult: "Clean rescued script.",
		},
		{
			name:       "Rescue Produces Garbage - Fallback to Original",
			maxWords:   10,
			script:     "This script is too long.",
			llmResp:    strings.Repeat("garbage ", 100),
			wantResult: "This script is too long.",
		},
		{
			name:       "Rescue Returns RESCUE_FAILED - Fallback to Original",
			maxWords:   10,
			script:     "Over the limit script.",
			llmResp:    "RESCUE_FAILED",
			wantResult: "Over the limit script.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLLM.Response = tt.llmResp
			req := &GenerationRequest{MaxWords: tt.maxWords}
			got := s.performRescueIfNeeded(context.Background(), req, tt.script)
			if got != tt.wantResult {
				t.Errorf("performRescueIfNeeded() = %v, want %v", got, tt.wantResult)
			}
		})
	}
}

func TestAIService_IsGarbage(t *testing.T) {
	s := &AIService{
		sessionMgr: session.NewManager(nil),
	}

	tests := []struct {
		name   string
		max    int
		input  string
		output string
		want   bool
	}{
		{
			name:   "Empty Output is Garbage",
			max:    100,
			input:  "Some input",
			output: "",
			want:   true,
		},
		{
			name:   "RESCUE_FAILED is Garbage",
			max:    100,
			input:  "Some input",
			output: "RESCUE_FAILED",
			want:   true,
		},
		{
			name:   "Clean Output is NOT Garbage",
			max:    50,
			input:  "Input with fifty words...",
			output: "Clean output with five words.",
			want:   false,
		},
		{
			name:   "Output Exceeding Threshold is Garbage",
			max:    10,
			input:  "Ten words input.",
			output: strings.Repeat("word ", 25), // 25 > 10 * 2.0
			want:   true,
		},
		{
			name:   "Output Exceeding Input Threshold is Garbage",
			max:    100, // Large target, but input is small
			input:  "Five words input here now.",
			output: strings.Repeat("word ", 30), // 30 > 5 * 1.5 = 7.5
			want:   true,
		},
		{
			name:   "Output Within Reasonable Limits is NOT Garbage",
			max:    10,
			input:  strings.Repeat("word ", 10),
			output: strings.Repeat("word ", 15), // 15 < max(10*2, 10*1.5) = 20
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &GenerationRequest{MaxWords: tt.max}
			if got := s.isGarbage(req, tt.input, tt.output); got != tt.want {
				t.Errorf("isGarbage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAIService_ConstructNarrative(t *testing.T) {
	s := &AIService{}
	req := &GenerationRequest{
		Type:         model.NarrativeTypePOI,
		Title:        "Test POI",
		MaxWords:     50,
		ThumbnailURL: "http://thumb",
	}

	n := s.constructNarrative(req, "The script", "Extracted Title", "audio.mp3", "mp3", time.Now(), time.Second, 10*time.Second)

	if n.Title != "Test POI" {
		t.Errorf("Expected Title 'Test POI', got '%s'", n.Title)
	}
	if n.Duration != 10*time.Second {
		t.Errorf("Expected Duration 10s, got %v", n.Duration)
	}
	if n.ThumbnailURL != "http://thumb" {
		t.Errorf("Expected ThumbnailURL 'http://thumb', got '%s'", n.ThumbnailURL)
	}

	// Test screenshot special case
	req.Type = model.NarrativeTypeScreenshot
	req.Title = ""
	req.ImagePath = "C:\\path\\to\\shot.png"
	n = s.constructNarrative(req, "The script", "Extracted Title", "audio.mp3", "mp3", time.Now(), time.Second, 5*time.Second)
	if n.Title != "Extracted Title" {
		t.Errorf("Expected Extracted Title, got '%s'", n.Title)
	}
	if n.ImagePath != "C:\\path\\to\\shot.png" {
		t.Errorf("Expected ImagePath to be preserved as raw path, got '%s'", n.ImagePath)
	}
}

func TestAIService_PerformSecondPass(t *testing.T) {
	tmpDir := t.TempDir()
	// Create the template BEFORE initializing the manager
	_ = writeFile(filepath.Join(tmpDir, "context", "second_pass.tmpl"), "Refined: {{.Script}} ({{.MaxWords}})")
	pm, _ := prompts.NewManager(tmpDir)

	mockLLM := &MockLLM{Response: "Refined script"}
	cfg := config.NewProvider(config.DefaultConfig(), nil)
	sess := session.NewManager(nil)

	s := &AIService{
		prompts:    pm,
		llm:        mockLLM,
		cfg:        cfg,
		sessionMgr: sess,
	}
	s.promptAssembler = prompt.NewAssembler(s.cfg, nil, s.prompts, nil, nil, nil, s.llm, nil, nil, nil, nil, nil)

	req := &GenerationRequest{
		MaxWords: 100,
	}

	// 1. Success case
	input := "This is a long original script with many words." // 9 words
	got := s.performSecondPass(context.Background(), req, input)
	if !strings.Contains(got, "Refined script") {
		t.Errorf("expected refined script, got %q", got)
	}

	// 2. Multiplier check
	mockLLM.GenerateTextFunc = func(ctx context.Context, name, promptBody string) (string, error) {
		if !strings.Contains(promptBody, "(120)") { // 100 * 1.2
			t.Errorf("expected MaxWords 120 in prompt, got %s", promptBody)
		}
		return "Refined with multiplier", nil
	}
	mockos := s.performSecondPass(context.Background(), req, input)
	if !strings.Contains(mockos, "Refined with multiplier") {
		t.Errorf("expected refined with multiplier, got %q", mockos)
	}

	// 3. Rescue Failed case
	mockLLM.GenerateTextFunc = nil
	mockLLM.Response = "RESCUE_FAILED"
	got = s.performSecondPass(context.Background(), req, input)
	if got != input {
		t.Errorf("expected original script on rescue failed, got %q", got)
	}
}

func TestAIService_SynthesizeRetry(t *testing.T) {
	mockTTS := &MockTTS{Format: "mp3"}
	mockLLM := &MockLLM{Response: "TITLE: OK\nScript"}
	s := &AIService{
		tts: mockTTS,
		llm: mockLLM,
		sim: &MockSim{},
		cfg: config.NewProvider(config.DefaultConfig(), nil),
	}
	s.promptAssembler = prompt.NewAssembler(s.cfg, nil, nil, nil, nil, nil, mockLLM, nil, nil, nil, nil, nil)

	req := &GenerationRequest{
		Type: model.NarrativeTypePOI,
	}

	// 1. Succeed on 3rd attempt
	attempts := 0
	mockTTS.SynthesizeFunc = func(ctx context.Context, text, voiceID, outputPath string) (string, error) {
		attempts++
		if attempts < 3 {
			return "", tts.NewFatalError(500, "transient error")
		}
		// Success: write file
		fullPath := outputPath + ".mp3"
		_ = os.WriteFile(fullPath, make([]byte, tts.MinAudioSize+1), 0644)
		return "mp3", nil
	}

	narrative, err := s.GenerateNarrative(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success on 3rd attempt, got error: %v", err)
	}
	if narrative == nil {
		t.Fatal("expected narrative, got nil")
	}
	if attempts != 3 {
		t.Errorf("expected 3 synthesis attempts, got %d", attempts)
	}

	// 2. Fail after 3 attempts
	attempts = 0
	mockTTS.SynthesizeFunc = func(ctx context.Context, text, voiceID, outputPath string) (string, error) {
		attempts++
		return "", tts.NewFatalError(500, "permanent error")
	}

	_, err = s.GenerateNarrative(context.Background(), req)
	if err == nil {
		t.Fatal("expected error after 3 failed attempts, got nil")
	}
	if attempts != 3 {
		t.Errorf("expected exactly 3 attempts before giving up, got %d", attempts)
	}
}

// Helper from manager_test or similar if not available
func writeFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
