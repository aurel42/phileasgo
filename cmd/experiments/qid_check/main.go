package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"time"

	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
	"phileasgo/pkg/wikidata"
)

// noOpCache implements cache.Cacher with a simple in-memory map or just no-op
type noOpCache struct{}

func (c *noOpCache) GetCache(ctx context.Context, key string) ([]byte, bool)    { return nil, false }
func (c *noOpCache) SetCache(ctx context.Context, key string, val []byte) error { return nil }

func main() {
	// Load categories.yaml
	data, err := os.ReadFile("configs/categories.yaml")
	if err != nil {
		slog.Error("Failed to read categories.yaml", "error", err)
		os.Exit(1)
	}

	var qids []string
	seen := make(map[string]bool)

	// Simple regex to find "Q" followed by digits
	re := regexp.MustCompile(`"Q(\d+)"`)

	matches := re.FindAllStringSubmatch(string(data), -1)
	for _, match := range matches {
		if len(match) > 1 {
			qid := "Q" + match[1]
			if !seen[qid] {
				qids = append(qids, qid)
				seen[qid] = true
			}
		}
	}

	fmt.Printf("Found %d unique QIDs in configs/categories.yaml\n", len(qids))

	if len(qids) == 0 {
		fmt.Println("No QIDs found, check regex or file content.")
		os.Exit(0)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	tr := tracker.New()
	c := &noOpCache{}
	reqClient := request.New(c, tr)
	wdClient := wikidata.NewClient(reqClient, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	start := time.Now()
	// GetEntitiesBatch handles batching internally (chunks of 50)
	meta, err := wdClient.GetEntitiesBatch(ctx, qids)
	if err != nil {
		slog.Error("Failed to get entities batch", "error", err)
		cancel()
		os.Exit(1)
	}
	duration := time.Since(start)
	cancel()

	fmt.Printf("Batch lookup took %v for %d QIDs\n", duration, len(qids))

	// Validate results
	missingCount := 0
	noLabelCount := 0

	for _, qid := range qids {
		if m, ok := meta[qid]; !ok {
			fmt.Printf("MISSING: %s\n", qid)
			missingCount++
		} else {
			if _, exists := m.Labels["en"]; !exists {
				fmt.Printf("Visual check: QID %s has no English label\n", qid)
				noLabelCount++
			}
		}
	}

	if missingCount == 0 {
		fmt.Println("SUCCESS: All QIDs resolved.")
	} else {
		fmt.Printf("FAILURE: %d QIDs failed to resolve.\n", missingCount)
	}
	if noLabelCount > 0 {
		fmt.Printf("WARNING: %d QIDs have no English label.\n", noLabelCount)
	}
}
