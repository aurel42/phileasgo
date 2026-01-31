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
)

type Assembler struct {
	cfg           *config.Config
	st            Store
	prompts       Renderer
	geoSvc        GeoProvider
	wikipedia     WikipediaProvider
	poiMgr        POIProvider
	llm           LLMProvider
	categoriesCfg *config.CategoriesConfig
	langRes       LanguageResolver
}

func NewAssembler(
	cfg *config.Config,
	st Store,
	prompts Renderer,
	geoSvc GeoProvider,
	wikipedia WikipediaProvider,
	poiMgr POIProvider,
	llm LLMProvider,
	categoriesCfg *config.CategoriesConfig,
	langRes LanguageResolver,
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
	pd["NavInstruction"] = a.calculateNavInstruction(p, tel)
	pd["ArticleURL"] = p.WPURL

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
	pd["FlightStatusSentence"] = GenerateFlightStatusSentence(t)

	// Geographical context for aircraft position
	loc := a.geoSvc.GetLocation(t.Latitude, t.Longitude)
	pd["TargetRegion"] = fmt.Sprintf("Near %s", loc.CityName)
	pd["TargetCountry"] = loc.CountryCode
}

func (a *Assembler) GenerateFlightStatusSentence(t *sim.Telemetry) string {
	return GenerateFlightStatusSentence(t)
}

func GenerateFlightStatusSentence(t *sim.Telemetry) string {
	if t == nil {
		return "The aircraft position is unknown."
	}

	lat := fmt.Sprintf("%.4f", t.PredictedLatitude)
	lon := fmt.Sprintf("%.4f", t.PredictedLongitude)

	if t.IsOnGround {
		action := "sitting"
		if t.GroundSpeed >= 2.0 {
			action = "taxiing"
		}
		return fmt.Sprintf("The aircraft is %s on the ground. Its position is %s, %s.", action, lat, lon)
	}

	// Flying
	alt := t.AltitudeAGL
	var altStr string
	if alt < 1000 {
		rounded := math.Round(alt/100) * 100
		altStr = fmt.Sprintf("%.0f", rounded)
	} else {
		rounded := math.Round(alt/1000) * 1000
		altStr = fmt.Sprintf("%.0f", rounded)
	}

	speed := fmt.Sprintf("%.0f", t.GroundSpeed)
	hdg := fmt.Sprintf("%.0f", t.Heading)

	return fmt.Sprintf("The aircraft is cruising about %s ft over the ground, moving at %s knots in heading %s. Its position is %s, %s.",
		altStr, speed, hdg, lat, lon)
}

func (a *Assembler) injectPersona(pd Data, session SessionState) {
	pd["TourGuideName"] = "Ava"
	pd["Persona"] = "Intelligent, fascinating"
	pd["Accent"] = "Neutral"
	pd["Language"] = a.cfg.Narrator.TargetLanguage
	pd["FemalePersona"] = "Intelligent, fascinating"
	pd["FemaleAccent"] = "Neutral"
	pd["PassengerMale"] = "Andrew"
	pd["MalePersona"] = "Curious traveler"
	pd["MaleAccent"] = "Neutral"
	pd["TripSummary"] = session.TripSummary
	pd["LastSentence"] = session.LastSentence
	pd["TargetLanguage"] = a.cfg.Narrator.TargetLanguage
	pd["MaxWords"] = a.cfg.Narrator.NarrationLengthLongWords

	// Language decoding logic if needed by templates
	targetLang := a.cfg.Narrator.TargetLanguage
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
	pd["Country"] = loc.CountryCode
	pd["Region"] = loc.Admin1Name
	pd["City"] = loc.CityName

	// Content
	pd["WikipediaText"] = a.fetchWikipediaText(ctx, p).Prose
	pd["PregroundContext"] = a.fetchPregroundContext(ctx, p)
	pd["RecentContext"] = a.fetchRecentContext(ctx, p.Lat, p.Lon)
}

func (a *Assembler) injectUnits(pd Data) {
	pd["UnitsInstruction"] = a.fetchUnitsInstruction()
}

func (a *Assembler) fetchUnitsInstruction() string {
	unitSys := strings.ToLower(a.cfg.Narrator.Units)
	if unitSys == "metric" {
		return "Use metric units (meters, kilometers) for all measurements."
	}
	if unitSys == "hybrid" {
		return "Use hybrid units (kilometers for distances, feet for altitudes)."
	}
	return "Use imperial units (miles, feet) for all measurements."
}

