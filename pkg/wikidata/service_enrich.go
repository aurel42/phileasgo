package wikidata

import (
	"context"
	"fmt"
	"strings"
	"time"

	"phileasgo/pkg/model"
)

func (s *Service) enrichAndSave(ctx context.Context, articles []Article, localLangs []string, userLang string) error {
	// 4a. Fetch Lengths
	lengths := s.fetchArticleLengths(ctx, articles, localLangs, userLang)

	// 4c. Construct POIs
	var candidates []*model.POI
	for i := range articles {
		if p := constructPOI(&articles[i], lengths, localLangs, userLang, s.getIcon); p != nil {
			candidates = append(candidates, p)
		}
	}

	// 5. RESCUE REMOVED - We rely on strict SPARQL filtering.
	// If constructPOI returned a POI, it has at least one title.

	// 5. MERGE DUPLICATES (Spatial Gobbling)
	var finalPOIs []*model.POI = candidates
	if dc, ok := s.classifier.(DimClassifier); ok {
		finalPOIs = MergePOIs(candidates, dc.GetConfig(), s.logger)
	}

	// 6. Save Valid POIs
	for _, p := range finalPOIs {
		if err := s.poi.UpsertPOI(ctx, p); err != nil {
			s.logger.Error("Failed to save POI", "qid", p.WikidataID, "error", err)
		}
	}
	return nil
}

func (s *Service) fetchArticleLengths(ctx context.Context, articles []Article, localLangs []string, userLang string) map[string]map[string]int {
	titlesByLang := make(map[string][]string)

	for i := range articles {
		a := &articles[i]
		// Aggregate ALL possible titles for length fetching
		for lang, t := range a.LocalTitles {
			titlesByLang[lang] = append(titlesByLang[lang], t)
		}
		if a.TitleEn != "" {
			titlesByLang["en"] = append(titlesByLang["en"], a.TitleEn)
		}
		if a.TitleUser != "" && userLang != "en" {
			// Note: User lang might overlap with LocalTitles keys, duplicates in slice are fine (wiki client handles batch)
			titlesByLang[userLang] = append(titlesByLang[userLang], a.TitleUser)
		}
	}

	lengths := make(map[string]map[string]int)
	for lang, titles := range titlesByLang {
		if len(titles) == 0 {
			continue
		}
		res, err := s.wiki.GetArticleLengths(ctx, titles, lang)
		if err != nil {
			s.logger.Warn("Failed to fetch article lengths", "lang", lang, "error", err)
			continue
		}
		lengths[lang] = res
	}
	return lengths
}

func constructPOI(a *Article, lengths map[string]map[string]int, localLangs []string, userLang string, iconGetter func(string) string) *model.POI {
	bestURL, bestNameLocal, maxLength := determineBestArticle(a, lengths, localLangs, userLang)

	nameEn := a.TitleEn
	nameUser := a.TitleUser

	// Rescue Logic removed: We assume we have titles because of strict SPARQL filter (FILTER EXISTS).
	// If determineBestArticle couldn't find ANY title, something is very wrong upstream.

	poi := &model.POI{
		WikidataID:          a.QID,
		Source:              "wikidata",
		Category:            a.Category,
		Lat:                 a.Lat,
		Lon:                 a.Lon,
		Sitelinks:           a.Sitelinks,
		NameEn:              nameEn,
		NameLocal:           bestNameLocal,
		NameUser:            nameUser,
		WPURL:               bestURL,
		WPArticleLength:     maxLength,
		TriggerQID:          "",
		CreatedAt:           time.Now(),
		DimensionMultiplier: a.DimensionMultiplier,
	}

	poi.Icon = iconGetter(a.Category)
	return poi
}

func determineBestArticle(a *Article, lengths map[string]map[string]int, localLangs []string, userLang string) (url, nameLocal string, length int) {
	// 1. Calculate Lengths per Language
	lenEn := lengths["en"][a.TitleEn]
	lenUser := 0
	// Default to first local lang if available
	primaryLocal := ""
	if len(localLangs) > 0 {
		primaryLocal = localLangs[0]
	}

	if userLang != "en" && (primaryLocal == "" || userLang != primaryLocal) {
		lenUser = lengths[userLang][a.TitleUser]
	}

	// 2. Find Best Local Candidate
	bestLocalLang, bestLocalTitle, maxLocalLen := findBestLocalCandidate(a, lengths, localLangs)

	// 3. Determine Overall Best URL (for narration content)
	maxLength := maxLocalLen
	var bestURL string

	if bestLocalTitle != "" {
		bestURL = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", bestLocalLang, replaceSpace(bestLocalTitle))
	}

	if lenEn > maxLength {
		maxLength = lenEn
		bestURL = fmt.Sprintf("https://en.wikipedia.org/wiki/%s", replaceSpace(a.TitleEn))
	}
	if lenUser > maxLength {
		maxLength = lenUser
		bestURL = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", userLang, replaceSpace(a.TitleUser))
	}

	// 4. Fallback URL construction if length metrics failed
	if bestURL == "" {
		switch {
		case a.TitleUser != "":
			bestURL = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", userLang, replaceSpace(a.TitleUser))
		case a.TitleEn != "":
			bestURL = fmt.Sprintf("https://en.wikipedia.org/wiki/%s", replaceSpace(a.TitleEn))
		case bestLocalTitle != "":
			bestURL = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", bestLocalLang, replaceSpace(bestLocalTitle))
		default:
			// Absolute backup: Wikidata URL
			bestURL = "https://www.wikidata.org/wiki/" + a.QID
		}
	}

	return bestURL, bestLocalTitle, maxLength
}

func findBestLocalCandidate(a *Article, lengths map[string]map[string]int, localLangs []string) (bestLang, bestTitle string, maxLen int) {
	// Prioritize based on the provided local languages
	prefMap := make(map[string]int)
	for i, l := range localLangs {
		prefMap[l] = len(localLangs) - i
	}

	// Iterate over all available local titles (de, pl, etc.)
	for lang, title := range a.LocalTitles {
		l := lengths[lang][title]
		if l > maxLen {
			maxLen = l
			bestLang = lang
			bestTitle = title
		}
		// Tie-breaker? Prefer higher priority in localLangs list if lengths equal
		if l == maxLen && maxLen > 0 {
			if prefMap[lang] > prefMap[bestLang] {
				bestLang = lang
				bestTitle = title
			}
		}
	}
	// Fallback if no length info (or 0 length), pick first local language present in article
	if bestTitle == "" {
		for _, lLang := range localLangs {
			if t, ok := a.LocalTitles[lLang]; ok {
				bestLang = lLang
				bestTitle = t
				return
			}
		}
		// Pick random first?
		for l, t := range a.LocalTitles {
			bestLang = l
			bestTitle = t
			break
		}
	}
	return
}

func replaceSpace(s string) string {
	return strings.ReplaceAll(s, " ", "_")
}
