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
	if tel == nil {
		t, _ := s.sim.GetTelemetry(ctx)
		tel = &t
	}

	pd := s.getCommonPromptData()
	s.injectTelemetry(pd, tel)
	s.injectPOI(ctx, pd, p)
	s.injectUnits(pd)

	// Custom/Specific logic for this request
	wikiInfo := s.fetchWikipediaText(ctx, p)
	pregroundText := pd["PregroundContext"].(string)
	pregroundWords := len(strings.Fields(pregroundText))

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

	pd["MaxWords"] = maxWords
	pd["DomStrat"] = domStrat
	pd["IsStub"] = isStub
	pd["NavInstruction"] = s.calculateNavInstruction(p, tel)
	pd["ArticleURL"] = p.WPURL

	// Final fetch of TTS instructions with full context
	pd["TTSInstructions"] = s.fetchTTSInstructions(pd)

	return pd
}

func (s *AIService) fetchTTSInstructions(data NarrationPromptData) string {
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

// NarrationPromptData is a dynamic map for template context.
type NarrationPromptData map[string]any

// Set adds or updates a field in the prompt data.
func (pd NarrationPromptData) Set(key string, val any) {
	pd[key] = val
}

// Get returns a field from the prompt data.
func (pd NarrationPromptData) Get(key string) any {
	return pd[key]
}

func (s *AIService) injectTelemetry(pd NarrationPromptData, t *sim.Telemetry) {
	if t == nil {
		return
	}
	pd["Lat"] = t.Latitude
	pd["Lon"] = t.Longitude
	pd["AltitudeMSL"] = t.AltitudeMSL
	pd["AltitudeAGL"] = t.AltitudeAGL
	pd["Heading"] = t.Heading
	pd["GroundSpeed"] = t.GroundSpeed
	pd["PredictedLat"] = t.PredictedLatitude
	pd["PredictedLon"] = t.PredictedLongitude
	pd["FlightStage"] = sim.FormatStage(t.FlightStage)
	pd["FlightStatusSentence"] = generateFlightStatusSentence(t)

	// Geographical context for aircraft position
	loc := s.geoSvc.GetLocation(t.Latitude, t.Longitude)
	pd["TargetRegion"] = fmt.Sprintf("Near %s", loc.CityName)
	pd["TargetCountry"] = loc.CountryCode
}

func (s *AIService) injectPersona(pd NarrationPromptData) {
	pd["TourGuideName"] = "Ava"
	pd["Persona"] = "Intelligent, fascinating"
	pd["Accent"] = "Neutral"
	pd["Language"] = s.cfg.Narrator.TargetLanguage
	pd["FemalePersona"] = "Intelligent, fascinating"
	pd["FemaleAccent"] = "Neutral"
	pd["PassengerMale"] = "Andrew"
	pd["MalePersona"] = "Curious traveler"
	pd["MaleAccent"] = "Neutral"
	pd["TripSummary"] = s.getTripSummary()
	pd["LastSentence"] = s.lastScriptEnd
	pd["TargetLanguage"] = s.cfg.Narrator.TargetLanguage
	pd["MaxWords"] = s.cfg.Narrator.NarrationLengthLongWords

	// Language decoding logic if needed by templates
	targetLang := s.cfg.Narrator.TargetLanguage
	langCode := "en"
	langName := "English"
	parts := strings.Split(targetLang, "-")
	if len(parts) >= 1 {
		langCode = parts[0]
		if len(parts) == 2 && s.langRes != nil {
			info := s.langRes.GetLanguageInfo(parts[1])
			if info.Name != "" {
				langName = info.Name
			}
		}
	}
	pd["Language_code"] = langCode
	pd["Language_name"] = langName
	pd["Language_region_code"] = targetLang

	pd["TTSInstructions"] = s.fetchTTSInstructions(pd)
}

func (s *AIService) injectPOI(ctx context.Context, pd NarrationPromptData, p *model.POI) {
	if p == nil {
		return
	}
	pd["POINameNative"] = p.NameEn // Use En as fallback if native missing
	if p.NameLocal != "" {
		pd["POINameNative"] = p.NameLocal
	}
	pd["POINameUser"] = p.DisplayName()
	pd["Category"] = p.Category
	pd["Lat"] = p.Lat
	pd["Lon"] = p.Lon

	// Location info
	loc := s.geoSvc.GetLocation(p.Lat, p.Lon)
	pd["Country"] = loc.CountryCode
	pd["Region"] = loc.Admin1Name
	pd["City"] = loc.CityName

	// Content
	pd["WikipediaText"] = s.fetchWikipediaText(ctx, p).Prose
	pd["PregroundContext"] = s.fetchPregroundContext(ctx, p)
	pd["RecentContext"] = s.fetchRecentContext(ctx, p.Lat, p.Lon)
}

func (s *AIService) injectUnits(pd NarrationPromptData) {
	pd["UnitsInstruction"] = s.fetchUnitsInstruction()
}

// ImageResult represents a candidate image for selection.
type ImageResult struct {
	Title string
	URL   string
}

// getCommonPromptData returns a baseline NarrationPromptData with language, persona, and trip context.
func (s *AIService) getCommonPromptData() NarrationPromptData {
	pd := make(NarrationPromptData)
	s.injectPersona(pd)
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
