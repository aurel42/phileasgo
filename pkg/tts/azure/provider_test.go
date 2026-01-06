package azure

import (
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/tracker"
)

func TestProvider_Structure(t *testing.T) {
	// Since we can't easily mock the external Azure API call without dependency injection for http.Client
	// (which is currently hardcoded or internal), we primarily test that the provider is constructed correctly
	// and the tracker is assigned.

	tests := []struct {
		name    string
		cfg     config.AzureSpeechConfig
		tracker *tracker.Tracker
	}{
		{
			name: "With Tracker",
			cfg: config.AzureSpeechConfig{
				Key:     "fake-key",
				Region:  "eastus",
				VoiceID: "en-US-AvaNeural",
			},
			tracker: tracker.New(),
		},
		{
			name: "Without Tracker",
			cfg: config.AzureSpeechConfig{
				Key:    "fake-key",
				Region: "eastus",
			},
			tracker: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider(tt.cfg, tt.tracker)
			if p == nil {
				t.Fatal("NewProvider returned nil")
			}
			if p.tracker != tt.tracker {
				t.Error("Tracker not assigned correctly")
			}
			if p.region != tt.cfg.Region {
				t.Errorf("Region = %q, want %q", p.region, tt.cfg.Region)
			}
		})
	}
}

// Ensure the Provider implements the interface
// (This is a compile-time check, but good to have in test file as documentation)
// var _ tts.Provider = (*Provider)(nil)

func TestBuildSSML(t *testing.T) {
	p := NewProvider(config.AzureSpeechConfig{VoiceID: "test-voice"}, nil)

	tests := []struct {
		name     string
		input    string
		wantText string // Expected text inside the <voice> tag
	}{
		{
			name:     "Normal Text",
			input:    "Hello World",
			wantText: "Hello World",
		},
		{
			name:     "Valid SSML",
			input:    "Hello <break time=\"1s\"/> World",
			wantText: "Hello <break time=\"1s\"/> World",
		},
		{
			// The original issue: <lang xml:ID"> breaks validation.
			// Plus check comma injection for truncation workaround.
			name:  "Reparable SSML (Extra Attribute & Injection)",
			input: `Hello <lang xml:lang="vi-VN" xml:ID="foo">World</lang>`,
			// Expect comma inside, invalid attribute gone.
			wantText: `Hello <lang xml:lang="vi-VN">World,</lang>`,
		},
		{
			name:     "Existing Punctuation (No Injection)",
			input:    `<lang xml:lang="de">Sentence.</lang>`,
			wantText: `<lang xml:lang="de">Sentence.</lang>`,
		},
		{
			name:     "Malformed SSML (Broken Nesting -> Strip Tags)",
			input:    "Hello <lang>Bad World", // Missing closing tag
			wantText: "Hello Bad World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSSML := p.buildSSML("test-voice", tt.input)

			// We check if the result contains the expected content
			// Since we wrap it in <speak> and <voice>, we can just check substring
			// But for "Malformed SSML", checking that tags are GONE is key.

			// Note: buildSSML adds <break> after </lang>, so strict equality might be tricky
			// if we pass input with </lang>.
			// For "Malformed SSML" case, stripTags removes everything, so it's simpler.

			// Simple check: does it contain what we want?
			// Also, ensure it does NOT contain the bad tag.
			if tt.name == "Malformed SSML (Broken Nesting -> Strip Tags)" {
				ifContains(t, gotSSML, "<lang")              // Should NOT contain <lang
				ifNotContains(t, gotSSML, "Hello Bad World") // Should contain clean text
			} else {
				// Just check presence
				ifNotContains(t, gotSSML, tt.wantText)
			}
		})
	}
}

func ifContains(t *testing.T, s, substr string) {
	t.Helper()
	// Actually we want "if NOT contains" for the negative check
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			t.Errorf("string %q should NOT contain %q", s, substr)
			return
		}
	}
}

func ifNotContains(t *testing.T, s, substr string) {
	t.Helper()
	// We want "if DOES contain"
	found := false
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("string %q SHOULD contain %q", s, substr)
	}
}
