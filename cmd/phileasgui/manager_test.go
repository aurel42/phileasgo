package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestCheckPrerequisites(t *testing.T) {
	// Setup temp dir for testing
	tempDir, err := os.MkdirTemp("", "phileasgui_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Save original CWD and restore after test
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	// Switch to temp dir
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		setupFiles []string
		setupDirs  []string
		want       bool
	}{
		{
			name:       "Empty Directory",
			setupFiles: []string{},
			setupDirs:  []string{},
			want:       false,
		},
		{
			name:       "Data Only",
			setupFiles: []string{},
			setupDirs:  []string{"data"},
			want:       false,
		},
		{
			name:       "Env Only",
			setupFiles: []string{".env"},
			setupDirs:  []string{},
			want:       false,
		},
		{
			name:       "Data and Env",
			setupFiles: []string{".env"},
			setupDirs:  []string{"data"},
			want:       true,
		},
		{
			name:       "Data and EnvLocal",
			setupFiles: []string{".env.local"},
			setupDirs:  []string{"data"},
			want:       true,
		},
		{
			name:       "Data and Both Envs",
			setupFiles: []string{".env", ".env.local"},
			setupDirs:  []string{"data"},
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up current dir for each subtest
			files, _ := filepath.Glob("*")
			for _, f := range files {
				os.RemoveAll(f)
			}

			// Setup
			for _, dir := range tt.setupDirs {
				os.Mkdir(dir, 0755)
			}
			for _, file := range tt.setupFiles {
				os.WriteFile(file, []byte(""), 0644)
			}

			m := &Manager{serverAddr: "localhost:1920"}
			got := m.checkPrerequisites()
			if got != tt.want {
				t.Errorf("checkPrerequisites() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManager_Stop(t *testing.T) {
	// 1. Setup Mock Server
	var wg sync.WaitGroup
	wg.Add(1)

	shutdownReceived := false

	handler := http.NewServeMux()
	handler.HandleFunc("/api/shutdown", func(w http.ResponseWriter, r *http.Request) {
		shutdownReceived = true
		wg.Done()
	})
	server := &http.Server{Handler: handler}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	serverAddr := ln.Addr().String()

	go func() {
		if err := server.Serve(ln); err != http.ErrServerClosed {
			t.Logf("Mock server failed to serve: %v", err)
		}
	}()
	defer server.Shutdown(context.Background())

	// 2. Setup Manager with dummy process
	// We need a real process so Process field is not nil
	cmd := exec.Command("cmd", "/c", "timeout 5")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start dummy cmd: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	mgr := &Manager{
		serverCmd:  cmd,
		serverAddr: serverAddr,
		logFunc:    func(s string) { fmt.Println(s) },
	}

	// 3. Run Stop()
	mgr.Stop()

	// 4. Verification
	// Wait for handler or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
		if !shutdownReceived {
			t.Error("Stop() completed but shutdown handler wasn't supposedly triggered?")
		}
	case <-time.After(3 * time.Second):
		t.Error("Timed out waiting for shutdown request")
	}
}

func TestManager_ResolveAddr(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{":1920", "127.0.0.1:1920"},
		{"localhost:1920", "127.0.0.1:1920"},
		{"127.0.0.1:1920", "127.0.0.1:1920"},
		{"192.168.1.1:1920", "192.168.1.1:1920"},
	}

	for _, tt := range tests {
		m := &Manager{serverAddr: tt.addr}
		got := m.resolveAddr()
		if got != tt.want {
			t.Errorf("resolveAddr(%s) = %s, want %s", tt.addr, got, tt.want)
		}
	}
}

func TestManager_ServerStatus(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("0.1.0"))
	})
	server := &http.Server{Handler: handler}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	serverAddr := ln.Addr().String()
	go server.Serve(ln)
	defer server.Shutdown(context.Background())

	m := &Manager{serverAddr: serverAddr}

	if !m.isServerRunning() {
		t.Error("isServerRunning() returned false")
	}
	if !m.isServerReady() {
		t.Error("isServerReady() returned false")
	}

	// Test failure cases
	mFail := &Manager{serverAddr: "127.0.0.1:1"} // Unlikely to be a server there
	if mFail.isServerRunning() {
		t.Error("isServerRunning() should be false for invalid port")
	}
	if mFail.isServerReady() {
		t.Error("isServerReady() should be false for invalid port")
	}
}
