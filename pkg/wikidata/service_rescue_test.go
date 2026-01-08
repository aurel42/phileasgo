package wikidata

import (
	"log/slog"
	"testing"

	"phileasgo/pkg/model"
)

func TestIdentifyRescueCandidates(t *testing.T) {
	tests := []struct {
		name       string
		candidates []*model.POI
		wantQIDs   []string
	}{
		{
			name: "All valid named",
			candidates: []*model.POI{
				{WikidataID: "Q1", NameUser: "Tower"},
				{WikidataID: "Q2", NameEn: "Bridge"},
			},
			wantQIDs: nil,
		},
		{
			name: "One unnamed (QID name)",
			candidates: []*model.POI{
				{WikidataID: "Q1", NameUser: "Tower"},
				{WikidataID: "Q123", NameUser: "Q123"}, // DisplayName returns Q123
			},
			wantQIDs: []string{"Q123"},
		},
		{
			name: "Empty name falls back to QID",
			candidates: []*model.POI{
				{WikidataID: "Q999", NameUser: "", NameEn: "", NameLocal: ""},
			},
			wantQIDs: []string{"Q999"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := identifyRescueCandidates(tt.candidates)
			if len(got) != len(tt.wantQIDs) {
				t.Fatalf("len(got) = %d, want %d", len(got), len(tt.wantQIDs))
			}
			for i, qid := range got {
				if qid != tt.wantQIDs[i] {
					t.Errorf("got[%d] = %s, want %s", i, qid, tt.wantQIDs[i])
				}
			}
		})
	}
}

