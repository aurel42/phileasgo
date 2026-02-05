package wikidata

import (
	"context"
	"fmt"
	"strings"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
)

func (p *Pipeline) enrichAndSave(ctx context.Context, articles []Article, localLangs []string, userLang string) error {
	lengths := p.fetchArticleLengths(ctx, articles, localLangs, userLang)

	var candidates []*model.POI
	var rejectedQIDs []string

	for i := range articles {
		if poi := p.constructPOI(&articles[i], lengths, localLangs, userLang, p.getIcon); poi != nil {
			candidates = append(candidates, poi)
		} else {
			rejectedQIDs = append(rejectedQIDs, articles[i].QID)
		}
	}

	var finalPOIs []*model.POI
	cfg := p.classifier.GetConfig()
	var mergedRejected []string
	finalPOIs, mergedRejected = MergePOIs(candidates, cfg, p.logger)
	rejectedQIDs = append(rejectedQIDs, mergedRejected...)

	for _, poi := range finalPOIs {
		if err := p.poi.UpsertPOI(ctx, poi); err != nil {
			p.logger.Error("Failed to save POI", "qid", poi.WikidataID, "error", err)
		}
	}

	if len(rejectedQIDs) > 0 {
		toMark := make(map[string][]string)
		for _, qid := range rejectedQIDs {
			toMark[qid] = []string{"rejected"}
		}
		if err := p.store.MarkEntitiesSeen(ctx, toMark); err != nil {
			p.logger.Warn("Failed to mark rejected POIs as seen", "count", len(rejectedQIDs), "error", err)
		} else {
			p.logger.Debug("Marked rejected POIs as seen", "count", len(rejectedQIDs))
		}
	}

	return nil
}

func (p *Pipeline) fetchArticleLengths(ctx context.Context, articles []Article, localLangs []string, userLang string) map[string]map[string]int {
	titlesByLang := make(map[string][]string)

	for i := range articles {
		a := &articles[i]
		for lang, t := range a.LocalTitles {
			titlesByLang[lang] = append(titlesByLang[lang], t)
		}
		if a.TitleEn != "" {
			titlesByLang["en"] = append(titlesByLang["en"], a.TitleEn)
		}
		if a.TitleUser != "" && userLang != "en" {
			titlesByLang[userLang] = append(titlesByLang[userLang], a.TitleUser)
		}
	}

	lengths := make(map[string]map[string]int)
	for lang, titles := range titlesByLang {
		if len(titles) == 0 {
			continue
		}
		res, err := p.wiki.GetArticleLengths(ctx, titles, lang)
		if err != nil {
			p.logger.Warn("Failed to fetch article lengths", "lang", lang, "error", err)
			continue
		}
		lengths[lang] = res
	}
	return lengths
}

func (p *Pipeline) constructPOI(a *Article, lengths map[string]map[string]int, localLangs []string, userLang string, iconGetter func(string) string) *model.POI {
	bestURL, bestNameLocal, rawLength := p.determineBestArticle(a, lengths, localLangs, userLang)
	nameEn := a.TitleEn
	nameUser := a.TitleUser

	if nameEn == "" && bestNameLocal == "" && nameUser == "" {
		return nil
	}

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
		WPArticleLength:     rawLength,
		TriggerQID:          "",
		CreatedAt:           time.Now(),
		DimensionMultiplier: a.DimensionMultiplier,
	}

	poi.Icon = iconGetter(a.Category)
	return poi
}

func (p *Pipeline) determineBestArticle(a *Article, lengths map[string]map[string]int, localLangs []string, userLang string) (url, nameLocal string, rawLength int) {
	lenEn := lengths["en"][a.TitleEn]
	adjLenEn := p.density.GetAdjustedLength(lenEn, "https://en.wikipedia.org/wiki/"+p.replaceSpace(a.TitleEn))

	lenUser := 0
	adjLenUser := 0
	primaryLocal := ""
	if len(localLangs) > 0 {
		primaryLocal = localLangs[0]
	}

	if userLang != "en" && (primaryLocal == "" || userLang != primaryLocal) {
		lenUser = lengths[userLang][a.TitleUser]
		adjLenUser = p.density.GetAdjustedLength(lenUser, fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", userLang, p.replaceSpace(a.TitleUser)))
	}

	bestLocalLang, bestLocalTitle, maxRawLocalLen, maxAdjLocalLen := p.findBestLocalCandidate(a, lengths, localLangs)

	maxAdjLength := maxAdjLocalLen
	rawLength = maxRawLocalLen
	var bestURL string

	if bestLocalTitle != "" {
		bestURL = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", bestLocalLang, p.replaceSpace(bestLocalTitle))
	}

	if adjLenEn > maxAdjLength {
		maxAdjLength = adjLenEn
		rawLength = lenEn
		bestURL = fmt.Sprintf("https://en.wikipedia.org/wiki/%s", p.replaceSpace(a.TitleEn))
	}
	if adjLenUser > maxAdjLength {
		rawLength = lenUser
		bestURL = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", userLang, p.replaceSpace(a.TitleUser))
	}

	if bestURL == "" {
		switch {
		case a.TitleUser != "":
			bestURL = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", userLang, p.replaceSpace(a.TitleUser))
		case a.TitleEn != "":
			bestURL = fmt.Sprintf("https://en.wikipedia.org/wiki/%s", p.replaceSpace(a.TitleEn))
		case bestLocalTitle != "":
			bestURL = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", bestLocalLang, p.replaceSpace(bestLocalTitle))
		default:
			bestURL = "https://www.wikidata.org/wiki/" + a.QID
		}
	}

	return bestURL, bestLocalTitle, rawLength
}

func (p *Pipeline) findBestLocalCandidate(a *Article, lengths map[string]map[string]int, localLangs []string) (bestLang, bestTitle string, maxRawLen, maxAdjLen int) {
	prefMap := make(map[string]int)
	for i, l := range localLangs {
		prefMap[l] = len(localLangs) - i
	}

	for lang, title := range a.LocalTitles {
		url := fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", lang, p.replaceSpace(title))
		rawLen := lengths[lang][title]
		adjLen := p.density.GetAdjustedLength(rawLen, url)

		if adjLen > maxAdjLen {
			maxAdjLen = adjLen
			maxRawLen = rawLen
			bestLang = lang
			bestTitle = title
		}
		if adjLen == maxAdjLen && maxAdjLen > 0 {
			if prefMap[lang] > prefMap[bestLang] {
				bestLang = lang
				bestTitle = title
			}
		}
	}
	if bestTitle == "" {
		for _, lLang := range localLangs {
			if t, ok := a.LocalTitles[lLang]; ok {
				bestLang = lLang
				bestTitle = t
				return
			}
		}
		if len(a.LocalTitles) > 0 {
			for l, t := range a.LocalTitles {
				bestLang = l
				bestTitle = t
				return
			}
		}
	}
	return
}

func (p *Pipeline) replaceSpace(s string) string {
	return strings.ReplaceAll(s, " ", "_")
}

func (p *Pipeline) getIcon(category string) string {
	type configProvider interface {
		GetConfig() *config.CategoriesConfig
	}
	if cp, ok := p.classifier.(configProvider); ok {
		if cfg, ok := cp.GetConfig().Categories[strings.ToLower(category)]; ok {
			return cfg.Icon
		}
	}
	return ""
}
