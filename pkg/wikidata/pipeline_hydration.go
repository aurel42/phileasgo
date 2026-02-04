package wikidata

import (
	"context"
	"strings"
)

func (p *Pipeline) hydrateCandidates(ctx context.Context, candidates []Article, allowedLangs []string) ([]Article, error) {
	qids := make([]string, len(candidates))
	for i := range candidates {
		qids[i] = candidates[i].QID
	}

	var allowedSites []string
	allowedCodes := make(map[string]bool)
	if len(allowedLangs) > 0 {
		for _, lang := range allowedLangs {
			allowedSites = append(allowedSites, lang+"wiki")
			allowedCodes[lang] = true
		}
	}

	fallbackData, err := p.client.FetchFallbackData(ctx, qids, allowedSites)
	if err != nil {
		return nil, err
	}

	hydrated := make([]Article, 0, len(candidates))
	for i := range candidates {
		cand := candidates[i]
		data, found := fallbackData[cand.QID]
		if !found {
			continue
		}

		p.processSitelinks(&cand, data.Sitelinks, allowedCodes)

		userLang := p.cfgProv.TargetLanguage(ctx)
		if userLang != "" {
			// Normalize userLang (e.g. "en-US" -> "en")
			normalizedLang := userLang
			if len(userLang) > 2 {
				normalizedLang = strings.Split(userLang, "-")[0]
			}
			if t, ok := cand.LocalTitles[normalizedLang]; ok {
				cand.TitleUser = t
			}
		}

		if lbl, ok := data.Labels["en"]; ok {
			cand.Label = lbl
		}

		hydrated = append(hydrated, cand)
	}

	return hydrated, nil
}

func (p *Pipeline) processSitelinks(cand *Article, sitelinks map[string]string, allowedCodes map[string]bool) {
	cand.LocalTitles = make(map[string]string)

	for site, title := range sitelinks {
		if site == "enwiki" {
			cand.TitleEn = title
			continue
		}
		if strings.HasSuffix(site, "wiki") {
			lang := strings.TrimSuffix(site, "wiki")
			if len(allowedCodes) > 0 && !allowedCodes[lang] {
				continue
			}
			if len(lang) == 2 || len(lang) == 3 {
				cand.LocalTitles[lang] = title
			}
		}
	}
}
