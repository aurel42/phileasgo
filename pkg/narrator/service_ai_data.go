package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

func (s *AIService) buildPromptData(ctx context.Context, p *model.POI, tel *sim.Telemetry) NarrationPromptData {
	// CC & Lang
	loc := s.geoSvc.GetLocation(p.Lat, p.Lon)
	cc := loc.CountryCode
	region := loc.CityName
	if loc.CityName != "Unknown" {
		region = "Near " + loc.CityName
	}

	// Navigation Instruction
	if tel == nil {
		t, _ := s.sim.GetTelemetry(ctx)
		tel = &t
	}
	nav := s.calculateNavInstruction(p, tel)
	maxWords, domStrat := s.sampleNarrationLength(p)

	// Language Logic (User's Target Language)
	targetLang := s.cfg.Narrator.TargetLanguage
	langCode := "en"
	langName := "English"
	langLocale := targetLang

	// Parse "de-DE" -> "DE"
	parts := strings.Split(targetLang, "-")
	if len(parts) == 2 {
		// Valid locale format
		targetCC := parts[1]
		if s.langRes != nil {
			info := s.langRes.GetLanguageInfo(targetCC)
			langCode = info.Code
			langName = info.Name
		} else {
			// Fallback if resolver missing (unlikely in prod)
			langCode = parts[0]
		}
	} else if len(parts) > 0 {
		// Fallback for non-standard config (though validation should catch this)
		langCode = parts[0]
	}

	pd := NarrationPromptData{
		TourGuideName:        "Ava", // TODO: Get from voice profile
		Persona:              "Intelligent, fascinating",
		Accent:               "Neutral",
		Language:             targetLang,
		Language_code:        langCode,
		Language_name:        langName,
		Language_region_code: langLocale,
		FemalePersona:        "Intelligent, fascinating",
		FemaleAccent:         "Neutral",
		PassengerMale:        "Andrew",
		MalePersona:          "Curious traveler",
		MaleAccent:           "Neutral",
		FlightStage:          determineFlightStage(tel),
		NameNative:           p.NameLocal,
		POINameNative:        p.NameLocal,
		NameUser:             p.DisplayName(),
		POINameUser:          p.DisplayName(),
		Category:             p.Category,
		WikipediaText:        s.fetchWikipediaText(ctx, p),
		NavInstruction:       nav,
		TargetLanguage:       s.cfg.Narrator.TargetLanguage,
		TargetCountry:        cc,
		Country:              cc,
		TargetRegion:         region,
		Region:               region,
		MaxWords:             maxWords,
		DominanceStrategy:    domStrat,
		RecentPoisContext:    s.fetchRecentContext(ctx, p.Lat, p.Lon),
		RecentContext:        s.fetchRecentContext(ctx, p.Lat, p.Lon),
		Lat:                  tel.Latitude,
		Lon:                  tel.Longitude,
		UnitsInstruction:     s.fetchUnitsInstruction(),
		Interests:            s.interests,
		AltitudeMSL:          tel.AltitudeMSL,
		AltitudeAGL:          tel.AltitudeAGL,
		Heading:              tel.Heading,
		GroundSpeed:          tel.GroundSpeed,
		PredictedLat:         tel.PredictedLatitude,
		PredictedLon:         tel.PredictedLongitude,
	}
	// Fetch TTS instructions with full context
	pd.TTSInstructions = s.fetchTTSInstructions(&pd)

	return pd
}

func (s *AIService) fetchTTSInstructions(data any) string {
	var tmplName string

	// If fallback TTS is active, always use edge-tts template
	if s.isUsingFallbackTTS() {
		tmplName = "tts/edge-tts.tmpl"
	} else {
		// engines: sapi, windows-sapi, edge, edge-tts, fish-audio
		switch strings.ToLower(s.cfg.TTS.Engine) {
		case "fish-audio":
			tmplName = "tts/fish-audio.tmpl"
		case "azure", "azure-speech":
			tmplName = "tts/azure.tmpl"
		default:
			// Default to edge-tts for clean output (no speaker labels) which is good for most
			tmplName = "tts/edge-tts.tmpl"
		}
	}

	content, err := s.prompts.Render(tmplName, data)
	if err != nil {
		// Fallback if template missing
		slog.Warn("Narrator: Failed to render TTS template, using fallback", "template", tmplName, "error", err)
		return "Do not use speaker labels."
	}
	return content
}

