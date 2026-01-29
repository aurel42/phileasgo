package rescue

import (
	"sort"
)

// Article is a local representation of an entity with dimensions for rescue logic.
// This avoids circular dependencies with the wikidata package.
type Article struct {
	ID                  string
	Height              *float64
	Length              *float64
	Area                *float64
	Category            string
	DimensionMultiplier float64
}

// TileStats captures the maximum dimensions observed in a single tile.
type TileStats struct {
	Lat, Lon  float64
	MaxHeight float64
	MaxLength float64
	MaxArea   float64
}

// MedianStats represents the neighborhood medians used as thresholds.
type MedianStats struct {
	MedianHeight float64
	MedianLength float64
	MedianArea   float64
}

// AnalyzeTile extracts the maximum dimensions from a set of articles.
func AnalyzeTile(lat, lon float64, articles []Article) TileStats {
	stats := TileStats{Lat: lat, Lon: lon}
	for _, a := range articles {
		if a.Height != nil && *a.Height > stats.MaxHeight {
			stats.MaxHeight = *a.Height
		}
		if a.Length != nil && *a.Length > stats.MaxLength {
			stats.MaxLength = *a.Length
		}
		if a.Area != nil && *a.Area > stats.MaxArea {
			stats.MaxArea = *a.Area
		}
	}
	return stats
}

// CalculateMedian computes the median of maximum dimensions from a neighborhood.
func CalculateMedian(neighbors []TileStats) MedianStats {
	if len(neighbors) == 0 {
		return MedianStats{}
	}

	heights := make([]float64, 0, len(neighbors))
	lengths := make([]float64, 0, len(neighbors))
	areas := make([]float64, 0, len(neighbors))

	for _, n := range neighbors {
		heights = append(heights, n.MaxHeight)
		lengths = append(lengths, n.MaxLength)
		areas = append(areas, n.MaxArea)
	}

	return MedianStats{
		MedianHeight: median(heights),
		MedianLength: median(lengths),
		MedianArea:   median(areas),
	}
}

// Batch identifies significant unclassified entities based on neighborhood medians and noise floors.
// It returns a list of rescued articles (copies with categories assigned).
func Batch(candidates []Article, localMax TileStats, medians MedianStats, minH, minL, minA float64) []Article {
	var rescued []Article

	// Dimension: Height
	if a := rescueDimension(candidates, "height", localMax.MaxHeight, medians.MedianHeight, minH); a != nil {
		rescued = append(rescued, *a)
	}

	// Dimension: Length
	if a := rescueDimension(candidates, "length", localMax.MaxLength, medians.MedianLength, minL); a != nil {
		if !isDuplicate(rescued, a.ID) {
			rescued = append(rescued, *a)
		}
	}

	// Dimension: Area
	if a := rescueDimension(candidates, "area", localMax.MaxArea, medians.MedianArea, minA); a != nil {
		if !isDuplicate(rescued, a.ID) {
			rescued = append(rescued, *a)
		}
	}

	return rescued
}

func rescueDimension(candidates []Article, category string, maxVal, medianVal, minVal float64) *Article {
	if maxVal <= 0 {
		return nil
	}

	threshold := 2 * medianVal
	if threshold < minVal {
		threshold = minVal
	}

	if maxVal < threshold {
		return nil
	}

	a := findBest(candidates, func(art Article) bool {
		switch category {
		case "height":
			return art.Height != nil && *art.Height == maxVal
		case "length":
			return art.Length != nil && *art.Length == maxVal
		case "area":
			return art.Area != nil && *art.Area == maxVal
		}
		return false
	})

	if a == nil {
		return nil
	}

	a.Category = category
	a.DimensionMultiplier = 1.0
	if medianVal > 0 {
		a.DimensionMultiplier = maxVal / medianVal
	}
	return a
}

func median(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sort.Float64s(data)
	mid := len(data) / 2
	if len(data)%2 == 0 {
		return (data[mid-1] + data[mid]) / 2
	}
	return data[mid]
}

func findBest(candidates []Article, matcher func(Article) bool) *Article {
	for i := range candidates {
		if matcher(candidates[i]) {
			return &candidates[i]
		}
	}
	return nil
}

func isDuplicate(list []Article, id string) bool {
	for _, a := range list {
		if a.ID == id {
			return true
		}
	}
	return false
}
