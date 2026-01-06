package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"sort"
	"sync"
	"time"
)

type LatencyStats struct {
	DNS        time.Duration
	Connect    time.Duration
	FirstByte  time.Duration
	Total      time.Duration
	StatusCode int
	Error      error
}

func main() {
	baseURL := flag.String("url", "http://localhost:1920", "Base URL of the API")
	datasetSize := flag.Int("n", 20, "Number of requests per endpoint")
	concurrency := flag.Int("c", 1, "Concurrency level (1 = sequential)")
	flag.Parse()

	endpoints := []string{
		"/", // Static file serving
		"/health",
		"/api/telemetry",
		"/api/version",
		"/api/config",
		"/api/stats",
		"/api/wikidata/cache",
		"/api/pois/tracked",
		"/api/map/visibility",
		"/api/audio/status",
		"/api/narrator/status",
	}

	fmt.Printf("Benchmarking %s with N=%d, C=%d\n\n", *baseURL, *datasetSize, *concurrency)

	for _, ep := range endpoints {
		benchmarkEndpoint(*baseURL+ep, *datasetSize, *concurrency)
	}
}

func benchmarkEndpoint(url string, n, concurrency int) {
	results := make([]LatencyStats, n)
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	startTotal := time.Now()

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = measureLatency(url)
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTotal)

	// Analyze
	var totalDurations []time.Duration
	var fbDurations []time.Duration
	var errorsCount int

	for _, r := range results {
		if r.Error != nil {
			errorsCount++
			continue
		}
		totalDurations = append(totalDurations, r.Total)
		fbDurations = append(fbDurations, r.FirstByte)
	}

	fmt.Printf("Endpoint: %s\n", url)
	if errorsCount > 0 {
		fmt.Printf("  Errors: %d/%d\n", errorsCount, n)
	}
	if len(totalDurations) == 0 {
		fmt.Println("  No successful requests.")
		return
	}

	sort.Slice(totalDurations, func(i, j int) bool { return totalDurations[i] < totalDurations[j] })
	sort.Slice(fbDurations, func(i, j int) bool { return fbDurations[i] < fbDurations[j] })

	avgTotal := average(totalDurations)
	avgFB := average(fbDurations)

	fmt.Printf("  Requests: %d | Time: %v | RPS: %.2f\n", n, duration.Round(time.Millisecond), float64(n)/duration.Seconds())
	fmt.Printf("  Latency (Total)   : Min %v | Avg %v | Max %v\n", totalDurations[0], avgTotal, totalDurations[len(totalDurations)-1])
	fmt.Printf("  Latency (TTFB)    : Min %v | Avg %v | Max %v\n", fbDurations[0], avgFB, fbDurations[len(fbDurations)-1])
	fmt.Println()
}

func measureLatency(url string) LatencyStats {
	var stats LatencyStats
	var start, dnsStart, connStart, wroteRequest time.Time

	req, _ := http.NewRequest("GET", url, http.NoBody)
	trace := &httptrace.ClientTrace{
		DNSStart: func(dsi httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone: func(ddi httptrace.DNSDoneInfo) {
			stats.DNS = time.Since(dnsStart)
		},
		ConnectStart: func(network, addr string) { connStart = time.Now() },
		ConnectDone: func(network, addr string, err error) {
			stats.Connect = time.Since(connStart)
		},
		WroteRequest: func(wri httptrace.WroteRequestInfo) {
			wroteRequest = time.Now()
		},
		GotFirstResponseByte: func() {
			stats.FirstByte = time.Since(wroteRequest)
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	start = time.Now()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		stats.Error = err
		return stats
	}
	defer resp.Body.Close()

	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		stats.Error = err
		return stats
	}
	stats.Total = time.Since(start)
	stats.StatusCode = resp.StatusCode

	return stats
}

func average(d []time.Duration) time.Duration {
	if len(d) == 0 {
		return 0
	}
	var sum time.Duration
	for _, v := range d {
		sum += v
	}
	return sum / time.Duration(len(d))
}
