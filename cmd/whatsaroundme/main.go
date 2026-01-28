// Package main provides a debugging CLI tool to inspect POIs around the current aircraft position.
// It fetches telemetry from the running PhileasGo server, retrieves cached geodata tiles
// from the database, parses them, and runs classification to show why each POI was
// categorized or ignored.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"phileasgo/pkg/classifier"
	"phileasgo/pkg/config"
	"phileasgo/pkg/db"
	"phileasgo/pkg/request"
	"phileasgo/pkg/store"
	"phileasgo/pkg/tracker"
	"phileasgo/pkg/wikidata"
)

// TelemetryResponse matches internal/api.TelemetryResponse
type TelemetryResponse struct {
	Latitude  float64 `json:"Latitude"`
	Longitude float64 `json:"Longitude"`
	Valid     bool    `json:"Valid"`
}

type articleDebug struct {
	wikidata.Article
	Distance float64
	TileKey  string
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfgPath := flag.String("config", "configs/phileas.yaml", "Path to config file")
	radius := flag.Float64("radius", 15.0, "Search radius in kilometers")
	showAll := flag.Bool("all", false, "Show all articles, not just first 50")
	checkQID := flag.String("check", "", "Check specific QID in DB (poi and seen_entities tables)")
	flag.Parse()

	// Load config
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Fetch telemetry from running server
	serverAddr := cfg.Server.Address
	if serverAddr == "" {
		serverAddr = "localhost:1920"
	}

	tel, err := fetchTelemetry(serverAddr)
	if err != nil {
		return fmt.Errorf("failed to fetch telemetry: %w\nIs PhileasGo running?", err)
	}

	if !tel.Valid {
		return fmt.Errorf("telemetry not valid (no data received from simulator yet)")
	}

	fmt.Printf("Position: %.4f, %.4f\n", tel.Latitude, tel.Longitude)
	fmt.Printf("Search radius: %.1f km\n\n", *radius)

	// Open database
	database, err := db.Init(cfg.DB.Path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	st := store.NewSQLiteStore(database)
	ctx := context.Background()

	if *checkQID != "" {
		checkDB(ctx, database, *checkQID)
		return nil
	}

	// Find tiles in the vicinity (±0.5° bounding box, approximately 55km)
	degRadius := 0.5
	tiles, err := st.GetGeodataInBounds(ctx,
		tel.Latitude-degRadius, tel.Latitude+degRadius,
		tel.Longitude-degRadius, tel.Longitude+degRadius,
	)
	if err != nil {
		return fmt.Errorf("failed to query tiles: %w", err)
	}

	if len(tiles) == 0 {
		fmt.Println("WARN: No cached geodata tiles found near this position.")
		fmt.Println("      Fly around a bit to let PhileasGo fetch some tiles.")
		return nil
	}

	fmt.Printf("Found %d cached tiles\n\n", len(tiles))

	// Analyze tiles
	articles, err := scanTiles(ctx, st, tiles, tel.Latitude, tel.Longitude, *radius*1000)
	if err != nil {
		return err
	}

	if len(articles) == 0 {
		fmt.Println("WARN: No articles found within radius.")
		return nil
	}

	// Prepare Classifier and Clients
	catCfg, err := config.LoadCategories("configs/categories.yaml")
	if err != nil {
		return fmt.Errorf("failed to load categories config: %w", err)
	}

	// Initialize Tracker and Request Client (needed for Classifier -> Client -> GetEntityClaims)
	tr := tracker.New()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	reqCfg := request.ClientConfig{
		Retries:   1,
		Timeout:   10 * time.Second,
		BaseDelay: 500 * time.Millisecond,
		MaxDelay:  2 * time.Second,
	}
	// We pass 'st' as the cache provider for the request client
	reqClient := request.New(st, tr, reqCfg)

	// Pass the request client to Wikidata Client
	wdClient := wikidata.NewClient(reqClient, logger)

	cls := classifier.NewClassifier(st, wdClient, catCfg, tr)

	printResults(ctx, cls, articles, *radius, *showAll)

	return nil
}

func scanTiles(ctx context.Context, st *store.SQLiteStore, tiles []store.GeodataRecord, lat, lon, radiusMeters float64) ([]articleDebug, error) {
	var allArticles []articleDebug

	for _, tile := range tiles {
		data, _, found := st.GetGeodataCache(ctx, tile.Key)
		if !found || len(data) == 0 {
			continue
		}

		articles, _, err := wikidata.ParseSPARQLStreaming(bytes.NewReader(data))
		if err != nil {
			fmt.Printf("WARN: Failed to parse tile %s: %v\n", tile.Key, err)
			continue
		}

		for i := range articles {
			a := &articles[i]
			dist := haversine(lat, lon, a.Lat, a.Lon)
			if dist > radiusMeters {
				continue
			}

			allArticles = append(allArticles, articleDebug{
				Article:  *a,
				Distance: dist,
				TileKey:  tile.Key,
			})
		}
	}
	return allArticles, nil
}

func printResults(ctx context.Context, cls *classifier.Classifier, allArticles []articleDebug, radiusKm float64, showAll bool) {
	// Sort by distance
	sort.Slice(allArticles, func(i, j int) bool {
		return allArticles[i].Distance < allArticles[j].Distance
	})

	// Limit output
	displayCount := len(allArticles)
	if !showAll && displayCount > 50 {
		displayCount = 50
	}

	fmt.Printf("Found %d articles within %.1f km (showing %d)\n\n", len(allArticles), radiusKm, displayCount)
	fmt.Println(strings.Repeat("-", 80))

	for i := 0; i < displayCount; i++ {
		a := allArticles[i]
		fmt.Printf("\nPOI: %s (%s)\n", a.Article.Label, a.Article.QID)
		fmt.Printf("   Loc:       %.4f, %.4f (%.1f km away)\n", a.Article.Lat, a.Article.Lon, a.Distance/1000)
		fmt.Printf("   Sitelinks: %d\n", a.Article.Sitelinks)

		if len(a.Article.Instances) > 0 {
			fmt.Printf("   Instances: %s\n", strings.Join(a.Article.Instances, ", "))
		}

		// Run classification explanation
		exp, err := cls.Explain(ctx, a.Article.QID)
		if err != nil {
			fmt.Printf("   ERROR: Classification failed: %v\n", err)
			continue
		}

		switch {
		case exp.Ignored:
			fmt.Printf("   IGNORED:   %s\n", exp.Reason)
		case exp.Category != "":
			fmt.Printf("   MATCH:     %s (size: %s)\n", exp.Category, exp.Size)
			fmt.Printf("   Reason:    %s\n", exp.Reason)
		default:
			fmt.Printf("   NO MATCH:  %s\n", exp.Reason)
		}
	}

	fmt.Println(strings.Repeat("-", 80))
	if len(allArticles) > displayCount {
		fmt.Printf("\n... and %d more. Use -all to see all.\n", len(allArticles)-displayCount)
	}
}

func fetchTelemetry(addr string) (*TelemetryResponse, error) {
	url := fmt.Sprintf("http://%s/api/telemetry", addr)
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var tel TelemetryResponse
	if err := json.NewDecoder(resp.Body).Decode(&tel); err != nil {
		return nil, err
	}
	return &tel, nil
}

// haversine calculates the distance in meters between two lat/lon points.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000 // Earth radius in meters

	dLat := (lat2 - lat1) * (math.Pi / 180)
	dLon := (lon2 - lon1) * (math.Pi / 180)
	lat1Rad := lat1 * (math.Pi / 180)
	lat2Rad := lat2 * (math.Pi / 180)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

func checkDB(ctx context.Context, db *db.DB, qid string) {
	fmt.Printf("\nInspecting database for QID: %s\n", qid)
	fmt.Println(strings.Repeat("-", 50))

	// Check POI table
	var category, name string
	err := db.QueryRowContext(ctx, "SELECT category, name_en FROM poi WHERE wikidata_id = ?", qid).Scan(&category, &name)
	if err == nil {
		fmt.Printf("Found in 'poi' table:\n")
		fmt.Printf("   Name:     %s\n", name)
		fmt.Printf("   Category: %s\n", category)
	} else {
		fmt.Printf("Not found in 'poi' table (%v)\n", err)
	}

	// Check seen_entities table
	var instances string
	var created time.Time
	err = db.QueryRowContext(ctx, "SELECT instances, created_at FROM seen_entities WHERE qid = ?", qid).Scan(&instances, &created)
	if err == nil {
		fmt.Printf("Found in 'seen_entities' table:\n")
		fmt.Printf("   Instances:  %s\n", instances)
		fmt.Printf("   Created At: %s\n", created.Format(time.RFC3339))
	} else {
		fmt.Printf("Not found in 'seen_entities' table (%v)\n", err)
	}
	fmt.Println(strings.Repeat("-", 50))
}
