package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"phileasgo/pkg/logging"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Regex to capture key=value or key="value with spaces"
var logRegex = regexp.MustCompile(`([a-zA-Z0-9_\-.]+)=(?:"([^"]*)"|([^ ]+))`)

// handleLatestLog returns the last captured log line.
func handleLatestLog(w http.ResponseWriter, r *http.Request) {
	line := logging.GlobalLogCapture.GetLastLine()
	formatted := formatLogLine(line)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"log": formatted,
	}); err != nil {
		// Just log locally, can't return error to client after writing
		fmt.Printf("Failed to write log response: %v\n", err)
	}
}

// formatLogLine parses the raw log line and applies filtering rules.
// Rules: Format time to HH:MM:SS, unwrap msg, sort other params, remove params > 20 chars.
// Output: HH:MM:SS MsgValue (key=value, key=value)
func formatLogLine(raw string) string {
	matches := logRegex.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return raw
	}

	var msg string
	var timeStr string
	var params []string

	for _, m := range matches {
		key := m[1]
		val := m[2]
		if val == "" {
			val = m[3]
		}
		val = strings.TrimSpace(val)

		if key == "time" {
			// Parse RFC3339 time (default for slog)
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				timeStr = t.Format("15:04:05")
			}
			continue
		}

		if key == "level" {
			continue
		}

		if key == "msg" {
			msg = val
			continue
		}

		// Filter long values (User request: remove params longer than 20 chars)
		// We treat value length as the constraint.
		if len(val) > 20 {
			continue
		}

		params = append(params, fmt.Sprintf("%s=%s", key, val))
	}

	if msg == "" {
		return raw
	}

	sort.Strings(params) // deterministic output

	output := msg
	if timeStr != "" {
		output = fmt.Sprintf("%s %s", timeStr, msg)
	}

	if len(params) > 0 {
		return fmt.Sprintf("%s (%s)", output, strings.Join(params, ", "))
	}
	return output
}
