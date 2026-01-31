package request

import "testing"

func TestNormalizeProvider(t *testing.T) {
	tests := []struct {
		host     string
		expected string
	}{
		{"www.wikidata.org", "wikidata"},
		{"query.wikidata.org", "wikidata"},
		{"en.wikipedia.org", "wikipedia"},
		{"fr.wikipedia.org", "wikipedia"},
		{"generativelanguage.googleapis.com", "gemini"},
		{"api.groq.com", "groq"}, // checking if existing logic covers this
		{"api.perplexity.ai", "Perplexity"},
		{"api.deepseek.com", "deepseek"}, // The new requirement
		{"other.com", "other.com"},
	}

	for _, tt := range tests {
		got := normalizeProvider(tt.host)
		if got != tt.expected {
			t.Errorf("normalizeProvider(%q) = %q; want %q", tt.host, got, tt.expected)
		}
	}
}
