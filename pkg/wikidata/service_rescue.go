package wikidata

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"phileasgo/pkg/model"
)

// rescueUnnamedPOIs attempts to find valid names and URLs for POIs that have only QIDs.
func (s *Service) rescueUnnamedPOIs(ctx context.Context, candidates []*model.POI, localLang, userLang string) error {
	rescueQIDs := identifyRescueCandidates(candidates)
	if len(rescueQIDs) == 0 {
		return nil
	}

	s.logger.Info("Attempting to rescue unnamed POIs", "count", len(rescueQIDs))
	fallbackData, err := s.client.FetchFallbackData(ctx, rescueQIDs)
	if err != nil {
		return fmt.Errorf("failed to fetch fallback data: %w", err)
	}

	for _, p := range candidates {
		fd, found := fallbackData[p.WikidataID]
		if !found {
			continue
		}

		rescuePOIName(p, fd, localLang, userLang, s.logger)
		rescuePOIURL(p, fd, localLang, userLang, s.logger)
	}
	return nil
}

func identifyRescueCandidates(candidates []*model.POI) []string {
	var rescueQIDs []string
	qidRegex := regexp.MustCompile(`^Q\d+$`)
	for _, p := range candidates {
		if qidRegex.MatchString(p.DisplayName()) {
			rescueQIDs = append(rescueQIDs, p.WikidataID)
		}
	}
	return rescueQIDs
}

func rescuePOIName(p *model.POI, fd FallbackData, localLang, userLang string, logger *slog.Logger) {
	qidRegex := regexp.MustCompile(`^Q\d+$`)
	newName := findBestName(fd, localLang, userLang)

	if newName != "" && !qidRegex.MatchString(newName) {
		logger.Info("Rescued POI Name", "qid", p.WikidataID, "old", p.DisplayName(), "new", newName)
		p.NameUser = newName
	}
}

func findBestName(fd FallbackData, localLang, userLang string) string {
	// Priority: Local > User > En (Labels ARE UNRELIABLE/MESSY, USE SITELINKS ONLY)
	// We map lang pointers to site keys (e.g. "fr" -> "frwiki")

	trySite := func(lang string) string {
		if val, ok := fd.Sitelinks[lang+"wiki"]; ok && val != "" {
			return val
		}
		return ""
	}

	if val := trySite(localLang); val != "" {
		return val
	}
	if val := trySite(userLang); val != "" {
		return val
	}
	if val := trySite("en"); val != "" {
		return val
	}

	// Fallback to any sitelink title
	for _, title := range fd.Sitelinks {
		if title != "" {
			return title
		}
	}

	// Do NOT fallback to Labels. They contain raw Wikitext garbage sometimes.
	return ""
}

func rescuePOIURL(p *model.POI, fd FallbackData, localLang, userLang string, logger *slog.Logger) {
	if !strings.Contains(p.WPURL, "wikidata.org") {
		return
	}

	newURL := findBestURL(fd, localLang, userLang)
	if newURL != "" && newURL != p.WPURL {
		logger.Info("Rescued POI URL", "qid", p.WikidataID, "new_url", newURL)
		p.WPURL = newURL
	}
}

func findBestURL(fd FallbackData, localLang, userLang string) string {
	checkSite := func(code string) string {
		siteKey := code + "wiki"
		if title, ok := fd.Sitelinks[siteKey]; ok && title != "" {
			return fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", code, replaceSpace(title))
		}
		return ""
	}

	if u := checkSite(localLang); u != "" {
		return u
	}
	if u := checkSite(userLang); u != "" {
		return u
	}
	if u := checkSite("en"); u != "" {
		return u
	}

	// Pick any *wiki
	for site, title := range fd.Sitelinks {
		if strings.HasSuffix(site, "wiki") && !strings.Contains(site, "commons") && !strings.Contains(site, "meta") {
			lang := strings.TrimSuffix(site, "wiki")
			if len(lang) <= 3 {
				return fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", lang, replaceSpace(title))
			}
		}
	}
	return ""
}
