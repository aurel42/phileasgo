package wikidata

import (
	"log/slog"
	"math"
	"sort"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
)

// MergePOIs groups spatially close POIs and selects the best candidate based on Article Length.
func MergePOIs(candidates []*model.POI, cfg *config.CategoriesConfig, logger *slog.Logger) []*model.POI {
	if len(candidates) == 0 {
		return nil
	}

	// 1. Sort Candidates by "Quality" (Length Descending)
	// This ensures we always process the "best" POIs first and let them "gobble" smaller ones.
	sort.Slice(candidates, func(i, j int) bool {
		// Priority 1: Article Length
		if candidates[i].WPArticleLength != candidates[j].WPArticleLength {
			return candidates[i].WPArticleLength > candidates[j].WPArticleLength
		}
		// Priority 2: Sitelinks
		if candidates[i].Sitelinks != candidates[j].Sitelinks {
			return candidates[i].Sitelinks > candidates[j].Sitelinks
		}
		// Priority 3: Stability (QID)
		return candidates[i].WikidataID < candidates[j].WikidataID
	})

	var accepted []*model.POI

	// 2. Greedy Selection
	for _, cand := range candidates {
		// Determine Merge Distance for this Candidate
		// Use the candidate's size category to determine its "influence radius"
		candSize := cand.Size
		if candSize == "" {
			candSize = cfg.GetSize(cand.Category)
		}
		candRadius := cfg.GetMergeDistance(candSize)

		isDuplicate := false

		// Check against already accepted POIs
		for _, acc := range accepted {
			// Determine Merge Distance for Accepted POI
			accSize := acc.Size
			if accSize == "" {
				accSize = cfg.GetSize(acc.Category)
			}
			accRadius := cfg.GetMergeDistance(accSize)

			// Effective Merge Distance is the MAX of the two radii
			// Rationale: A Large item (Airport) should gobble a Small item (Terminal) even if the Small item has a small radius.
			mergeDist := math.Max(candRadius, accRadius)

			distMeters := geo.Distance(
				geo.Point{Lat: cand.Lat, Lon: cand.Lon},
				geo.Point{Lat: acc.Lat, Lon: acc.Lon},
			)

			if distMeters < mergeDist {
				// It's a duplicate!
				// Since we sorted by quality, 'acc' is better or equal to 'cand'.
				// We drop 'cand'.
				isDuplicate = true
				if logger != nil {
					// slog.Debug("Merged POI",
					// 	"kept", acc.DisplayName(), "kept_qid", acc.WikidataID, "kept_len", acc.WPArticleLength,
					// 	"dropped", cand.DisplayName(), "dropped_qid", cand.WikidataID, "dropped_len", cand.WPArticleLength,
					// 	"dist_m", distMeters, "threshold", mergeDist)
				}
				break
			}
		}

		if !isDuplicate {
			accepted = append(accepted, cand)
		}
	}

	return accepted
}
