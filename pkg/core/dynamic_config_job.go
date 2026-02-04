package core

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"phileasgo/pkg/classifier"
	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/wikidata"
)

// DynamicConfigJob triggers AI-suggested Wikidata categories based on location.
type DynamicConfigJob struct {
	BaseJob
	appCfg     config.Provider
	llm        llm.Provider
	prompts    *prompts.Manager
	validator  *wikidata.Validator
	classifier *classifier.Classifier
	geo        *geo.Service
	wikiSvc    *wikidata.Service

	lastMajorPos geo.Point
	lastRunTime  time.Time
	firstRun     bool
}

func NewDynamicConfigJob(
	appCfg config.Provider,
	llmProv llm.Provider,
	prompts *prompts.Manager,
	validator *wikidata.Validator,
	classifier *classifier.Classifier,
	geoSvc *geo.Service,
	wikiSvc *wikidata.Service,
) *DynamicConfigJob {
	return &DynamicConfigJob{
		BaseJob:    NewBaseJob("Dynamic Config"),
		appCfg:     appCfg,
		llm:        llmProv,
		prompts:    prompts,
		validator:  validator,
		classifier: classifier,
		geo:        geoSvc,
		wikiSvc:    wikiSvc,
		firstRun:   true,
	}
}

func (j *DynamicConfigJob) ShouldFire(t *sim.Telemetry) bool {
	// 0. Disable if no provider supports dynamic_config
	if !j.llm.HasProfile("dynamic_config") {
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

func (j *DynamicConfigJob) Run(ctx context.Context, t *sim.Telemetry) {
	if !j.TryLock() {
		return
	}
	// Note: We DO NOT defer Unlock() here. The goroutine is responsible for unlocking.

	j.lastMajorPos = geo.Point{Lat: t.Latitude, Lon: t.Longitude}
	j.lastRunTime = time.Now()
	j.firstRun = false

	slog.Info("DynamicConfigJob: Triggering location-aware config", "lat", t.Latitude, "lon", t.Longitude)

	// Context Data (Capture synchronously to avoid race conditions with telemetry pointer)
	lat := t.Latitude
	lon := t.Longitude

	go func() {
		defer j.Unlock() // Release lock when the background job completes

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

		data := map[string]any{
			"Lat":          lat,
			"Lon":          lon,
			"Country":      country,
			"Region":       region,
			"CategoryList": categoryList,
		}

		prompt, err := j.prompts.Render("context/wikidata.tmpl", data)
		if err != nil {
			slog.Error("DynamicConfigJob: Failed to render prompt", "error", err)
			return
		}

		var response struct {
			Subclasses []struct {
				Name             string `json:"name"`
				Category         string `json:"category"`
				SpecificCategory string `json:"specific_category"`
				Size             string `json:"size"`
			} `json:"subclasses"`
		}

		// Use a detached context for the async job if needed, but inheriting ctx ensures cancellation on shutdown
		if err := j.llm.GenerateJSON(ctx, "dynamic_config", prompt, &response); err != nil {
			slog.Error("DynamicConfigJob: Gemini request failed", "error", err)
			return
		}

		if len(response.Subclasses) == 0 {
			slog.Info("DynamicConfigJob: Gemini suggested no subclasses")
			return
		}

		// Validate QIDs (Lookup by name since we don't trust LLM QIDs)
		suggestions := make(map[string]string)
		for _, sub := range response.Subclasses {
			suggestions[sub.Name] = "" // Empty QID triggers lookup
		}

		validated := j.validator.ValidateBatch(ctx, suggestions)

		// Prepare final dynamic interests for classifier
		dynamicInterests := make(map[string]string)
		for _, sub := range response.Subclasses {
			v, ok := validated[sub.Name]
			if !ok {
				continue
			}
			if j.isKnownQID(v.QID) {
				slog.Info("DynamicConfigJob: Skipping duplicate static QID", "name", sub.Name, "qid", v.QID)
				continue
			}

			// When category is "Generic", use specific_category instead
			effectiveCategory := sub.Category
			if sub.Category == "Generic" && sub.SpecificCategory != "" {
				effectiveCategory = sub.SpecificCategory
				slog.Info("DynamicConfigJob: Using specific_category for Generic", "name", sub.Name, "specific", sub.SpecificCategory)
			}
			// We map the validated QID to the effective category
			dynamicInterests[v.QID] = effectiveCategory
			slog.Info("DynamicConfigJob: Added dynamic interest", "name", sub.Name, "qid", v.QID, "category", effectiveCategory)
		}

		if len(dynamicInterests) > 0 {
			j.classifier.SetDynamicInterests(dynamicInterests)
			slog.Info("DynamicConfigJob: Updated classifier with new dynamic interests", "count", len(dynamicInterests))
			// Reprocessing disabled per user request
		} else {
			slog.Warn("DynamicConfigJob: No valid interests found in suggestion")
		}
	}()
}
func (j *DynamicConfigJob) isKnownQID(qid string) bool {
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
func (j *DynamicConfigJob) ResetSession(ctx context.Context) {
	// We need to lock because these fields are used in ShouldFire
	if !j.TryLock() {
		// If locked, we can't reset safely right now.
		// However, since this is called on Teleport detection (sync tick),
		// we might force it? Or just wait?
		// For simplicity, just reset. This job runs infrequently.
		// Actually, if it's running, we should let it finish or force reset.
		// Given BaseJob logic, we can't force lock easily.
		// But simple assignment is atomic enough for bool/struct copy? No.
		// Let's just log warning if locked.
		slog.Warn("DynamicConfigJob: Could not lock for session reset (job running)")
		return
	}
	defer j.Unlock()

	j.lastMajorPos = geo.Point{}
	j.lastRunTime = time.Time{}
	j.firstRun = true
	slog.Info("DynamicConfigJob: Session reset")
}
