package wikidata

import (
	"context"
	"errors"
	"testing"
)

func TestHydrateCandidates_Filtering(t *testing.T) {
	// Mock Client
	mockClient := &MockWikidataClient{
		FetchFallbackDataFunc: func(ctx context.Context, ids []string, allowedSites []string) (map[string]FallbackData, error) {
			// Verify we received a site filter!
			if len(allowedSites) == 0 {
				return nil, errors.New("expected site filter, got none")
			}

			// Verify content of filter for specific test logic
			// For simplicity, we just return data that includes "forbidden" languages
			// to ensure the service filters them out if the client didn't (double safety)
			// OR validation that client filter was passed.
			// Since we use the client filter, we trust the client does its job (tested in client_test.go).
			// Here we mainly verify that allowedSites was populated correctly passed from input.

			res := make(map[string]FallbackData)
			res["Q1"] = FallbackData{
				Labels: map[string]string{"en": "Tower"},
				Sitelinks: map[string]string{
					"enwiki": "Tower",
					"dewiki": "Turm",
					"ruwiki": "Башня", // Should be filtered out if not requested?
					// Wait, if we use sitefilter in API, the API won't return ruwiki.
					// But if we mock returning it, our service logic inside hydrateCandidates
					// should ALSO filter it out as a second layer of defense (as per plan).
				},
			}
			return res, nil
		},
	}

	svc := &Service{
		client:   mockClient,
		userLang: "de",
	}

	candidates := []Article{
		{QID: "Q1"},
	}

	// Case 1: Filter Enabled (en + de)
	allowedLangs := []string{"en", "de"}
	hydrated, err := svc.hydrateCandidates(context.Background(), candidates, allowedLangs)
	if err != nil {
		t.Fatalf("hydrateCandidates failed: %v", err)
	}

	if len(hydrated) != 1 {
		t.Fatalf("Expected 1 hydrated candidate, got %d", len(hydrated))
	}

	cand := hydrated[0]
	// Check LocalTitles
	if cand.TitleEn != "Tower" {
		t.Error("Missing 'en' title (TitleEn)")
	}
	if _, ok := cand.LocalTitles["de"]; !ok {
		t.Error("Missing 'de' title")
	}
	if _, ok := cand.LocalTitles["ru"]; ok {
		t.Error("Should NOT have 'ru' title")
	}

	// Case 2: Filter with different lang (en only)
	allowedLangs2 := []string{"en"}
	mockClient.FetchFallbackDataFunc = func(ctx context.Context, ids []string, allowedSites []string) (map[string]FallbackData, error) {
		// Verify only enwiki requested
		if len(allowedSites) != 1 || allowedSites[0] != "enwiki" {
			t.Errorf("Expected [enwiki], got %v", allowedSites)
		}
		return map[string]FallbackData{
			"Q1": {
				Labels: map[string]string{"en": "Tower"},
				Sitelinks: map[string]string{
					"enwiki": "Tower",
					"ruwiki": "ShouldNotBeReturnedByRealAPIButMockMight",
				},
			},
		}, nil
	}

	hydrated2, err := svc.hydrateCandidates(context.Background(), candidates, allowedLangs2)
	if err != nil {
		t.Fatalf("hydrateCandidates failed: %v", err)
	}
	cand2 := hydrated2[0]
	if _, ok := cand2.LocalTitles["ru"]; ok {
		t.Error("Should NOT have 'ru' title even if API returned it (secondary filter check)")
	}
}
