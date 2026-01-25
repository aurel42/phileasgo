package main

import (
	"os"
	"path/filepath"
	"testing"
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

			m := &Manager{}
			got := m.checkPrerequisites()
			if got != tt.want {
				t.Errorf("checkPrerequisites() = %v, want %v", got, tt.want)
			}
		})
	}
}
