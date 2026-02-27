package maintenance

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"time"

	"phileasgo/pkg/db"
	"phileasgo/pkg/model"
	"phileasgo/pkg/store"
)

const msfsPOITableStateKey = "msfs_master_csv_mtime"

// Run executes all maintenance tasks: Import and Pruning.
// It uses the provided logger or falls back to default.
// It blocks until completion.
func Run(ctx context.Context, s store.Store, d *db.DB, csvPath string) error {
	slog.Info("Starting database maintenance...")

	if err := importMSFS(ctx, s, csvPath); err != nil {
		slog.Error("MSFS Import failed", "error", err)
		// We don't stop startup for import failure, but we log it.
	} else {
		slog.Info("MSFS Import check completed")
	}

	if err := pruneCache(ctx, d); err != nil {
		slog.Error("Cache pruning failed", "error", err)
	} else {
		slog.Info("Cache pruning completed")
	}

	return nil
}

// importMSFS imports POIs from a CSV file conditional on modification time.
// Moved from pkg/importer.
func importMSFS(ctx context.Context, s store.Store, csvPath string) error {
	info, err := os.Stat(csvPath)
	if os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to import
	}
	if err != nil {
		return fmt.Errorf("failed to stat csv: %w", err)
	}

	fileMTime := info.ModTime().UTC().Format(time.RFC3339)

	// Check stored state
	storedMTime, found := s.GetState(ctx, msfsPOITableStateKey)
	if found && storedMTime == fileMTime {
		return nil // Up to date
	}

	slog.Info("Importing MSFS POIs from CSV...", "path", csvPath)

	f, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("failed to open csv: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)

	// Headers: Type,Name,Ident,Latitude,Longitude,Elevation,...
	headers, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	// Handle potential BOM (Byte Order Mark) at start of file
	if len(headers) > 0 {
		// Check for UTF-8 BOM \xef\xbb\xbf
		if len(headers[0]) >= 3 && headers[0][:3] == "\xef\xbb\xbf" {
			headers[0] = headers[0][3:]
		}
	}

	slog.Info("CSV Headers parsed", "headers", headers)

	// Map headers to indices
	idxMap := make(map[string]int)
	for i, h := range headers {
		idxMap[h] = i
	}
	slog.Debug("CSV Header Map", "idxMap", idxMap)

	// Clear existing rows before re-import to prevent duplicates.
	// The table is fully derived from the CSV, so a full replace is safe.
	if err := s.ClearMSFSPOIs(ctx); err != nil {
		return fmt.Errorf("failed to clear msfs_poi: %w", err)
	}

	count, err := processMSFSRows(ctx, s, reader, idxMap)
	if err != nil {
		return err
	}

	slog.Info("Imported MSFS POIs", "count", count)

	// Update State
	if err := s.SetState(ctx, msfsPOITableStateKey, fileMTime); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	return nil
}

func processMSFSRows(ctx context.Context, s store.Store, reader *csv.Reader, idxMap map[string]int) (int, error) {
	get := func(row []string, col string) string {
		if i, ok := idxMap[col]; ok && i < len(row) {
			return row[i]
		}
		return ""
	}

	count := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, fmt.Errorf("csv read error: %w", err)
		}

		poi := &model.MSFSPOI{
			Type:  get(record, "Type"),
			Name:  get(record, "Name"),
			Ident: get(record, "Ident"),
		}

		if lat, err := strconv.ParseFloat(get(record, "Latitude"), 64); err == nil {
			poi.Lat = lat
		}
		if lon, err := strconv.ParseFloat(get(record, "Longitude"), 64); err == nil {
			poi.Lon = lon
		}
		if elev, err := strconv.ParseFloat(get(record, "Elevation"), 64); err == nil {
			poi.Elevation = elev
		}

		if poi.Lat != 0 || poi.Lon != 0 {
			if err := s.SaveMSFSPOI(ctx, poi); err != nil {
				return count, fmt.Errorf("failed to save row %d: %w", count, err)
			}
			count++
		}
	}
	return count, nil
}

// pruneCache removes cache entries older than 30 days.
func pruneCache(ctx context.Context, d *db.DB) error {
	// 30 days
	return d.PruneCache(30 * 24 * time.Hour)
}