func (a *Assembler) fetchWikipediaText(ctx context.Context, p *model.POI) *articleproc.Info {
	if p == nil || p.WikidataID == "" {
		return &articleproc.Info{}
	}
	// 1. Try Store using QID as UUID
	art, _ := a.st.GetArticle(ctx, p.WikidataID)
	if art != nil && art.Text != "" {
		return &articleproc.Info{
			Prose:     art.Text,
			WordCount: len(strings.Fields(art.Text)),
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

	if len(contextParts) == 0 {
		return "None"
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
		Name     string
		Category string
		Country  string
		Lat      float64
		Lon      float64
	}{
		Name:     p.DisplayName(),
		Category: p.Category,
		Country:  a.geoSvc.GetLocation(p.Lat, p.Lon).CountryCode,
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

	return result
}

func (a *Assembler) fetchTTSInstructions(data Data) string {
	var tmplName string
	// Simplified fallback logic for now
	switch strings.ToLower(a.cfg.TTS.Engine) {
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
	shortTarget := a.cfg.Narrator.NarrationLengthShortWords
	longTarget := a.cfg.Narrator.NarrationLengthLongWords
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
	textLengthVal, _ := a.st.GetState(context.Background(), "text_length")
	textLength := 1
	if textLengthVal != "" {
		_, _ = fmt.Sscanf(textLengthVal, "%d", &textLength)
	}

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

func (a *Assembler) calculateNavInstruction(p *model.POI, tel *sim.Telemetry) string {
	latSrc, lonSrc := tel.Latitude, tel.Longitude
	if tel.PredictedLatitude != 0 || tel.PredictedLongitude != 0 {
		latSrc, lonSrc = tel.PredictedLatitude, tel.PredictedLongitude
	}

	pSrc := geo.Point{Lat: latSrc, Lon: lonSrc}
	pTarget := geo.Point{Lat: p.Lat, Lon: p.Lon}

	distMeters := geo.Distance(pSrc, pTarget)
	distKm := distMeters / 1000.0

	if distKm < 4.5 {
		if tel.IsOnGround {
			return ""
		}
		return a.formatAirborneRelative(pSrc, pTarget, tel.Heading)
	}

	var distStr string
	unitSys := strings.ToLower(a.cfg.Narrator.Units)

	if unitSys == "metric" || unitSys == "hybrid" {
		val := a.humanRound(distKm)
		distStr = fmt.Sprintf("about %.0f kilometers", val)
	} else {
		distNm := distMeters * 0.000539957
		val := a.humanRound(distNm)
		distStr = fmt.Sprintf("about %.0f miles", val)
	}

	if tel.IsOnGround {
		return a.formatGroundCardinal(pSrc, pTarget, distStr)
	}
	return a.formatAirborneClock(pSrc, pTarget, tel.Heading, distStr)
}

func (a *Assembler) formatGroundCardinal(pSrc, pTarget geo.Point, distStr string) string {
	bearing := geo.Bearing(pSrc, pTarget)
	normBearing := math.Mod(bearing+360, 360)
	dirs := []string{"North", "North-East", "East", "South-East", "South", "South-West", "West", "North-West"}
	idx := int((normBearing+22.5)/45.0) % 8
	direction := fmt.Sprintf("to the %s", dirs[idx])

	return a.capitalizeStart(fmt.Sprintf("%s, %s away", direction, distStr))
}

func (a *Assembler) formatAirborneRelative(pSrc, pTarget geo.Point, userHdg float64) string {
	bearing := geo.Bearing(pSrc, pTarget)
	relBearing := math.Mod(bearing-userHdg+360, 360)

	var direction string
	switch {
	case relBearing >= 345 || relBearing <= 15:
		direction = "straight ahead"
	case relBearing > 15 && relBearing <= 135:
		direction = "on your right"
	case relBearing > 135 && relBearing <= 225:
		direction = "behind you"
	case relBearing > 225 && relBearing < 345:
		direction = "on your left"
	}

	return a.capitalizeStart(direction)
}

func (a *Assembler) formatAirborneClock(pSrc, pTarget geo.Point, userHdg float64, distStr string) string {
	bearing := geo.Bearing(pSrc, pTarget)
	relBearing := math.Mod(bearing-userHdg+360, 360)

	clock := int((relBearing + 15) / 30)
	if clock == 0 {
		clock = 12
	}
	direction := fmt.Sprintf("at your %d o'clock", clock)

	return a.capitalizeStart(fmt.Sprintf("%s, %s away", direction, distStr))
}

func (a *Assembler) capitalizeStart(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
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
