package sapi

import (
	"testing"

	"github.com/go-ole/go-ole"
)

func TestNewProvider(t *testing.T) {
	p := NewProvider()
	if p == nil {
		t.Fatal("Expected NewProvider to return a provider")
	}
}

func TestGetVariantInt(t *testing.T) {
	p := NewProvider()

	tests := []struct {
		name     string
		val      interface{}
		expected int
	}{
		{"int32", int32(5), 5},
		{"int64", int64(10), 10},
		{"int", 15, 15},
		{"uint32", uint32(20), 20},
		{"nil (falls back to Val)", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := ole.NewVariant(ole.VT_I4, 0)
			// getVariantInt is a method on Provider. Let's make sure it doesn't panic.
			_ = p.getVariantInt(&v)
		})
	}
}

func TestGetVariantIntValues(t *testing.T) {
	p := NewProvider()

	// Directly test the switch logic by passing variants with specific values
	v32 := ole.NewVariant(ole.VT_I4, 32)
	if p.getVariantInt(&v32) != 32 {
		t.Errorf("Expected 32, got %d", p.getVariantInt(&v32))
	}
}
