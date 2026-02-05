package prompt

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"phileasgo/pkg/articleproc"
	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/wikidata"
)

type Assembler struct {
	cfg           config.Provider
	st            Store
	prompts       Renderer
	geoSvc        GeoProvider
	wikipedia     WikipediaProvider
	poiMgr        POIProvider
	llm           LLMProvider
	categoriesCfg *config.CategoriesConfig
	langRes       LanguageResolver
	density       *wikidata.DensityManager
}

func NewAssembler(
	cfg config.Provider,
	st Store,
	prompts Renderer,
	geoSvc GeoProvider,
	wikipedia WikipediaProvider,
	poiMgr POIProvider,
	llm LLMProvider,
	categoriesCfg *config.CategoriesConfig,
	langRes LanguageResolver,
	density *wikidata.DensityManager,
) *Assembler {
	return &Assembler{
		cfg:           cfg,
		st:            st,
		prompts:       prompts,
		geoSvc:        geoSvc,
		wikipedia:     wikipedia,
		poiMgr:        poiMgr,
		llm:           llm,
		categoriesCfg: categoriesCfg,
		langRes:       langRes,
		density:       density,
	}
}

func (a *Assembler) NewPromptData(session SessionState) Data {
	pd := make(Data)
	a.injectPersona(pd, session)
	return pd
}

func (a *Assembler) ForPOI(ctx context.Context, p *model.POI, tel *sim.Telemetry, strategy string, session SessionState) Data {
	pd := a.NewPromptData(session)
	a.injectTelemetry(pd, tel)
	a.injectPOI(ctx, pd, p)
	a.injectUnits(pd)

	// Custom/Specific logic for this request
	wikiInfo := a.fetchWikipediaText(ctx, p)
	pregroundText := ""
	if pt, ok := pd["PregroundContext"].(string); ok {
		pregroundText = pt
	}
	pregroundWords := len(strings.Fields(pregroundText))

	maxWords, domStrat := a.sampleNarrationLength(p, strategy, wikiInfo.WordCount+pregroundWords)

	if p == nil {
		pd["MaxWords"] = maxWords
		pd["DomStrat"] = domStrat
		pd["TTSInstructions"] = a.fetchTTSInstructions(pd)
		return pd
	}

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
	pd["ArticleURL"] = p.WPURL

	// Inject raw navigation data for template-side logic
	a.injectNavigationData(pd, p, tel)

	// Final fetch of TTS instructions with full context
	pd["TTSInstructions"] = a.fetchTTSInstructions(pd)

	return pd
}

func (a *Assembler) ForGeneric(ctx context.Context, tel *sim.Telemetry, session SessionState) Data {
	pd := a.NewPromptData(session)
	a.injectTelemetry(pd, tel)
	a.injectUnits(pd)
	pd["TTSInstructions"] = a.fetchTTSInstructions(pd)
	return pd
}

func (a *Assembler) injectTelemetry(pd Data, t *sim.Telemetry) {
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
	pd["IsOnGround"] = t.IsOnGround

	// Geographical context for aircraft position
	loc := a.geoSvc.GetLocation(t.Latitude, t.Longitude)
	pd["TargetRegion"] = fmt.Sprintf("Near %s", loc.CityName)
	pd["TargetCountry"] = loc.CountryName
}

