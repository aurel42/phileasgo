package classifier

import (
	"sort"
	"sync"
)

// DimensionRecord stores the max dimensions for a single tile.
type DimensionRecord struct {
	MaxHeight float64
	MaxLength float64
	MaxArea   float64
}

// DimensionTracker tracks dimensional records across tiles to identify "huge" objects.
type DimensionTracker struct {
	mu sync.RWMutex

	// Window for median calculation
	windowSize int
	records    []DimensionRecord

	// Current tile stats
	currentHeight float64
	currentLength float64
	currentArea   float64
}

// NewDimensionTracker creates a new tracker with the specified window size.
func NewDimensionTracker(windowSize int) *DimensionTracker {
	return &DimensionTracker{
		windowSize: windowSize,
		records:    make([]DimensionRecord, 0, windowSize),
	}
}

// ResetTile prepares the tracker for a new tile batch.
func (dt *DimensionTracker) ResetTile() {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	dt.currentHeight = 0
	dt.currentLength = 0
	dt.currentArea = 0
}

// ObserveArticle updates the current tile's max records based on an article.
// Should be called for every article in the tile.
func (dt *DimensionTracker) ObserveArticle(h, l, a float64) {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	if h > dt.currentHeight {
		dt.currentHeight = h
	}
	if l > dt.currentLength {
		dt.currentLength = l
	}
	if a > dt.currentArea {
		dt.currentArea = a
	}
}

// FinalizeTile adds the current tile's max records to the sliding window.
func (dt *DimensionTracker) FinalizeTile() {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	rec := DimensionRecord{
		MaxHeight: dt.currentHeight,
		MaxLength: dt.currentLength,
		MaxArea:   dt.currentArea,
	}

	dt.records = append(dt.records, rec)
	if len(dt.records) > dt.windowSize {
		dt.records = dt.records[1:]
	}
}

// ShouldRescue returns true if the given dimensions qualify the object as a Landmark.
func (dt *DimensionTracker) ShouldRescue(h, l, a float64) bool {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	if h <= 0 && l <= 0 && a <= 0 {
		return false
	}

	// 1. Is it a tile record?
	if (h > 0 && h >= dt.currentHeight) ||
		(l > 0 && l >= dt.currentLength) ||
		(a > 0 && a >= dt.currentArea) {
		return true
	}

	// 2. Does it exceed the global median? (Only if window is full)
	if len(dt.records) < dt.windowSize {
		return false
	}

	return dt.exceedsMedians(h, l, a)
}

// GetMultiplier calculates the score multiplier based on dimensions.
// Returns 1.0, 2.0 (Tile Record), or 4.0 (Tile Record + Exceeds Median).
func (dt *DimensionTracker) GetMultiplier(h, l, a float64) float64 {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	if h <= 0 && l <= 0 && a <= 0 {
		return 1.0
	}

	mult := 1.0

	// 1. Tile Record Check
	isRecord := (h > 0 && h >= dt.currentHeight) ||
		(l > 0 && l >= dt.currentLength) ||
		(a > 0 && a >= dt.currentArea)

	if isRecord {
		mult = 2.0
	}

	// 2. Median Check (Additive)
	// Only if we have a full window of history
	if len(dt.records) >= dt.windowSize {
		if dt.exceedsMedians(h, l, a) {
			if mult == 1.0 {
				mult = 2.0 // Just exceeds median -> 2.0
			} else {
				mult *= 2.0 // Record AND Exceeds Median -> 4.0
			}
		}
	}

	return mult
}

func (dt *DimensionTracker) exceedsMedians(h, l, a float64) bool {
	medH, medL, medA := dt.getMedians()
	return (h > 0 && h > medH) || (l > 0 && l > medL) || (a > 0 && a > medA)
}

func (dt *DimensionTracker) getMedians() (medH, medL, medA float64) {
	if len(dt.records) == 0 {
		return 0, 0, 0
	}

	hs := make([]float64, len(dt.records))
	ls := make([]float64, len(dt.records))
	as := make([]float64, len(dt.records))

	for i, r := range dt.records {
		hs[i] = r.MaxHeight
		ls[i] = r.MaxLength
		as[i] = r.MaxArea
	}

	return median(hs), median(ls), median(as)
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	return values[len(values)/2]
}
