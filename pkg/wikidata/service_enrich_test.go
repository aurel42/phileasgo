package wikidata

import (
	"testing"
)

func TestDetermineBestArticle(t *testing.T) {
	tests := []struct {
		name       string
		article    Article
		lengths    map[string]map[string]int
		localLang  string
		userLang   string
		wantURL    string
		wantLength int
	}{
		{
			name: "Local is best",
			article: Article{
				Title:   "LocalTitle",
				TitleEn: "EnTitle",
			},
			lengths: map[string]map[string]int{
				"fr": {"LocalTitle": 1000},
				"en": {"EnTitle": 500},
			},
			localLang:  "fr",
			userLang:   "en",
			wantURL:    "https://fr.wikipedia.org/wiki/LocalTitle",
			wantLength: 1000,
		},
		{
			name: "English is best (longer)",
			article: Article{
				Title:   "LocalTitle",
				TitleEn: "EnTitle",
			},
			lengths: map[string]map[string]int{
				"fr": {"LocalTitle": 500},
				"en": {"EnTitle": 1000},
			},
			localLang:  "fr",
			userLang:   "en",
			wantURL:    "https://en.wikipedia.org/wiki/EnTitle",
			wantLength: 1000,
		},
		{
			name: "User lang is best",
			article: Article{
				Title:     "LocalTitle",
				TitleEn:   "EnTitle",
				TitleUser: "UserTitle",
			},
			lengths: map[string]map[string]int{
				"fr": {"LocalTitle": 500},
				"en": {"EnTitle": 600},
				"de": {"UserTitle": 1200},
			},
			localLang:  "fr",
			userLang:   "de",
			wantURL:    "https://de.wikipedia.org/wiki/UserTitle",
			wantLength: 1200,
		},
		{
			name: "Fallback to any (no lengths)",
			article: Article{
				Title:   "",
				TitleEn: "EnTitle",
			},
			lengths:    map[string]map[string]int{},
			localLang:  "fr",
			userLang:   "en",
			wantURL:    "https://en.wikipedia.org/wiki/EnTitle",
			wantLength: 0,
		},
		{
			name: "Spaces replaced",
			article: Article{
				TitleEn: "New York City",
			},
			lengths: map[string]map[string]int{
				"en": {"New York City": 100},
			},
			localLang:  "fr",
			userLang:   "en",
			wantURL:    "https://en.wikipedia.org/wiki/New_York_City",
			wantLength: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, length := determineBestArticle(&tt.article, tt.lengths, tt.localLang, tt.userLang)
			if url != tt.wantURL {
				t.Errorf("got url %q, want %q", url, tt.wantURL)
			}
			if length != tt.wantLength {
				t.Errorf("got length %d, want %d", length, tt.wantLength)
			}
		})
	}
}

func TestConstructPOI(t *testing.T) {
	mockIcon := func(cat string) string { return "icon-" + cat }

	tests := []struct {
		name     string
		article  Article
		lengths  map[string]map[string]int
		wantPOI  bool // true if not nil
		wantName string
		wantURL  string
	}{
		{
			name: "Standard POI",
			article: Article{
				QID:      "Q1",
				TitleEn:  "Tower",
				Category: "castle",
			},
			wantPOI:  true,
			wantName: "Tower",
			wantURL:  "https://en.wikipedia.org/wiki/Tower",
		},
		{
			name: "Unnamed (No Label) - Allowed for Rescue",
			article: Article{
				QID: "Q2",
			},
			wantPOI:  true, // Allowed to proceed to rescue
			wantName: "",
			wantURL:  "https://www.wikidata.org/wiki/Q2",
		},
		{
			name: "Unnamed Rescued by Label",
			article: Article{
				QID:   "Q3",
				Label: "Ghost Castle",
			},
			wantPOI:  true,
			wantName: "", // Now empty, waiting for rescue
			wantURL:  "https://www.wikidata.org/wiki/Q3",
		},
		{
			name: "Unnamed (Empty Label) - Allowed for Rescue",
			article: Article{
				QID:   "Q4",
				Label: "",
			},
			wantPOI:  true, // Allowed to proceed to rescue
			wantName: "",
			wantURL:  "https://www.wikidata.org/wiki/Q4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := constructPOI(&tt.article, tt.lengths, "fr", "en", mockIcon)
			if (p != nil) != tt.wantPOI {
				t.Fatalf("got POI %v, wantPOI %v", p, tt.wantPOI)
			}
			if p != nil {
				if p.NameEn != tt.wantName && p.NameLocal != tt.wantName {
					t.Errorf("got NameEn=%q NameLocal=%q, want %q", p.NameEn, p.NameLocal, tt.wantName)
				}
				if p.WPURL != tt.wantURL {
					t.Errorf("got URL %q, want %q", p.WPURL, tt.wantURL)
				}
				if p.Icon != "icon-"+tt.article.Category && tt.article.Category != "" {
					t.Errorf("got Icon %q, want icon-%s", p.Icon, tt.article.Category)
				}
			}
		})
	}
}
