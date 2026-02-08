package main

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		s        string
		l        int
		expected string
	}{
		{"Hello World", 5, "He..."},
		{"Hello World", 20, "Hello World"},
		{"Hello", 5, "Hello"},
		{"Hello", 3, "Hel"},
		{"", 5, ""},
		{"Long text", 4, "L..."},
	}

	for _, tt := range tests {
		result := truncate(tt.s, tt.l)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.l, result, tt.expected)
		}
	}
}
