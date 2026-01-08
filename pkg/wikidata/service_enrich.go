package wikidata

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"phileasgo/pkg/model"
)

func (s *Service) enrichAndSave(ctx context.Context, articles []Article, localLang, userLang string) error {
	// 4a. Fetch Lengths
	lengths := s.fetchArticleLengths(ctx, articles, localLang, userLang)

	// 4c. Construct POIs
	var candidates []*model.POI
	for i := range articles {
		if p := constructPOI(&articles[i], lengths, localLang, userLang, s.getIcon); p != nil {
			candidates = append(candidates, p)
		}
	}

	// 5. RESCUE UNNAMED POIS
	if err := s.rescueUnnamedPOIs(ctx, candidates, localLang, userLang); err != nil {
		s.logger.Warn("Rescue failed", "error", err)
	}

	// 5b. FILTER UNRESOLVED: If rescue failed (still QID name), drop them.
	// We strictly want Wikipedia titles.
	var distinctCandidates []*model.POI
	// We reuse 'identifyRescueCandidates' logic which uses regex.
	// But simple DisplayName == QID check is enough if we trust DisplayName impl.
	// Let's use the regex to be safe and consistent.
	qidRegex := regexp.MustCompile(`^Q\d+$`)
	for _, p := range candidates {
		if !qidRegex.MatchString(p.DisplayName()) {
			distinctCandidates = append(distinctCandidates, p)
		}
	}
	candidates = distinctCandidates

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

func (s *Service) fetchArticleLengths(ctx context.Context, articles []Article, localLang, userLang string) map[string]map[string]int {
	titlesByLang := make(map[string][]string)

	for i := range articles {
		a := &articles[i]
		if a.Title != "" {
			titlesByLang[localLang] = append(titlesByLang[localLang], a.Title)
		}
		if a.TitleEn != "" {
			titlesByLang["en"] = append(titlesByLang["en"], a.TitleEn)
		}
		if a.TitleUser != "" && userLang != "en" && userLang != localLang {
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

func constructPOI(a *Article, lengths map[string]map[string]int, localLang, userLang string, iconGetter func(string) string) *model.POI {
	bestURL, maxLength := determineBestArticle(a, lengths, localLang, userLang)

	nameLocal := a.Title
	nameEn := a.TitleEn
	nameUser := a.TitleUser

	// If no titles found (Nameless Ghost), we leave names empty.
	// This allows rescueUnnamedPOIs to detect it (via DisplayName == QID)
	// and attempt to find a version in another language.
	// If rescue fails, we will filter it out later.
	if nameLocal == "" && nameEn == "" && nameUser == "" {
		if bestURL == "" {
			bestURL = "https://www.wikidata.org/wiki/" + a.QID
		}
	}

	poi := &model.POI{
		WikidataID:          a.QID,
		Source:              "wikidata",
		Category:            a.Category,
		Lat:                 a.Lat,
		Lon:                 a.Lon,
		Sitelinks:           a.Sitelinks,
		NameEn:              nameEn,
		NameLocal:           nameLocal,
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

func determineBestArticle(a *Article, lengths map[string]map[string]int, localLang, userLang string) (url string, length int) {
	lenLocal := lengths[localLang][a.Title]
	lenEn := lengths["en"][a.TitleEn]
	lenUser := 0
	if userLang != "en" && userLang != localLang {
		lenUser = lengths[userLang][a.TitleUser]
	}

	maxLength := lenLocal
	var bestURL string
	if a.Title != "" {
		bestURL = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", localLang, replaceSpace(a.Title))
	}

	if lenEn > maxLength {
		maxLength = lenEn
		bestURL = fmt.Sprintf("https://en.wikipedia.org/wiki/%s", replaceSpace(a.TitleEn))
	}
	if lenUser > maxLength {
		maxLength = lenUser
		bestURL = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", userLang, replaceSpace(a.TitleUser))
	}

	if bestURL == "" {
		switch {
		case a.TitleUser != "":
			bestURL = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", userLang, replaceSpace(a.TitleUser))
		case a.TitleEn != "":
			bestURL = fmt.Sprintf("https://en.wikipedia.org/wiki/%s", replaceSpace(a.TitleEn))
		case a.Title != "":
			bestURL = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", localLang, replaceSpace(a.Title))
		}
	}
	return bestURL, maxLength
}

func replaceSpace(s string) string {
	return strings.ReplaceAll(s, " ", "_")
}
