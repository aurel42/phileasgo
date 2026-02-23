package core

import (
	"context"
	"log/slog"
	"math"
	"strings"
	"time"

	"phileasgo/pkg/classifier"
	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"phileasgo/pkg/wikidata"
)

// RegionalCategoriesJob triggers AI-suggested Wikidata categories based on location.
type RegionalCategoriesJob struct {
	BaseJob
	appCfg     config.Provider
	llm        llm.Provider
	prompts    *prompts.Manager
	validator  *wikidata.Validator
	classifier *classifier.Classifier
	geo        *geo.Service
	wikiSvc    *wikidata.Service
	store      store.RegionalCategoriesStore

	lastMajorPos geo.Point
	lastRunTime  time.Time
	firstRun     bool
}

func NewRegionalCategoriesJob(
	appCfg config.Provider,
	llmProv llm.Provider,
	prompts *prompts.Manager,
	validator *wikidata.Validator,
	classifier *classifier.Classifier,
	geoSvc *geo.Service,
	wikiSvc *wikidata.Service,
	store store.RegionalCategoriesStore,
) *RegionalCategoriesJob {
	return &RegionalCategoriesJob{
		BaseJob:    NewBaseJob("Regional Categories"),
		appCfg:     appCfg,
		llm:        llmProv,
		prompts:    prompts,
		validator:  validator,
		classifier: classifier,
		geo:        geoSvc,
		wikiSvc:    wikiSvc,
		store:      store,
		firstRun:   true,
	}
}

func (j *RegionalCategoriesJob) ShouldFire(t *sim.Telemetry) bool {
	// 0. Disable if no provider supports the basic ontological profile
	if !j.llm.HasProfile("regional_categories_ontological") {
		return false
	}

	if j.TryLock() {
		j.Unlock() // We check lock in ShouldFire to avoid overlapping triggers
	} else {
		return false
	}

	currPos := geo.Point{Lat: t.Latitude, Lon: t.Longitude}

	if j.firstRun {
		return true
	}

	// Trigger after both >50nm AND >30 minutes
	dist := geo.Distance(j.lastMajorPos, currPos)
	distNM := dist / 1852.0

	if distNM >= 50 && time.Since(j.lastRunTime) >= 30*time.Minute {
		return true
	}

	return false
}

func (j *RegionalCategoriesJob) Run(ctx context.Context, t *sim.Telemetry) {
	if !j.TryLock() {
		return
	}
	// Note: We DO NOT defer Unlock() here. The goroutine is responsible for unlocking.

	j.lastMajorPos = geo.Point{Lat: t.Latitude, Lon: t.Longitude}
	j.lastRunTime = time.Now()
	j.firstRun = false

	slog.Info("RegionalCategoriesJob: Triggering location-aware context refresh", "lat", t.Latitude, "lon", t.Longitude)

	// Context Data (Capture synchronously to avoid race conditions with telemetry pointer)
	lat := t.Latitude
	lon := t.Longitude
	heading := t.Heading
	onGround := t.IsOnGround

	// Determine ScavengeArea range
	radius := 30.0
	arc := 360.0
	if !onGround {
		radius = float64(j.appCfg.AppConfig().Wikidata.Area.MaxDist) / 1000.0 // Convert meters to km
		arc = 180.0                                                           // +/- 90 degrees forward
	}

	go func() {
		defer j.Unlock() // Release lock when the background job completes

		// 1. Grid Awareness (math.Round for integer-centered tiles)
		latGrid := int(math.Round(lat))
		lonGrid := int(math.Round(lon))

		// 2. Check Cache for current tile and 8 neighbors
		cachedSubclasses, cachedLabels := j.getNeighboringCategories(ctx, latGrid, lonGrid)

		if len(cachedSubclasses) > 0 {
			slog.Info("RegionalCategoriesJob: Loading regional categories from spatial cache", "count", len(cachedSubclasses))

			j.hydrateMissingLabels(ctx, latGrid, lonGrid, cachedSubclasses, cachedLabels)

			j.classifier.AddRegionalCategories(cachedSubclasses, cachedLabels)

			// We also scavenge the area when loading from cache because we might have teleported into this cache zone
			if err := j.wikiSvc.ScavengeArea(ctx, lat, lon, radius, heading, arc); err != nil {
				slog.Warn("RegionalCategoriesJob: Cache Load ScavengeArea failed", "error", err)
			}
		}

		j.discoverNewCategories(ctx, lat, lon, latGrid, lonGrid, radius, heading, arc)
	}()
}

