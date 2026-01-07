package poi

import (
	"context"
	"testing"

	"phileasgo/pkg/config"
)

func TestManager_ResetHistory(t *testing.T) {
	mockStore := NewMockStore()
	mgr := NewManager(&config.Config{}, mockStore, nil)
	ctx := context.Background()

	// Calling ResetLastPlayed should not panic and should delegate to store (mock returns nil)
	err := mgr.ResetLastPlayed(ctx, 10.0, 20.0, 5000.0)
	if err != nil {
		t.Errorf("ResetLastPlayed failed: %v", err)
	}
}
