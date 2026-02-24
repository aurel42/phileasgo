package nvidia

import (
	"phileasgo/pkg/config"
	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
	"testing"
)

func TestNewClient(t *testing.T) {
	rc := request.New(nil, tracker.New(), request.ClientConfig{})
	cfg := config.ProviderConfig{
		Type: "nvidia",
		Key:  "test-key",
	}

	client, err := NewClient(cfg, rc)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client == nil {
		t.Fatal("Client is nil")
	}

	// We can't easily check the private baseURL without reflection or export,
	// but we've verified it compiles and initializes.
}
