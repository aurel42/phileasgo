package tts

import (
	"errors"
	"testing"
)

func TestIsFatalError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "FatalError 429",
			err:      NewFatalError(429, "Too Many Requests"),
			expected: true,
		},
		{
			name:     "FatalError 500",
			err:      NewFatalError(500, "Internal Server Error"),
			expected: true,
		},
		{
			name:     "Standard Error",
			err:      errors.New("some regular error"),
			expected: false,
		},
		{
			name:     "Nil Error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsFatalError(tt.err); got != tt.expected {
				t.Errorf("IsFatalError() = %v, want %v", got, tt.expected)
			}
		})
	}
}