func (a *Assembler) injectPersona(pd Data, session SessionState) {
	appCfg := a.cfg.AppConfig()
	pd["TourGuideName"] = "Ava"
	pd["Persona"] = "Intelligent, fascinating"
	pd["Accent"] = "Neutral"
	pd["Language"] = a.cfg.ActiveTargetLanguage(context.Background())
	pd["FemalePersona"] = "Intelligent, fascinating"
	pd["FemaleAccent"] = "Neutral"
	pd["PassengerMale"] = "Andrew"
	pd["MaleAccent"] = "Neutral"
	pd["TripSummary"] = a.formatTripLog(session.Events)
	pd["LastSentence"] = session.LastSentence
	pd["TargetLanguage"] = a.cfg.ActiveTargetLanguage(context.Background())
	pd["MaxWords"] = appCfg.Narrator.NarrationLengthLongWords

	// Dynamic Style & Trip Theme
	pd["ActiveStyle"] = a.cfg.ActiveStyle(context.Background())
	pd["ActiveSecretWord"] = a.cfg.ActiveSecretWord(context.Background())
	// Config values for template context
	pd["TemperatureBase"] = appCfg.Narrator.TemperatureBase
	pd["TemperatureJitter"] = appCfg.Narrator.TemperatureJitter
	pd["MinPOIScore"] = a.cfg.MinScoreThreshold(context.Background())
	pd["TextLengthScale"] = a.cfg.TextLengthScale(context.Background())
	pd["UnitSetting"] = a.cfg.Units(context.Background())

	targetLang := a.cfg.ActiveTargetLanguage(context.Background())
	langCode := "en"
	langName := "English"
	parts := strings.Split(targetLang, "-")
	if len(parts) >= 1 {
		langCode = parts[0]
		if len(parts) == 2 && a.langRes != nil {
			info := a.langRes.GetLanguageInfo(parts[1])
			if info.Name != "" {
				langName = info.Name
			}
		}
	}
	pd["Language_code"] = langCode
	pd["Language_name"] = langName
	pd["Language_region_code"] = targetLang
}

func (a *Assembler) formatTripLog(events []model.TripEvent) string {
	if len(events) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, e := range events {
		timeStr := e.Timestamp.Format("15:04")
		if e.Type == "narration" {
			category := string(e.Category)
			if category == "" {
				category = "POI"
			}
			sb.WriteString(fmt.Sprintf("* [%s] %s: %s - %s\n", timeStr, strings.ToUpper(category), e.Title, e.Summary))
		} else {
			sb.WriteString(fmt.Sprintf("* [%s] %s: %s\n", timeStr, strings.ToUpper(e.Type), e.Title))
		}
	}
	return sb.String()
}

func (a *Assembler) injectPOI(ctx context.Context, pd Data, p *model.POI) {
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
	loc := a.geoSvc.GetLocation(p.Lat, p.Lon)
	pd["Country"] = loc.CountryName
	pd["Region"] = loc.Admin1Name
	pd["City"] = loc.CityName

	// Content
	pd["WikipediaText"] = a.fetchWikipediaText(ctx, p).Prose
	pd["PregroundContext"] = a.fetchPregroundContext(ctx, p)
	pd["RecentContext"] = a.fetchRecentContext(ctx, p.Lat, p.Lon)
}

func (a *Assembler) injectUnits(pd Data) {
	pd["UnitsInstruction"] = a.fetchUnitsInstruction()
	pd["UnitSystem"] = strings.ToLower(a.cfg.Units(context.Background()))
}

func (a *Assembler) fetchUnitsInstruction() string {
	unitSys := strings.ToLower(a.cfg.Units(context.Background()))
	tmplName := fmt.Sprintf("units/%s.tmpl", unitSys)

	// Default to imperial if invalid/empty
	if unitSys == "" {
		tmplName = "units/imperial.tmpl"
	}

	content, err := a.prompts.Render(tmplName, nil)
	if err != nil {
		return ""
	}
	return content
}

func (a *Assembler) fetchWikipediaText(ctx context.Context, p *model.POI) *articleproc.Info {
	if p == nil || p.WikidataID == "" {
		return &articleproc.Info{}
	}
	// 1. Try Store using QID as UUID
	art, _ := a.st.GetArticle(ctx, p.WikidataID)
	if art != nil && art.Text != "" {
		wordCount := len(strings.Fields(art.Text))
		if a.density != nil && p.WPURL != "" {
			wordCount = a.density.EstimateWordCount(len(art.Text), p.WPURL)
		}
		return &articleproc.Info{
			Prose:     art.Text,
			WordCount: wordCount,
		}
	}

	// 2. Fetch if missing
	if p.WPURL == "" || strings.Contains(p.WPURL, "wikidata.org") {
		return &articleproc.Info{}
	}

	parts := strings.Split(p.WPURL, "/")
	if len(parts) < 5 {
		return &articleproc.Info{}
	}
	title := parts[len(parts)-1]
	lang := "en"
	if strings.Contains(parts[2], ".") {
		lang = strings.Split(parts[2], ".")[0]
	}

	htmlContent, err := a.wikipedia.GetArticleHTML(ctx, title, lang)
	if err != nil {
		return &articleproc.Info{}
	}

	info, err := articleproc.ExtractProse(strings.NewReader(htmlContent))
	if err != nil {
		return &articleproc.Info{}
	}

	// 3. Cache it
	_ = a.st.SaveArticle(ctx, &model.Article{
		UUID:  p.WikidataID,
		Title: title,
		URL:   p.WPURL,
		Text:  info.Prose,
	})

	return info
}

