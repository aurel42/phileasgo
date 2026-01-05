package model

import "testing"

func TestModel(t *testing.T) {
	// Simple instantiation test to ensure structs compile and tags are present
	p := POI{
		WikidataID: "Q123",
	}
	if p.WikidataID != "Q123" {
		t.Error("Model instantiation failed")
	}
}
