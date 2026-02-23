package api

import (
	"encoding/json"
	"net/http"

	"phileasgo/pkg/classifier"
	"phileasgo/pkg/store"
)

// RegionalCategoriesHandler exposes the currently active dynamic regional categories.
type RegionalCategoriesHandler struct {
	classifier *classifier.Classifier
	store      store.HierarchyStore
}

// NewRegionalCategoriesHandler creates a new handler.
func NewRegionalCategoriesHandler(c *classifier.Classifier, s store.HierarchyStore) *RegionalCategoriesHandler {
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

	// Look up the human-readable names from the hierarchy DB cache
	for qid, categoryName := range cats {
		displayName := qid

		hNode, err := h.store.GetHierarchy(ctx, qid)
		if err == nil && hNode != nil && hNode.Name != "" {
			displayName = hNode.Name
		}

		response = append(response, RegionalCategoryResponse{
			QID:      qid,
			Name:     displayName,
			Category: categoryName,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