func (a *Assembler) fetchRecentContext(ctx context.Context, lat, lon float64) string {
	since := time.Now().Add(-1 * time.Hour)
	pois, err := a.st.GetRecentlyPlayedPOIs(ctx, since)
	if err != nil {
		return "None"
	}

	var contextParts []string
	p1 := geo.Point{Lat: lat, Lon: lon}
	for _, p := range pois {
		p2 := geo.Point{Lat: p.Lat, Lon: p.Lon}
		dist := geo.Distance(p1, p2)
		if dist < 50000 {
			contextParts = append(contextParts, fmt.Sprintf("%s (%s)", p.NameEn, p.Category))
		}
	}

	return strings.Join(contextParts, ", ")
}

func (a *Assembler) fetchPregroundContext(ctx context.Context, p *model.POI) string {
	if a.categoriesCfg == nil || !a.categoriesCfg.ShouldPreground(p.Category) {
		return ""
	}

	if !a.llm.HasProfile("pregrounding") {
		return ""
	}

	data := struct {
		Category string
		Country  string
		Lat      float64
		Lon      float64
	}{
		Category: p.Category,
		Country:  a.geoSvc.GetLocation(p.Lat, p.Lon).CountryName,
		Lat:      p.Lat,
		Lon:      p.Lon,
	}

	prompt, err := a.prompts.Render("context/pregrounding.tmpl", data)
	if err != nil {
		return ""
	}

	result, err := a.llm.GenerateText(ctx, "pregrounding", prompt)
	if err != nil {
		return ""
	}

	return a.cleanPregroundingResult(result)
}

func (a *Assembler) cleanPregroundingResult(s string) string {
	// Strip bracketed citations like [1], [23], etc.
	// We use a simple regex-like approach or strings replace if we want to avoid regex overhead,
	// but for this depth, a simple regex is safest.
	// However, I'll stick to a robust manual pass or a few common replacements to be safe.

	// 1. Strip markdown links [text](url) -> text
	// 2. Strip bracketed numbers [1]
	// 3. Strip parenthetical numbers (1)

	// For now, let's keep it simple and effective:
	lines := strings.Split(s, "\n")
	var cleaned []string
	for _, line := range lines {
		l := line
		// Remove bracketed citations [1]...[99]
		for i := 1; i < 20; i++ {
			l = strings.ReplaceAll(l, fmt.Sprintf("[%d]", i), "")
		}
		// Remove Markdown URLs but keep the text
		// This is a bit too complex for simple string replace, but we can do a basic pass

		cleaned = append(cleaned, strings.TrimSpace(l))
	}

	return strings.Join(cleaned, "\n")
}

func (a *Assembler) fetchTTSInstructions(data Data) string {
	var tmplName string
	// Simplified fallback logic for now
	appCfg := a.cfg.AppConfig()
	data["TTSEngine"] = appCfg.TTS.Engine
	switch strings.ToLower(appCfg.TTS.Engine) {
	case "fish-audio":
		tmplName = "tts/fish-audio.tmpl"
	case "azure", "azure-speech":
		tmplName = "tts/azure.tmpl"
	default:
		tmplName = "tts/edge-tts.tmpl"
	}

	content, err := a.prompts.Render(tmplName, data)
	if err != nil {
		return "Do not use speaker labels."
	}
	return content
}

func (a *Assembler) sampleNarrationLength(p *model.POI, strategy string, sourceWords int) (words int, strategyUsed string) {
	appCfg := a.cfg.AppConfig()
	shortTarget := appCfg.Narrator.NarrationLengthShortWords
	longTarget := appCfg.Narrator.NarrationLengthLongWords
	if shortTarget <= 0 {
		shortTarget = 50
	}
	if longTarget <= 0 {
		longTarget = 200
	}

	if strategy == "" {
		strategy = a.DetermineSkewStrategy(p, false)
	}
	baseTarget := longTarget
	if strategy == StrategyMinSkew {
		baseTarget = shortTarget
	}

	targetLimit := a.ApplyWordLengthMultiplier(baseTarget)
	sourceLimit := sourceWords / 2

	finalWords := targetLimit
	if sourceLimit < targetLimit {
		finalWords = sourceLimit
	}

	return finalWords, strategy
}