func (j *RegionalCategoriesJob) getNeighboringCategories(ctx context.Context, latGrid, lonGrid int) (subclasses, labels map[string]string) {
	subclasses = make(map[string]string)
	labels = make(map[string]string)
	for dLat := -1; dLat <= 1; dLat++ {
		for dLon := -1; dLon <= 1; dLon++ {
			cats, lbls, err := j.store.GetRegionalCategories(ctx, latGrid+dLat, lonGrid+dLon)
			if err != nil {
				slog.Error("RegionalCategoriesJob: Cache lookup failed", "lat", latGrid+dLat, "lon", lonGrid+dLon, "error", err)
				continue
			}
			for qid, cat := range cats {
				subclasses[qid] = cat
			}
			for qid, l := range lbls {
				labels[qid] = l
			}
		}
	}
	return subclasses, labels
}

func (j *RegionalCategoriesJob) hydrateMissingLabels(ctx context.Context, latGrid, lonGrid int, subclasses, labels map[string]string) {
	missingLabelQIDs := []string{}
	for qid := range subclasses {
		if labels[qid] == "" {
			missingLabelQIDs = append(missingLabelQIDs, qid)
		}
	}

	if len(missingLabelQIDs) > 0 {
		slog.Info("RegionalCategoriesJob: Hydrating missing labels from legacy cache", "count", len(missingLabelQIDs))
		hydrated := j.validator.FetchLabels(ctx, missingLabelQIDs)
		for qid, label := range hydrated {
			labels[qid] = label
		}
		// Save back to spatial cache to persist the fixes
		if err := j.store.SaveRegionalCategories(ctx, latGrid, lonGrid, subclasses, labels); err != nil {
			slog.Warn("RegionalCategoriesJob: Failed to update cache with hydrated labels", "error", err)
		}
	}
}

func (j *RegionalCategoriesJob) discoverNewCategories(ctx context.Context, lat, lon float64, latGrid, lonGrid int, radius, heading, arc float64) {
	// 3. Check if current tile needs LLM discovery
	currentTile, _, _ := j.store.GetRegionalCategories(ctx, latGrid, lonGrid)
	if currentTile != nil {
		slog.Info("RegionalCategoriesJob: Current tile already discovered, skipping LLM", "lat", latGrid, "lon", lonGrid)
		return
	}

	location := j.geo.GetLocation(lat, lon)
	country := j.geo.GetCountry(lat, lon)
	region := location.RegionName
	if region == "" {
		region = location.CityName
	}

	// Build category list for prompt
	catNames := []string{}
	for name := range j.classifier.GetConfig().Categories {
		catNames = append(catNames, name)
	}
	categoryList := strings.Join(catNames, ", ")

	combinedSubclasses := j.generateSubclasses(ctx, lat, lon, country, region, categoryList)

	if len(combinedSubclasses) == 0 {
		slog.Info("RegionalCategoriesJob: No regional categories suggested")
		// Save empty map to current tile to avoid repeat LLM calls for "dead" zones
		_ = j.store.SaveRegionalCategories(ctx, latGrid, lonGrid, make(map[string]string), make(map[string]string))
		return
	}

	j.processSubclasses(ctx, latGrid, lonGrid, combinedSubclasses, radius, heading, arc)
}

func (j *RegionalCategoriesJob) generateSubclasses(ctx context.Context, lat, lon float64, country, region, categoryList string) []subclass {
	data := map[string]any{
		"Lat":          lat,
		"Lon":          lon,
		"Country":      country,
		"Region":       region,
		"CategoryList": categoryList,
	}

	var combined []subclass

	// Dual-Stream Sequential Generation
	profiles := []struct {
		name     string
		template string
	}{
		{"regional_categories_ontological", "context/ontological.tmpl"},
		{"regional_categories_topographical", "context/topographical.tmpl"},
	}

	for _, p := range profiles {
		prompt, err := j.prompts.Render(p.template, data)
		if err != nil {
			slog.Error("RegionalCategoriesJob: Failed to render prompt", "profile", p.name, "error", err)
			continue
		}

		var resp llmResponse
		if err := j.llm.GenerateJSON(ctx, p.name, prompt, &resp); err != nil {
			slog.Error("RegionalCategoriesJob: LLM request failed", "profile", p.name, "error", err)
			continue
		}

		if len(resp.Subclasses) > 0 {
			combined = append(combined, resp.Subclasses...)
			slog.Info("RegionalCategoriesJob: Received suggestions", "profile", p.name, "count", len(resp.Subclasses))
		}
	}
	return combined
}

