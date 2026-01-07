package wikidata

import (
	"regexp"
	"testing"
)

// MockClient is a wrapper to intercept FetchFallbackData calls if needed,
// or we can test the service method logic by mocking the *response* of the client if we could inject it.
// Since Service uses a concrete *Client, we can't easily mock the client method *calls*.
// However, we can use a "live" client if we want integeration tests, but that's bad.
// Or we can rely on the fact that if FetchFallbackData returns empty/error, nothing breaks.
//
// BUT, to test the LOGIC inside enrichAndSave, we need FetchFallbackData to return something.
// Since struct Client is concrete in Service, we can't mock it unless we modify Service to use an interface.
// Refactoring to interface is the "clean" way, but maybe too big for this hotfix.
//
// Alternative: Modify the test to use a real client but mock the network transport?
// The request.Client uses http.Client. We can mock the HTTP transport.
//
// Let's try to verify the REGEX and logic by extracting the loop into a testable helper or
// by relying on the fact that we can't easily test `enrichAndSave` without a mock client
// and just adding a test for `constructPOI` to ensure it produces the "unnamed" candidate.

func TestConstructPOI_Unnamed(t *testing.T) {

	// Case 1: Title missing, Label is QID -> Produces Candidate with Name=QID (for rescue)
	a1 := &Article{
		QID:       "Q123",
		Title:     "",
		TitleEn:   "",
		TitleUser: "",
		Label:     "Q123",
		Lat:       50.0,
		Lon:       14.0,
		Category:  "Castle",
	}
	lengths := map[string]map[string]int{
		"en": {},
		"cs": {},
	}

	mockIcon := func(c string) string { return "" }
	p1 := constructPOI(a1, lengths, "cs", "en", mockIcon)

	if p1 == nil {
		t.Fatal("Expected valid POI for rescue, got nil")
	}
	if p1.DisplayName() != "Q123" {
		t.Errorf("Expected DisplayName='Q123', got '%s'", p1.DisplayName())
	}
	if p1.NameUser != "" {
		t.Errorf("Expected NameUser empty (before rescue), got '%s'", p1.NameUser)
	}
	if p1.WPURL != "https://www.wikidata.org/wiki/Q123" {
		t.Errorf("Expected fallback URL, got '%s'", p1.WPURL)
	}

	// Verify Regex
	qidRegex := regexp.MustCompile(`^Q\d+$`)
	if !qidRegex.MatchString(p1.DisplayName()) {
		t.Errorf("Expected DisplayName '%s' to match QID regex", p1.DisplayName())
	}
}