func (a *Assembler) ApplyWordLengthMultiplier(baseWords int) int {
	textLength := a.cfg.TextLengthScale(context.Background())

	multiplier := 1.0
	if textLength > 1 {
		if textLength > 5 {
			textLength = 5
		}
		multiplier = 1.0 + float64(textLength-1)*0.25
	}

	return int(float64(baseWords) * multiplier)
}

func (a *Assembler) DetermineSkewStrategy(p *model.POI, isOnGround bool) string {
	return DetermineSkewStrategy(p, a.poiMgr, isOnGround)
}

func DetermineSkewStrategy(p *model.POI, poiMgr POIProvider, isOnGround bool) string {
	if p == nil {
		return StrategyUniform
	}

	if isOnGround {
		return StrategyMaxSkew
	}

	threshold := math.Max(p.Score*0.2, 0.5)
	rivals := poiMgr.CountScoredAbove(threshold, 2)

	if rivals > 1 {
		return StrategyMinSkew
	}
	return StrategyMaxSkew
}

func (a *Assembler) injectNavigationData(pd Data, p *model.POI, tel *sim.Telemetry) {
	if p == nil || tel == nil {
		return
	}

	latSrc, lonSrc := tel.Latitude, tel.Longitude
	if tel.PredictedLatitude != 0 || tel.PredictedLongitude != 0 {
		latSrc, lonSrc = tel.PredictedLatitude, tel.PredictedLongitude
	}

	pSrc := geo.Point{Lat: latSrc, Lon: lonSrc}
	pTarget := geo.Point{Lat: p.Lat, Lon: p.Lon}

	distMeters := geo.Distance(pSrc, pTarget)
	bearing := geo.Bearing(pSrc, pTarget)
	normBearing := math.Mod(bearing+360, 360)
	relBearing := math.Mod(bearing-tel.Heading+360, 360)

	pd["DistMeters"] = distMeters
	pd["DistKm"] = a.humanRound(distMeters / 1000.0)
	pd["DistNm"] = a.humanRound(distMeters * 0.000539957)
	pd["Bearing"] = normBearing
	pd["RelBearing"] = relBearing
	pd["ClockPos"] = a.calculateClockPos(relBearing)
	pd["CardinalDir"] = a.calculateCardinalDir(normBearing)
	pd["RelativeDir"] = a.calculateRelativeDir(relBearing)
	pd["Movement"] = a.calculateMovement(relBearing)
}

func (a *Assembler) calculateClockPos(relBearing float64) int {
	clock := int((relBearing + 15) / 30)
	if clock == 0 {
		return 12
	}
	return clock
}

func (a *Assembler) calculateCardinalDir(normBearing float64) string {
	dirs := []string{"North", "North-East", "East", "South-East", "South", "South-West", "West", "North-West"}
	idx := int((normBearing+22.5)/45.0) % 8
	return dirs[idx]
}

func (a *Assembler) calculateRelativeDir(relBearing float64) string {
	switch {
	case relBearing >= 345 || relBearing <= 15:
		return "ahead"
	case relBearing > 15 && relBearing <= 135:
		return "right"
	case relBearing > 135 && relBearing <= 225:
		return "behind"
	default:
		return "left"
	}
}

func (a *Assembler) calculateMovement(relBearing float64) string {
	switch {
	case relBearing > 45 && relBearing <= 135:
		return "passing"
	case relBearing > 135 && relBearing <= 225:
		return "beyond"
	case relBearing > 225 && relBearing < 315:
		return "passing"
	default:
		return "approaching"
	}
}

func (a *Assembler) humanRound(val float64) float64 {
	if val < 10 {
		return math.Round(val)
	}
	if val < 100 {
		return math.Round(val/5.0) * 5.0
	}
	return math.Round(val/10.0) * 10.0
}