func (s *AIService) fetchWikipediaText(ctx context.Context, p *model.POI) string {
	// 1. Try Store using QID as UUID
	art, _ := s.st.GetArticle(ctx, p.WikidataID)
	if art != nil && art.Text != "" {
		return art.Text
	}

	// 2. Fetch if missing
	if p.WPURL == "" {
		return ""
	}
	// Safeguard: If URL is still pointing to Wikidata (failed rescue), do not attempt fetch
	if strings.Contains(p.WPURL, "wikidata.org") {
		return ""
	}

	// Parse Title/Lang from URL: https://en.wikipedia.org/wiki/Title
	parts := strings.Split(p.WPURL, "/")
	if len(parts) < 5 {
		return ""
	}
	title := parts[len(parts)-1]
	lang := "en"
	if strings.Contains(parts[2], ".") {
		lang = strings.Split(parts[2], ".")[0]
	}

	text, err := s.wikipedia.GetArticleContent(ctx, title, lang)
	if err != nil {
		slog.Warn("Narrator: Failed to fetch Wikipedia extract", "title", title, "error", err)
		return ""
	}

	// 3. Cache it
	_ = s.st.SaveArticle(ctx, &model.Article{
		UUID:  p.WikidataID,
		Title: title,
		URL:   p.WPURL,
		Text:  text,
	})

	return text
}

func (s *AIService) fetchRecentContext(ctx context.Context, lat, lon float64) string {
	since := time.Now().Add(-1 * time.Hour)
	pois, err := s.st.GetRecentlyPlayedPOIs(ctx, since)
	if err != nil {
		slog.Warn("Narrator: Failed to fetch recent POIs for context", "error", err)
		return "None"
	}

	var contextParts []string
	p1 := geo.Point{Lat: lat, Lon: lon}
	for _, p := range pois {
		// Filter by distance (50km) in Go
		p2 := geo.Point{Lat: p.Lat, Lon: p.Lon}
		dist := geo.Distance(p1, p2)
		if dist < 50000 {
			contextParts = append(contextParts, fmt.Sprintf("%s (%s)", p.NameEn, p.Category))
		}
	}

	if len(contextParts) == 0 {
		return "None"
	}

	return strings.Join(contextParts, ", ")
}

// NarrationPromptData struct for templates
type NarrationPromptData struct {
	TourGuideName        string
	Persona              string
	Accent               string
	Language             string
	Language_code        string
	Language_name        string
	Language_region_code string
	FemalePersona        string
	FemaleAccent         string
	PassengerMale        string
	MalePersona          string
	MaleAccent           string
	FlightStage          string
	NameNative           string
	POINameNative        string
	NameUser             string
	POINameUser          string
	Category             string
	WikipediaText        string
	NavInstruction       string
	TargetLanguage       string
	TargetCountry        string
	Country              string
	TargetRegion         string
	Region               string
	Lat                  float64
	Lon                  float64
	MaxWords             int
	RecentPoisContext    string
	RecentContext        string
	UnitsInstruction     string
	TTSInstructions      string
	Interests            []string
	AltitudeMSL          float64
	AltitudeAGL          float64
	Heading              float64
	GroundSpeed          float64
	PredictedLat         float64
	PredictedLon         float64
	DominanceStrategy    string
}

func (s *AIService) sampleNarrationLength(p *model.POI) (words int, strategy string) {
	minL := s.cfg.Narrator.NarrationLengthMin
	maxL := s.cfg.Narrator.NarrationLengthMax
	if minL == 0 {
		minL = 400
	}
	if maxL == 0 {
		maxL = 600
	}
	if maxL <= minL {
		return minL, "fixed"
	}

	// Dynamic Length Logic: Relative Dominance
	// "Rivals" are other POIs with > 50% of the winner's score.
	// Note: CountScoredAbove includes the winner itself if score > 0.
	threshold := 0.0
	if p != nil {
		threshold = p.Score * 0.5
	}

	// We only need to know if there are > 1 rivals (so limit=2 is sufficient to know if count >= 2)
	rivals := s.poiMgr.CountScoredAbove(threshold, 2)

	// Default Strategy: Uniform Random
	strategy = "uniform"

	// If rivals > 1 (Winner + at least 1 other) -> Skew Short (High Competition)
	if rivals > 1 {
		strategy = "min_skew"
	} else if rivals <= 1 {
		// Winner is alone -> Skew Long (Lone Wolf)
		strategy = "max_skew"
	}

	slog.Debug("Narrator: Sampling Length", "strategy", strategy, "rivals", rivals)

	// Helper to get a random value in range
	randomVal := func() int {
		steps := (maxL - minL) / 10
		step := rand.Intn(steps + 1)
		return minL + step*10
	}

	// Pool Selection
	poolSize := 3
	pool := make([]int, poolSize)
	for i := 0; i < poolSize; i++ {
		pool[i] = randomVal()
	}

	var result int
	switch strategy {
	case "min_skew":
		// Pick smallest
		minVal := pool[0]
		for _, v := range pool {
			if v < minVal {
				minVal = v
			}
		}
		result = minVal
	case "max_skew":
		// Pick largest
		maxVal := pool[0]
		for _, v := range pool {
			if v > maxVal {
				maxVal = v
			}
		}
		result = maxVal
	default:
		// Pick first
		result = pool[0]
	}
	return result, strategy
}
