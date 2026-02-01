package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"phileasgo/pkg/logging"
)

// handleLatestLog returns the last captured log line.
func handleLatestLog(w http.ResponseWriter, r *http.Request) {
	line := logging.GlobalEventCapture.GetLastLine()
	// Trip events are already formatted for humans "[2006-01-02 15:04:05] [type] Title"
	// We return it as is.
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"log": line,
	}); err != nil {
		// Just log locally, can't return error to client after writing
		fmt.Printf("Failed to write log response: %v\n", err)
	}
}
