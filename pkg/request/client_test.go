package request

import (
	"context"
	"io"
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
	client := New(c, tr, ClientConfig{})

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
	// Use tiny backoff for test speed
	client := New(cache.NewSQLiteCache(d), tracker.New(), ClientConfig{
		BaseDelay: 10 * time.Millisecond,
		MaxDelay:  50 * time.Millisecond,
		Retries:   5,
	})

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

func TestPost_Retry(t *testing.T) {
	attempts := 0
	expectedBody := "request-payload"
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++

		// Read body and verify it's there
		body, _ := io.ReadAll(r.Body)
		if string(body) != expectedBody {
			t.Errorf("Attempt %d: Expected body '%s', got '%s'", attempts, expectedBody, string(body))
		}

		if attempts < 2 {
			w.WriteHeader(503) // Service Unavailable (Retryable)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("success"))
	}))
	defer svr.Close()

	tempDir := t.TempDir()
	d, err := db.Init(filepath.Join(tempDir, "post_retry_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	client := New(cache.NewSQLiteCache(d), tracker.New(), ClientConfig{
		BaseDelay: 10 * time.Millisecond,
		MaxDelay:  50 * time.Millisecond,
		Retries:   5,
	})

	body, err := client.Post(context.Background(), svr.URL, []byte(expectedBody), "text/plain")
	if err != nil {
		t.Fatalf("Expected success after retry, got error: %v", err)
	}
	if string(body) != "success" {
		t.Errorf("Expected 'success', got '%s'", string(body))
	}
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestPostWithCache(t *testing.T) {
	tests := []struct {
		name       string
		respBody   string
		respStatus int
		mockErr    bool
		wantBody   string
		wantErr    bool
	}{
		{
			name:       "Success",
			respBody:   "posted",
			respStatus: 200,
			wantBody:   "posted",
			wantErr:    false,
		},
		{
			name:       "Server Error",
			respBody:   "",
			respStatus: 500,
			mockErr:    true,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("Expected POST, got %s", r.Method)
				}
				if tt.mockErr {
					w.WriteHeader(tt.respStatus)
					return
				}
				w.WriteHeader(tt.respStatus)
				w.Write([]byte(tt.respBody))
			}))
			defer svr.Close()

			tempDir := t.TempDir()
			d, err := db.Init(filepath.Join(tempDir, "post_test.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			// Use tiny backoff for test speed
			client := New(cache.NewSQLiteCache(d), tracker.New(), ClientConfig{
				BaseDelay: 10 * time.Millisecond,
				MaxDelay:  50 * time.Millisecond,
				Retries:   5,
			})

			got, err := client.PostWithCache(context.Background(), svr.URL, []byte("data"), nil, "cache_key")

			if (err != nil) != tt.wantErr {
				t.Errorf("PostWithCache() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(got) != tt.wantBody {
				t.Errorf("PostWithCache() body = %s, want %s", string(got), tt.wantBody)
			}
		})
	}
}

func TestClient_Integration(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		urlSuffix  string
		body       []byte
		mockStatus int
		mockResp   string
		expectErr  bool
		expectBody string
	}{
		{
			name:       "Get Success",
			method:     "GET",
			urlSuffix:  "/get",
			mockStatus: 200,
			mockResp:   "got it",
			expectErr:  false,
			expectBody: "got it",
		},
		{
			name:       "Get 404",
			method:     "GET",
			urlSuffix:  "/404",
			mockStatus: 404,
			expectErr:  true,
		},
		{
			name:       "Post Success",
			method:     "POST",
			urlSuffix:  "/post",
			body:       []byte("payload"),
			mockStatus: 200,
			mockResp:   "posted",
			expectErr:  false,
			expectBody: "posted",
		},
		{
			name:       "Post 500",
			method:     "POST",
			urlSuffix:  "/err",
			body:       []byte("payload"),
			mockStatus: 500,
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method {
					t.Errorf("Expected method %s, got %s", tt.method, r.Method)
				}
				w.WriteHeader(tt.mockStatus)
				w.Write([]byte(tt.mockResp))
			}))
			defer svr.Close()

			tempDir := t.TempDir()
			d, err := db.Init(filepath.Join(tempDir, "integ_test.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			// Use tiny backoff for test speed
			client := New(cache.NewSQLiteCache(d), tracker.New(), ClientConfig{
				BaseDelay: 10 * time.Millisecond,
				MaxDelay:  50 * time.Millisecond,
				Retries:   5,
			})

			var got []byte
			var reqErr error

			if tt.method == "GET" {
				got, reqErr = client.Get(context.Background(), svr.URL+tt.urlSuffix, "")
			} else {
				got, reqErr = client.Post(context.Background(), svr.URL+tt.urlSuffix, tt.body, "text/plain")
			}

			if (reqErr != nil) != tt.expectErr {
				t.Errorf("Error expectation mismatch: got %v, wantErr %v", reqErr, tt.expectErr)
			}

			if !tt.expectErr && string(got) != tt.expectBody {
				t.Errorf("Body mismatch: got %s, want %s", string(got), tt.expectBody)
			}
		})
	}
}

func TestInvalidURL(t *testing.T) {
	tempDir := t.TempDir()
	d, err := db.Init(filepath.Join(tempDir, "invalid_url.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	client := New(cache.NewSQLiteCache(d), tracker.New(), ClientConfig{})

	_, err = client.Get(context.Background(), "::invalid-url", "")
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}

	_, err = client.Post(context.Background(), "::invalid-url", nil, "")
	if err == nil {
		t.Error("Expected error for invalid URL in Post, got nil")
	}
}
