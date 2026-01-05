package request

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"phileasgo/pkg/cache"
	"phileasgo/pkg/db"
	"phileasgo/pkg/tracker"
)

func TestGet_Sequential(t *testing.T) {
	// Mock Server using simple handler that sleeps to prove sequential execution
	var conc int32
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&conc, 1)
		defer atomic.AddInt32(&conc, -1)

		if current > 1 {
			// If concurrent > 1, the queue didn't work (for same provider)
			// But note: different providers run in parallel. Svr has one URL host.
			t.Errorf("Concurrency detected! Expected sequential.")
		}
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(200)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Logf("Write failed: %v", err)
		}
	}))
	defer svr.Close()

	// Setup Client
	tempDir := t.TempDir()
	d, err := db.Init(filepath.Join(tempDir, "client_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	c := cache.NewSQLiteCache(d)
	tr := tracker.New()
	client := New(c, tr)

	// Fire 3 requests
	for i := 0; i < 3; i++ {
		go func() {
			_, err := client.Get(context.Background(), svr.URL, "test_key")
			if err != nil {
				t.Errorf("Get failed: %v", err)
			}
		}()
	}

	// wait for them (simple sleep for test)
	time.Sleep(500 * time.Millisecond)
}

func TestGet_Retry(t *testing.T) {
	attempts := 0
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(429) // Too Many Requests
			return
		}
		w.WriteHeader(200)
		if _, err := w.Write([]byte("success")); err != nil {
			t.Logf("Write failed: %v", err)
		}
	}))
	defer svr.Close()

	tempDir := t.TempDir()
	d, err := db.Init(filepath.Join(tempDir, "retry_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	client := New(cache.NewSQLiteCache(d), tracker.New())

	body, err := client.Get(context.Background(), svr.URL, "")
	if err != nil {
		t.Fatalf("Expected success after retry, got error: %v", err)
	}
	if string(body) != "success" {
		t.Errorf("Expected 'success', got '%s'", string(body))
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}
