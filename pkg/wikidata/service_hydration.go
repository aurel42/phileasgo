package wikidata

import (
	"context"
	"strings"
)

func (s *Service) hydrateCandidates(ctx context.Context, candidates []Article) ([]Article, error) {
	qids := make([]string, len(candidates))
	for i := range candidates {
		qids[i] = candidates[i].QID
	}

	// Fetch Fallback Data (Sitelinks + Labels)
	// This uses the Wikidata API (wbgetentities) which is much faster/stable than SPARQL joins
	fallbackData, err := s.client.FetchFallbackData(ctx, qids)
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

		// Map Sitelinks to Titles
		// "enwiki" -> TitleEn
		// "dewiki" -> LocalTitles["de"]
		cand.LocalTitles = make(map[string]string)

		for site, title := range data.Sitelinks {
			if site == "enwiki" {
				cand.TitleEn = title
				continue
			}
			// Simple mapping: "frwiki" -> "fr"
			if strings.HasSuffix(site, "wiki") {
				lang := strings.TrimSuffix(site, "wiki")
				// Filter out non-language wikis if needed (commons, etc - unlikely to match simple suffix)
				if len(lang) == 2 || len(lang) == 3 { // rough check for iso codes
					cand.LocalTitles[lang] = title
				}
			}
		}

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

func (s *Service) fetchMissingMetadata(ctx context.Context, toFetch []string, metaCache map[string]EntityMetadata) error {
	if len(toFetch) == 0 {
		return nil
	}
	chunkSize := 50
	for i := 0; i < len(toFetch); i += chunkSize {
		end := i + chunkSize
		if end > len(toFetch) {
			end = len(toFetch)
		}
		chunk := toFetch[i:end]

		meta, err := s.client.GetEntitiesBatch(ctx, chunk)
		if err != nil {
			s.logger.Warn("Wikidata batch fetch failed", "error", err, "chunk_size", len(chunk))
			continue
		}
		for id, m := range meta {
			metaCache[id] = m
		}
	}
	return nil
}
