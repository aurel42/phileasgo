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
// It returns the accepted POIs and a list of QIDs that were rejected (merged away).
func MergePOIs(candidates []*model.POI, cfg *config.CategoriesConfig, logger *slog.Logger) (accepted []*model.POI, rejected []string) {
	if len(candidates) == 0 {
		return nil, nil
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

	// 2. Greedy Selection
	for _, cand := range candidates {
		// Determine Merge Distance for this Candidate
		// Use the candidate's size category to determine its "influence radius"
		candSize := cand.Size
		if candSize == "" {
			candSize = cfg.GetSize(cand.Category)
		}
		candRadius := cfg.GetMergeDistance(candSize)
		candGroup := cfg.GetGroup(cand.Category)

		isDuplicate := false

		// Check against already accepted POIs
		for _, acc := range accepted {
			// 2a. Group Isolation Check: Never merge across different category groups
			accGroup := cfg.GetGroup(acc.Category)
			if candGroup != "" && accGroup != "" && candGroup != accGroup {
				continue // distinct items, skip merge check against this 'acc'
			}

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
				rejected = append(rejected, cand.WikidataID)
				break
			}
		}

		if !isDuplicate {
			accepted = append(accepted, cand)
		}
	}

	return accepted, rejected
}

// MergeArticles groups spatially close Articles and selects the best candidate based on Sitelinks.
// This runs BEFORE hydration/enrichment to reduce API calls.
// It returns the accepted Articles and a list of QIDs that were rejected.
func MergeArticles(candidates []Article, cfg *config.CategoriesConfig, logger *slog.Logger) (accepted []Article, rejected []string) {
	if len(candidates) == 0 {
		return nil, nil
	}

	// 1. Sort Candidates by "Importance" Proxy (Sitelinks Descending)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Sitelinks != candidates[j].Sitelinks {
			return candidates[i].Sitelinks > candidates[j].Sitelinks
		}
		return candidates[i].QID < candidates[j].QID
	})

	// 2. Greedy Selection
	for i := range candidates {
		cand := &candidates[i]
		// Determine Merge Distance
		candSize := cfg.GetSize(cand.Category)
		candRadius := cfg.GetMergeDistance(candSize)
		candGroup := cfg.GetGroup(cand.Category)

		isDuplicate := false

		for j := range accepted {
			acc := &accepted[j]
			// Determine Merge Distance for Accepted Item
			accSize := cfg.GetSize(acc.Category)
			accRadius := cfg.GetMergeDistance(accSize)

			// 2a. Group Isolation Check: Never merge across different category groups
			accGroup := cfg.GetGroup(acc.Category)
			if candGroup != "" && accGroup != "" && candGroup != accGroup {
				continue // distinct items, skip merge check against this 'acc'
			}

			// 2b. Spatial Check
			mergeDist := math.Max(candRadius, accRadius)
			distMeters := geo.Distance(
				geo.Point{Lat: cand.Lat, Lon: cand.Lon},
				geo.Point{Lat: acc.Lat, Lon: acc.Lon},
			)

			if distMeters < mergeDist {
				// It's a duplicate!
				isDuplicate = true
				rejected = append(rejected, cand.QID)
				break
			}
		}

		if !isDuplicate {
			accepted = append(accepted, *cand)
		}
	}

	return accepted, rejected
}
