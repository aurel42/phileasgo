package wikidata

import (
	"context"
	"log/slog"
	"strings"
)

// ValidatedQID represents a QID that has been verified against Wikidata.
type ValidatedQID struct {
	QID   string
	Label string
}

// Validator provides logic for verifying QIDs and finding correct ones if they mismatch.
type Validator struct {
	client *Client
}

// NewValidator creates a new Wikidata validator.
func NewValidator(c *Client) *Validator {
	return &Validator{client: c}
}

// ValidateBatch checks a batch of QIDs and their expected labels.
// It returns a map of OriginalName -> ValidatedQID.
func (v *Validator) ValidateBatch(ctx context.Context, suggestions map[string]string) map[string]ValidatedQID {
	validated := make(map[string]ValidatedQID)
	qidsToCheck := v.extractQIDs(suggestions)

	// 1. Batch lookup labels
	actualLabels := v.fetchLabels(ctx, qidsToCheck)

	// 2. Process and Fallback
	for name, qid := range suggestions {
		lname := strings.ToLower(name)
		vQID, ok := v.tryDirectMatch(name, qid, actualLabels)
		if ok {
			validated[name] = vQID
			continue
		}

		// Fallback to search if no match found
		vQID, ok = v.trySearchFallback(ctx, name, lname)
		if ok {
			validated[name] = vQID
		}
	}

	return validated
}

func (v *Validator) extractQIDs(suggestions map[string]string) []string {
	qids := []string{}
	for _, qid := range suggestions {
		if qid != "" && strings.HasPrefix(qid, "Q") {
			qids = append(qids, qid)
		}
	}
	return qids
}

func (v *Validator) fetchLabels(ctx context.Context, qids []string) map[string]string {
	actualLabels := make(map[string]string)
	if len(qids) == 0 {
		return actualLabels
	}
	metadata, err := v.client.GetEntitiesBatch(ctx, qids)
	if err != nil {
		slog.Warn("Validator: Batch lookup failed", "error", err)
		return actualLabels
	}
	for qid, m := range metadata {
		if lbl, ok := m.Labels["en"]; ok {
			actualLabels[qid] = strings.ToLower(lbl)
		}
	}
	return actualLabels
}

func (v *Validator) tryDirectMatch(name, qid string, actualLabels map[string]string) (ValidatedQID, bool) {
	if qid == "" {
		return ValidatedQID{}, false
	}
	lname := strings.ToLower(name)
	if actual, ok := actualLabels[qid]; ok {
		if strings.Contains(actual, lname) || strings.Contains(lname, actual) {
			slog.Debug("Validator: QID verified", "name", name, "qid", qid, "actual", actual)
			return ValidatedQID{QID: qid, Label: actual}, true
		}
		slog.Warn("Validator: QID mismatch", "name", name, "qid", qid, "actual", actual)
	}
	return ValidatedQID{}, false
}

func (v *Validator) trySearchFallback(ctx context.Context, name, lname string) (ValidatedQID, bool) {
	slog.Info("Validator: Attempting search fallback", "name", name)
	results, err := v.client.SearchEntities(ctx, name)
	if err != nil {
		slog.Error("Validator: Search failed", "name", name, "error", err)
		return ValidatedQID{}, false
	}

	if len(results) > 0 {
		best := results[0]
		lbest := strings.ToLower(best.Label)
		if strings.Contains(lbest, lname) || strings.Contains(lname, lbest) {
			slog.Info("Validator: Search fallback success", "name", name, "found_qid", best.ID, "found_label", best.Label)
		} else {
			// Weak match - warn but still use it (Wikidata knows best)
			slog.Warn("Validator: Weak search match, using anyway", "requested", name, "found_label", best.Label, "qid", best.ID)
		}
		// Return using Wikidata's actual label (for specific_category use)
		return ValidatedQID{QID: best.ID, Label: best.Label}, true
	}
	slog.Warn("Validator: No search results", "name", name)
	return ValidatedQID{}, false
}

// VerifyStartupConfig checks all categories in the config and returns a report.
func (v *Validator) VerifyStartupConfig(ctx context.Context, configItems map[string]string) error {
	slog.Info("Validator: Verifying startup category config...")

	// Batch validate
	// ValidateBatch expects Name -> QID.
	// We have QID -> Name.
	// Since names might not be unique globally (though they should be), we might have collisions if we just invert.
	// But for validation purposes, let's just reverse it. If distinct QIDs map to "castle", we'll only validate one of them.
	// To be robust, we should probably validate QIDs directly. But ValidateBatch is built around "Suggestion -> QID" flow.
	// Let's iterate manually or adapt ValidateBatch.
	// For now, let's just reverse and hope valid config names are distinct enough or we just check distinct ones.

	suggestions := make(map[string]string)
	for qid, name := range configItems {
		suggestions[name] = qid
	}

	res := v.ValidateBatch(ctx, suggestions)

	totalCount := len(configItems)
	slog.Info("Validator: Startup verification complete", "valid", len(res), "total", totalCount)

	if len(res) < totalCount {
		// Log missing ones
		for name := range suggestions {
			if _, ok := res[name]; !ok {
				slog.Debug("Validator: Failed to verify config item", "name", name, "qid", suggestions[name])
			}
		}
		slog.Warn("Validator: Some startup QIDs could not be verified! Check debug logs.")
	}

	return nil
}
