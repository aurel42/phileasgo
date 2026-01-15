package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	reGen   = regexp.MustCompile(`time=(\S+) .+Generating script.+ name="?([^"]+)"? .+relative_dominance=(\S+) predicted_delay=(\S+)`)
	reStats = regexp.MustCompile(`time=(\S+) .+Narration stats.+name="([^"]+)".+requested_len=(\d+)`)
)

type RateTracker struct {
	rates []float64
}

func (r *RateTracker) Add(durationMs float64, words int) {
	if words > 0 {
		rate := durationMs / float64(words)
		r.rates = append(r.rates, rate)
		if len(r.rates) > 10 {
			r.rates = r.rates[1:]
		}
	}
}

func (r *RateTracker) Median() float64 {
	if len(r.rates) == 0 {
		return 0 // No data yet
	}
	sorted := make([]float64, len(r.rates))
	copy(sorted, r.rates)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

func (r *RateTracker) Predict(targetWords int) time.Duration {
	if len(r.rates) == 0 {
		return 60 * time.Second // Same as old algorithm's fallback
	}
	return time.Duration(r.Median()*float64(targetWords)) * time.Millisecond
}

func cleanName(n string) string { return strings.Trim(n, `"`) }

type GenEvent struct {
	Name     string
	Time     time.Time
	Strategy string
	OldPred  time.Duration
}

type StatsEvent struct {
	Name      string
	Time      time.Time
	Requested int
}

type POIEvent struct {
	Name      string
	GenTime   time.Time
	Strategy  string
	OldPred   time.Duration
	Actual    time.Duration
	Requested int
}

func parseLog(path string) ([]GenEvent, []StatsEvent) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Error opening log:", err)
		return nil, nil
	}
	defer file.Close()

	var genEvents []GenEvent
	var statsEvents []StatsEvent

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if match := reGen.FindStringSubmatch(line); match != nil {
			ts, _ := time.Parse("2006-01-02T15:04:05.999-07:00", match[1])
			pred, _ := time.ParseDuration(match[4])
			genEvents = append(genEvents, GenEvent{cleanName(match[2]), ts, match[3], pred})
		}
		if match := reStats.FindStringSubmatch(line); match != nil {
			ts, _ := time.Parse("2006-01-02T15:04:05.999-07:00", match[1])
			var req int
			if _, err := fmt.Sscanf(match[3], "%d", &req); err == nil {
				statsEvents = append(statsEvents, StatsEvent{cleanName(match[2]), ts, req})
			}
		}
	}
	return genEvents, statsEvents
}

func matchEvents(genEvents []GenEvent, statsEvents []StatsEvent) []POIEvent {
	var events []POIEvent
	usedGen := make(map[int]bool)

	for _, stats := range statsEvents {
		lastGenIdx := -1
		for i := len(genEvents) - 1; i >= 0; i-- {
			if usedGen[i] {
				continue
			}
			if genEvents[i].Name == stats.Name && genEvents[i].Time.Before(stats.Time) {
				lastGenIdx = i
				break
			}
		}
		if lastGenIdx >= 0 {
			gen := genEvents[lastGenIdx]
			events = append(events, POIEvent{
				Name:      gen.Name,
				GenTime:   gen.Time,
				Strategy:  gen.Strategy,
				OldPred:   gen.OldPred,
				Actual:    stats.Time.Sub(gen.Time),
				Requested: stats.Requested,
			})
			usedGen[lastGenIdx] = true
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].GenTime.Before(events[j].GenTime) })
	return events
}

func runSimulation(events []POIEvent) {
	tracker := &RateTracker{}

	fmt.Println("\n=== SIMULATION: Predict using actual requested_len (as if we SET it) ===")
	fmt.Printf("%-10s | %-20s | %-5s | %-4s | %-6s | %-6s | %-6s | %-6s | %-6s\n",
		"Time", "Name", "Words", "Rate", "Actual", "OldPrd", "OldDif", "NewPrd", "NewDif")

	var totalOldErr, totalNewErr time.Duration
	var totalOldUnder, totalNewUnder time.Duration

	for _, e := range events {
		rate := tracker.Median()
		newPred := tracker.Predict(e.Requested)

		oldDiff := e.OldPred - e.Actual
		newDiff := newPred - e.Actual
		absOld := oldDiff
		if absOld < 0 {
			absOld = -absOld
		}
		absNew := newDiff
		if absNew < 0 {
			absNew = -absNew
		}
		totalOldErr += absOld
		totalNewErr += absNew

		oldWait := -oldDiff
		if oldWait < 0 {
			oldWait = 0
		}
		newWait := -newDiff
		if newWait < 0 {
			newWait = 0
		}
		totalOldUnder += oldWait
		totalNewUnder += newWait

		fmt.Printf("%-10s | %-20s | %-5d | %-4.0f | %-6v | %-6v | %+6v | %-6v | %+6v\n",
			e.GenTime.Format("15:04:05"), truncate(e.Name, 20), e.Requested, rate,
			e.Actual.Round(time.Second), e.OldPred.Round(time.Second), oldDiff.Round(time.Second),
			newPred.Round(time.Second), newDiff.Round(time.Second))

		tracker.Add(float64(e.Actual.Milliseconds()), e.Requested)
	}

	n := len(events)
	if n > 0 {
		fmt.Println(strings.Repeat("-", 95))
		fmt.Printf("Avg Error (all):        Old=%v New=%v\n",
			(totalOldErr / time.Duration(n)).Round(time.Second),
			(totalNewErr / time.Duration(n)).Round(time.Second))
		fmt.Printf("Avg Waiting Cost:       Old=%v New=%v (max(0, -diff) per POI)\n",
			(totalOldUnder / time.Duration(n)).Round(time.Second),
			(totalNewUnder / time.Duration(n)).Round(time.Second))
	}
}

func main() {
	logPath := "logs/server.log"
	if len(os.Args) > 1 {
		logPath = os.Args[1]
	}

	genEvents, statsEvents := parseLog(logPath)
	events := matchEvents(genEvents, statsEvents)
	runSimulation(events)
}

func truncate(s string, limit int) string {
	if len(s) > limit {
		return s[:limit-3] + "..."
	}
	return s
}