func (j *RegionalCategoriesJob) processSubclasses(ctx context.Context, latGrid, lonGrid int, subclasses []subclass, radius, heading, arc float64) {
	// Validate QIDs (Lookup by name since we don't trust LLM QIDs)
	suggestions := make(map[string]string)
	for _, sub := range subclasses {
		suggestions[sub.Name] = "" // Empty QID triggers lookup
	}

	validated := j.validator.ValidateBatch(ctx, suggestions)

	// Prepare final regional categories for classifier
	regionalCategories := make(map[string]string)
	for _, sub := range subclasses {
		v, ok := validated[sub.Name]
		if !ok {
			continue
		}
		if j.isKnownQID(v.QID) {
			slog.Debug("RegionalCategoriesJob: Skipping duplicate static QID", "name", sub.Name, "qid", v.QID)
			continue
		}

		// When category is "Generic", use specific_category instead
		effectiveCategory := sub.Category
		if sub.Category == "Generic" && sub.SpecificCategory != "" {
			effectiveCategory = sub.SpecificCategory
			slog.Debug("RegionalCategoriesJob: Using specific_category for Generic", "name", sub.Name, "specific", sub.SpecificCategory)
		}
		// We map the validated QID to the effective category
		regionalCategories[v.QID] = effectiveCategory
		slog.Info("RegionalCategoriesJob: Added regional category", "name", sub.Name, "qid", v.QID, "category", effectiveCategory)
	}

	if len(regionalCategories) > 0 {
		regionalLabels := make(map[string]string)
		for _, sub := range subclasses {
			if v, ok := validated[sub.Name]; ok {
				regionalLabels[v.QID] = v.Label
			}
		}

		j.classifier.AddRegionalCategories(regionalCategories, regionalLabels)
		slog.Info("RegionalCategoriesJob: Appended new regional categories to classifier", "count", len(regionalCategories))

		// Reprocess local cache based on new rules immediately
		if err := j.wikiSvc.ScavengeArea(ctx, float64(latGrid), float64(lonGrid), radius, heading, arc); err != nil {
			slog.Warn("RegionalCategoriesJob: Failed to scavenge area after discovering new rules", "error", err)
		}

		// Save to spatial cache for current tile
		if err := j.store.SaveRegionalCategories(ctx, latGrid, lonGrid, regionalCategories, regionalLabels); err != nil {
			slog.Error("RegionalCategoriesJob: Failed to save to spatial cache", "error", err)
		}
	} else {
		slog.Warn("RegionalCategoriesJob: No valid regional categories found in suggestion")
		// Save empty to avoid repeat LLM
		_ = j.store.SaveRegionalCategories(ctx, latGrid, lonGrid, make(map[string]string), make(map[string]string))
	}
}

type subclass struct {
	Name             string `json:"name"`
	Category         string `json:"category"`
	SpecificCategory string `json:"specific_category"`
	Size             string `json:"size"`
}

type llmResponse struct {
	Subclasses []subclass `json:"subclasses"`
}

func (j *RegionalCategoriesJob) isKnownQID(qid string) bool {
	if j.classifier == nil || j.classifier.GetConfig() == nil {
		return false
	}
	for _, catData := range j.classifier.GetConfig().Categories {
		if _, exists := catData.QIDs[qid]; exists {
			return true
		}
	}
	return false
}

// ResetSession resets the job state for a new flight/teleport.
func (j *RegionalCategoriesJob) ResetSession(ctx context.Context) {
	// We reset the state variables even if the job is currently locked (running).
	// This ensures that as soon as the current run finishes and releases the lock,
	// the next ShouldFire call will return true and trigger a fresh update for the new location.
	j.lastMajorPos = geo.Point{}
	j.lastRunTime = time.Time{} // Reset to zero time to ensure time threshold is also cleared
	j.firstRun = true
	j.classifier.ResetRegionalCategories()
	slog.Info("RegionalCategoriesJob: Session reset and regional categories cleared")
}
