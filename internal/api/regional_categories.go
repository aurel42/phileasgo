package api

import (
	"encoding/json"
	"net/http"
	"sort"

	"phileasgo/pkg/classifier"
	"phileasgo/pkg/store"
)

// RegionalCategoriesHandler exposes the currently active dynamic regional categories.
type RegionalCategoriesHandler struct {
	classifier *classifier.Classifier
	store      store.HierarchyStore
}

// NewRegionalCategoriesHandler creates a new handler. Returns nil if dependencies are missing.
func NewRegionalCategoriesHandler(c *classifier.Classifier, s store.HierarchyStore) *RegionalCategoriesHandler {
	if c == nil || s == nil {
		return nil
	}
	return &RegionalCategoriesHandler{
		classifier: c,
		store:      s,
	}
}

// RegionalCategoryResponse represents a single mapped regional category.
type RegionalCategoryResponse struct {
	QID      string `json:"qid"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

// HandleGet returns the actively configured regional categories with display names.
func (h *RegionalCategoriesHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	cats := h.classifier.GetRegionalCategories()

	// Fast response for empty map
	if len(cats) == 0 {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte("[]\n")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	response := make([]RegionalCategoryResponse, 0, len(cats))
	ctx := r.Context()
	regionalLabels := h.classifier.GetRegionalLabels()

	// Look up the human-readable names
	for qid, categoryName := range cats {
		displayName := qid

		// 1. Try regional labels from classifier (labels discovered by LLM/Validator)
		if lbl, ok := regionalLabels[qid]; ok && lbl != "" {
			displayName = lbl
		} else {
			// 2. Fallback to hierarchy DB cache
			hNode, err := h.store.GetHierarchy(ctx, qid)
			if err == nil && hNode != nil && hNode.Name != "" {
				displayName = hNode.Name
			}
		}

		response = append(response, RegionalCategoryResponse{
			QID:      qid,
			Name:     displayName,
			Category: categoryName,
		})
	}

	// Sort by QID to ensure stable response order for EFB comparison
	sort.Slice(response, func(i, j int) bool {
		return response[i].QID < response[j].QID
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