func TestFindBestName(t *testing.T) {
	tests := []struct {
		name      string
		fd        FallbackData
		localLang string
		userLang  string
		want      string
	}{
		{
			name: "Local lang LABEL ignored",
			fd: FallbackData{
				Labels: map[string]string{"fr": "Tour", "en": "Tower"},
			},
			localLang: "fr",
			userLang:  "de",
			want:      "", // Labels are ignored
		},
		{
			name: "Local lang SITELINK used",
			fd: FallbackData{
				Sitelinks: map[string]string{"frwiki": "Tour_Wikipedia"},
			},
			localLang: "fr",
			userLang:  "de",
			want:      "Tour_Wikipedia",
		},
		{
			name: "User lang SITELINK used",
			fd: FallbackData{
				Sitelinks: map[string]string{"dewiki": "Turm_Wiki"},
			},
			localLang: "fr",
			userLang:  "de",
			want:      "Turm_Wiki",
		},
		{
			// Belgium Case: Mapper thinks it's French (local=fr), User is English.
			// POI is only in Dutch (nlwiki). Should fallback to nlwiki.
			name: "Belgium Case (Local mismatch, fallback to other)",
			fd: FallbackData{
				Sitelinks: map[string]string{"nlwiki": "Atomium_Dutch"},
			},
			localLang: "fr",
			userLang:  "en",
			want:      "Atomium_Dutch",
		},
		{
			// Exclave Case: Flying over Country A (local=A-lang), User=English.
			// POI is physically in B-lang theory but actually only has C-lang?
			// Or simply: POI is exotic and only has 'ruwiki'.
			name: "Exclave/Border Case (No local/user match, fallback to any)",
			fd: FallbackData{
				Sitelinks: map[string]string{"ruwiki": "Russian_Only"},
			},
			localLang: "pl", // Poland
			userLang:  "en",
			want:      "Russian_Only",
		},
		{
			name: "Fallback to any VALID sitelink (jawiki)",
			fd: FallbackData{
				Labels:    map[string]string{},
				Sitelinks: map[string]string{"jawiki": "Tokyo_Tower_Wiki"},
			},
			localLang: "fr",
			userLang:  "en",
			want:      "Tokyo_Tower_Wiki",
		},
		{
			name: "Reject Category Namespace",
			fd: FallbackData{
				Sitelinks: map[string]string{"enwiki": "Category:Trees_in_Germany"},
			},
			localLang: "en",
			userLang:  "en",
			want:      "",
		},
		{
			name: "Reject File/Image Namespace",
			fd: FallbackData{
				Sitelinks: map[string]string{"enwiki": "File:Tree.jpg"},
			},
			localLang: "en",
			userLang:  "en",
			want:      "",
		},
		{
			name: "Reject Commons Wiki",
			fd: FallbackData{
				Sitelinks: map[string]string{"commonswiki": "Category:Naturdenkmal"},
			},
			localLang: "en",
			userLang:  "en",
			want:      "",
		},
		{
			name: "Reject Wikiquote",
			fd: FallbackData{
				Sitelinks: map[string]string{"enwikiquote": "Tree_Quotes"},
			},
			localLang: "en",
			userLang:  "en",
			want:      "",
		},
		{
			name: "Reject Low Quality Wikis (rowiki)",
			fd: FallbackData{
				Sitelinks: map[string]string{"rowiki": "Stieleiche_Bot_Stub"},
			},
			localLang: "de", // Even if not local, should fail. If local was ro, it would still fail.
			userLang:  "en",
			want:      "",
		},
		{
			name: "Reject Low Quality Wikis (cewiki)",
			fd: FallbackData{
				Sitelinks: map[string]string{"cewiki": "Bot_Article"},
			},
			localLang: "en",
			userLang:  "en",
			want:      "",
		},
		{
			name: "Reject Low Quality Wikis (warwiki)",
			fd: FallbackData{
				Sitelinks: map[string]string{"warwiki": "Bot_Article"},
			},
			localLang: "en",
			userLang:  "en",
			want:      "",
		},
		{
			name: "Empty",
			fd: FallbackData{
				Labels: map[string]string{},
			},
			localLang: "fr",
			userLang:  "en",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findBestName(tt.fd, tt.localLang, tt.userLang); got != tt.want {
				t.Errorf("findBestName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindBestURL(t *testing.T) {
	tests := []struct {
		name      string
		fd        FallbackData
		localLang string
		userLang  string
		want      string
	}{
		{
			name: "Local wiki match",
			fd: FallbackData{
				Sitelinks: map[string]string{"frwiki": "Tour Eiffel"},
			},
			localLang: "fr",
			userLang:  "en",
			want:      "https://fr.wikipedia.org/wiki/Tour_Eiffel",
		},
		{
			name: "User wiki match",
			fd: FallbackData{
				Sitelinks: map[string]string{"dewiki": "Eiffelturm"},
			},
			localLang: "fr",
			userLang:  "de",
			want:      "https://de.wikipedia.org/wiki/Eiffelturm",
		},
		{
			name: "English wiki match",
			fd: FallbackData{
				Sitelinks: map[string]string{"enwiki": "Eiffel Tower"},
			},
			localLang: "fr",
			userLang:  "es",
			want:      "https://en.wikipedia.org/wiki/Eiffel_Tower",
		},
		{
			name: "Fallback to any valid wiki",
			fd: FallbackData{
				Sitelinks: map[string]string{"jawiki": "Tokyo_Tower"},
			},
			localLang: "fr",
			userLang:  "en",
			want:      "https://ja.wikipedia.org/wiki/Tokyo_Tower",
		},
		{
			name: "Ignore commons/meta",
			fd: FallbackData{
				Sitelinks: map[string]string{
					"commonswiki": "File:Foo.jpg",
					"metawiki":    "Project:Foo",
					// NO valid wiki
				},
			},
			localLang: "fr",
			userLang:  "en",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findBestURL(tt.fd, tt.localLang, tt.userLang); got != tt.want {
				t.Errorf("findBestURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRescuePOIName(t *testing.T) {
	// Setup
	logger := slog.Default()

	tests := []struct {
		name        string
		poi         *model.POI
		fd          FallbackData
		wantName    string
		wantChanged bool
	}{
		{
			name: "Update QID name (Ignore Label)",
			poi: &model.POI{
				WikidataID: "Q1",
				NameUser:   "Q1",
			},
			fd: FallbackData{
				Labels: map[string]string{"en": "Real Name"},
			},
			wantName:    "Q1", // Label ignored
			wantChanged: false,
		},
		{
			name: "Update QID name (Use Sitelink)",
			poi: &model.POI{
				WikidataID: "Q1",
				NameUser:   "Q1",
			},
			fd: FallbackData{
				Sitelinks: map[string]string{"enwiki": "Real_Page"},
			},
			wantName:    "Real_Page",
			wantChanged: true, // Sitelink used
		},
		{
			name: "No valid label found",
			poi: &model.POI{
				WikidataID: "Q1",
				NameUser:   "Q1",
			},
			fd: FallbackData{
				Labels: map[string]string{},
			},
			wantName:    "Q1",
			wantChanged: false,
		},
		{
			name: "Found name is also QID (ignore)",
			poi: &model.POI{
				WikidataID: "Q1",
				NameUser:   "Q1",
			},
			fd: FallbackData{
				Labels: map[string]string{"en": "Q1"},
			},
			wantName:    "Q1",
			wantChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rescuePOIName(tt.poi, tt.fd, "fr", "en", logger)
			if tt.poi.NameUser != tt.wantName {
				t.Errorf("rescuePOIName() name = %v, want %v", tt.poi.NameUser, tt.wantName)
			}
		})
	}
}

func TestRescuePOIURL(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name    string
		poi     *model.POI
		fd      FallbackData
		wantURL string
	}{
		{
			name: "Replace wikidata URL",
			poi: &model.POI{
				WPURL: "https://www.wikidata.org/wiki/Q1",
			},
			fd: FallbackData{
				Sitelinks: map[string]string{"enwiki": "Foo"},
			},
			wantURL: "https://en.wikipedia.org/wiki/Foo",
		},
		{
			name: "Ignore non-wikidata URL",
			poi: &model.POI{
				WPURL: "https://fr.wikipedia.org/wiki/Bar",
			},
			fd: FallbackData{
				Sitelinks: map[string]string{"enwiki": "Foo"},
			},
			wantURL: "https://fr.wikipedia.org/wiki/Bar", // Should NOT change
		},
		{
			name: "No valid sitelink",
			poi: &model.POI{
				WPURL: "https://www.wikidata.org/wiki/Q1",
			},
			fd: FallbackData{
				Sitelinks: map[string]string{},
			},
			wantURL: "https://www.wikidata.org/wiki/Q1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rescuePOIURL(tt.poi, tt.fd, "fr", "en", logger)
			if tt.poi.WPURL != tt.wantURL {
				t.Errorf("rescuePOIURL() url = %v, want %v", tt.poi.WPURL, tt.wantURL)
			}
		})
	}
}
