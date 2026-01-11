package wikidata

import (
	"context"
	"strings"
)

func (s *Service) hydrateCandidates(ctx context.Context, candidates []Article, allowedLangs []string) ([]Article, error) {
	qids := make([]string, len(candidates))
	for i := range candidates {
		qids[i] = candidates[i].QID
	}

	// Prepare Site Filter
	var allowedSites []string
	allowedCodes := make(map[string]bool)
	if len(allowedLangs) > 0 {
		for _, lang := range allowedLangs {
			allowedSites = append(allowedSites, lang+"wiki")
			allowedCodes[lang] = true
		}
	}

	// Fetch FallbackData with Site Filter
	// This uses the Wikidata API (wbgetentities) which is much faster/stable than SPARQL joins
	fallbackData, err := s.client.FetchFallbackData(ctx, qids, allowedSites)
	if err != nil {
		return nil, err
	}

	hydrated := make([]Article, 0, len(candidates))
	for i := range candidates {
		cand := candidates[i]
		data, found := fallbackData[cand.QID]
		if !found {
			// e.g. Merged/Redirected? Skip/Drop.
			continue
		}

		s.processSitelinks(&cand, data.Sitelinks, allowedCodes) // Pass pointer to modify

		// Set User Title (if user lang matches)
		if t, ok := cand.LocalTitles[s.userLang]; ok {
			cand.TitleUser = t
		}

		// Map Label (if we ever decide to use it)
		if lbl, ok := data.Labels["en"]; ok {
			cand.Label = lbl
		}

		hydrated = append(hydrated, cand)
	}

	return hydrated, nil
}

func (s *Service) processSitelinks(cand *Article, sitelinks map[string]string, allowedCodes map[string]bool) {
	cand.LocalTitles = make(map[string]string)

	for site, title := range sitelinks {
		if site == "enwiki" {
			cand.TitleEn = title
			continue
		}
		// Simple mapping: "frwiki" -> "fr"
		if strings.HasSuffix(site, "wiki") {
			lang := strings.TrimSuffix(site, "wiki")

			// Secondary Filter (Double defense)
			// Only accept if allowed, or if no filter was provided (safety)
			if len(allowedCodes) > 0 && !allowedCodes[lang] {
				continue
			}

			// Filter out non-language wikis if needed (commons, etc - unlikely to match simple suffix)
			if len(lang) == 2 || len(lang) == 3 { // rough check for iso codes
				cand.LocalTitles[lang] = title
			}
		}
	}
}
