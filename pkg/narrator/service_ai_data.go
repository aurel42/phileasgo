package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"phileasgo/pkg/articleproc"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

func (s *AIService) buildPromptData(ctx context.Context, p *model.POI, tel *sim.Telemetry, strategy string) NarrationPromptData {
	// CC & Lang
	loc := s.geoSvc.GetLocation(p.Lat, p.Lon)
	cc := loc.CountryCode
	region := loc.CityName
	if loc.CityName != "Unknown" {
		region = "Near " + loc.CityName
	}

	if tel == nil {
		t, _ := s.sim.GetTelemetry(ctx)
		tel = &t
	}
	nav := s.calculateNavInstruction(p, tel)

	// Stub Detection & Text Fetch
	wikiInfo := s.fetchWikipediaText(ctx, p)
	wpText := wikiInfo.Prose

	// Phase 3: Pregrounding context - fetch early to influence word count
	pregroundText := s.fetchPregroundContext(ctx, p)
	pregroundWords := len(strings.Fields(pregroundText))

	// Include pregrounding context in source depth for scaling
	maxWords, domStrat := s.sampleNarrationLength(p, strategy, wikiInfo.WordCount+pregroundWords)

	isStub := false
	for _, b := range p.Badges {
		if b == "stub" {
			isStub = true
			break
		}
	}
	if isStub {
		maxWords = 0
	}

	p.NarrationStrategy = domStrat

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
		WikipediaText:        wpText,
		ArticleURL:           p.WPURL,
		IsStub:               isStub,
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
		Avoid:                s.avoid,
		AltitudeMSL:          tel.AltitudeMSL,
		AltitudeAGL:          tel.AltitudeAGL,
		Heading:              tel.Heading,
		GroundSpeed:          tel.GroundSpeed,
		PredictedLat:         tel.PredictedLatitude,
		PredictedLon:         tel.PredictedLongitude,
		TripSummary:          s.getTripSummary(),
		LastSentence:         s.lastScriptEnd,
		FlightStatusSentence: generateFlightStatusSentence(tel),
		PregroundContext:     pregroundText,
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

func (s *AIService) fetchWikipediaText(ctx context.Context, p *model.POI) *articleproc.Info {
	// 1. Try Store using QID as UUID
	art, _ := s.st.GetArticle(ctx, p.WikidataID)
	if art != nil && art.Text != "" {
		// Calculate word count for cached articles since old ones might not have it in DB
		// (though we usually count them at extraction time)
		return &articleproc.Info{
			Prose:     art.Text,
			WordCount: len(strings.Fields(art.Text)),
		}
	}

	// 2. Fetch if missing
	if p.WPURL == "" {
		return &articleproc.Info{}
	}
	// Safeguard: If URL is still pointing to Wikidata (failed rescue), do not attempt fetch
	if strings.Contains(p.WPURL, "wikidata.org") {
		return &articleproc.Info{}
	}

	// Parse Title/Lang from URL: https://en.wikipedia.org/wiki/Title
	parts := strings.Split(p.WPURL, "/")
	if len(parts) < 5 {
		return &articleproc.Info{}
	}
	title := parts[len(parts)-1]
	lang := "en"
	if strings.Contains(parts[2], ".") {
		lang = strings.Split(parts[2], ".")[0]
	}

	// PHASE 2: Use HTML parsing for clean prose
	htmlContent, err := s.wikipedia.GetArticleHTML(ctx, title, lang)
	if err != nil {
		slog.Warn("Narrator: Failed to fetch Wikipedia HTML", "title", title, "error", err)
		return &articleproc.Info{}
	}

	info, err := articleproc.ExtractProse(strings.NewReader(htmlContent))
	if err != nil {
		slog.Warn("Narrator: Failed to extract prose from Wikipedia HTML", "title", title, "error", err)
		return &articleproc.Info{}
	}

	// 3. Cache it (We store the CLEAN prose now)
	_ = s.st.SaveArticle(ctx, &model.Article{
		UUID:  p.WikidataID,
		Title: title,
		URL:   p.WPURL,
		Text:  info.Prose,
	})

	return info
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

// fetchPregroundContext fetches enriched local context from Sonar (Perplexity) for POIs
// in categories marked with preground: true. Returns empty string if disabled, unavailable,
// or on error (graceful degradation).
func (s *AIService) fetchPregroundContext(ctx context.Context, p *model.POI) string {
	// 1. Check if category has pregrounding enabled
	if s.categoriesCfg == nil || !s.categoriesCfg.ShouldPreground(p.Category) {
		return ""
	}

	// 2. Check if LLM provider has pregrounding profile
	if !s.llm.HasProfile("pregrounding") {
		return ""
	}

	// 3. Render the pregrounding prompt with POI context
	data := struct {
		Name     string
		Category string
		Country  string
		Lat      float64
		Lon      float64
	}{
		Name:     p.DisplayName(),
		Category: p.Category,
		Country:  s.geoSvc.GetLocation(p.Lat, p.Lon).CountryCode,
		Lat:      p.Lat,
		Lon:      p.Lon,
	}

	prompt, err := s.prompts.Render("context/pregrounding.tmpl", data)
	if err != nil {
		slog.Debug("Narrator: Failed to render pregrounding prompt", "error", err)
		return ""
	}

	// 4. Call Sonar via the pregrounding profile
	result, err := s.llm.GenerateText(ctx, "pregrounding", prompt)
	if err != nil {
		slog.Debug("Narrator: Pregrounding call failed", "poi", p.WikidataID, "error", err)
		return ""
	}

	slog.Info("Narrator: Pregrounded POI context", "poi", p.WikidataID, "chars", len(result))
	return result
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
	Avoid                []string
	AltitudeMSL          float64
	AltitudeAGL          float64
	Heading              float64
	GroundSpeed          float64
	PredictedLat         float64
	PredictedLon         float64
	DominanceStrategy    string
	TripSummary          string
	LastSentence         string
	FlightStatusSentence string

	// Narrator Logic
	PregroundContext string // Sonar-enriched local context (empty if disabled or N/A)
	IsStub           bool

	// Generic fields for flexibility in common macros
	Title  string
	Script string
	From   string
	To     string

	// Specific fields for summary updates
	CurrentSummary string
	LastTitle      string
	LastScript     string

	// Specific fields for other templates (Thumbnail, Wikidata, Essay, Screenshot)
	Name             string
	ArticleURL       string
	Images           []ImageResult
	CategoryList     string
	TopicName        string
	TopicDescription string
	City             string
	Alt              string
}

// ImageResult represents a candidate image for selection.
type ImageResult struct {
	Title string
	URL   string
}

// getCommonPromptData returns a baseline NarrationPromptData with language, persona, and trip context.
func (s *AIService) getCommonPromptData() NarrationPromptData {
	s.mu.RLock()
	tripSummary := s.tripSummary
	lastSentence := s.lastScriptEnd
	s.mu.RUnlock()

	// Language Logic (User's Target Language)
	targetLang := s.cfg.Narrator.TargetLanguage
	langCode := "en"
	langName := "English"
	langLocale := targetLang

	parts := strings.Split(targetLang, "-")
	if len(parts) == 2 {
		targetCC := parts[1]
		if s.langRes != nil {
			info := s.langRes.GetLanguageInfo(targetCC)
			langCode = info.Code
			langName = info.Name
		} else {
			langCode = parts[0]
		}
	} else if len(parts) > 0 {
		langCode = parts[0]
	}

	pd := NarrationPromptData{
		TourGuideName:        "Ava",
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
		TripSummary:          tripSummary,
		LastSentence:         lastSentence,
		TargetLanguage:       targetLang,
		MaxWords:             s.cfg.Narrator.NarrationLengthLongWords,
	}

	// Fetch TTS instructions with this context
	pd.TTSInstructions = s.fetchTTSInstructions(&pd)

	return pd
}

func (s *AIService) sampleNarrationLength(p *model.POI, strategy string, sourceWords int) (words int, strategyUsed string) {
	// 1. Base targets from config
	shortTarget := s.cfg.Narrator.NarrationLengthShortWords
	longTarget := s.cfg.Narrator.NarrationLengthLongWords
	if shortTarget <= 0 {
		shortTarget = 50
	}
	if longTarget <= 0 {
		longTarget = 200
	}

	// 2. Determine Strategy and Base Target
	if strategy == "" {
		strategy = DetermineSkewStrategy(p, s.poiMgr, false)
	}
	baseTarget := longTarget
	if strategy == StrategyMinSkew {
		baseTarget = shortTarget
	}

	// 3. Calculate Upper Limit (Config * Multiplier)
	targetLimit := s.applyWordLengthMultiplier(baseTarget)

	// 4. Calculate Source Limit (Depth / 2)
	sourceLimit := sourceWords / 2

	// 5. Final Result: min(sourceLimit, targetLimit)
	finalWords := targetLimit
	scalingLog := "none (honoring target)"
	if sourceLimit < targetLimit {
		finalWords = sourceLimit
		scalingLog = fmt.Sprintf("capped by source (%d / 2 = %d)", sourceWords, sourceLimit)
	}

	slog.Debug("Narrator: Sampling Length",
		"strategy", strategy,
		"source_words", sourceWords,
		"target_limit", targetLimit,
		"final_words", finalWords,
		"scaling", scalingLog,
	)

	return finalWords, strategy
}

// applyWordLengthMultiplier applies the user's text length setting (1..5) to the base word count.
// 1 -> 1.0x (Shortest)
// 2 -> 1.25x
// 3 -> 1.50x
// 4 -> 1.75x
// 5 -> 2.00x (Longest)
func (s *AIService) applyWordLengthMultiplier(baseWords int) int {
	// 1. Get User Preference for Text Length (1..5)
	if s.st == nil {
		return baseWords
	}

	// We read directly from store to get the latest value
	// Default to 1 (Shortest i.e. x1.0) if not set
	textLengthVal, _ := s.st.GetState(context.Background(), "text_length")
	textLength := 1
	if textLengthVal != "" {
		_, _ = fmt.Sscanf(textLengthVal, "%d", &textLength)
	}

	// 2. Calculate Multiplier
	multiplier := 1.0
	if textLength > 1 {
		// Clamp to 5 max just in case
		if textLength > 5 {
			textLength = 5
		}
		multiplier = 1.0 + float64(textLength-1)*0.25
	}

	return int(float64(baseWords) * multiplier)
}
