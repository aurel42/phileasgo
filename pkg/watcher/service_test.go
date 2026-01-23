package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewService(t *testing.T) {
	tests := []struct {
		name    string
		paths   []string
		wantLen int
	}{
		{
			name:    "Default Paths",
			paths:   []string{},
			wantLen: 1,
		},
		{
			name:    "Custom Paths",
			paths:   []string{"path1", "path2"},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewService(tt.paths)
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}
			if len(s.paths) != tt.wantLen {
				t.Errorf("len(s.paths) = %v, want %v", len(s.paths), tt.wantLen)
			}
		})
	}
}

func TestService_CheckNew(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	s, err := NewService([]string{dir1, dir2})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// 1. Initial check - should be empty
	path, found := s.CheckNew()
	if found {
		t.Errorf("Initial CheckNew() found file: %s", path)
	}

	// 2. Add file to dir1
	time.Sleep(10 * time.Millisecond) // Ensure modTime is after lastChecked
	file1 := filepath.Join(dir1, "shot1.png")
	if err := os.WriteFile(file1, []byte("test1"), 0644); err != nil {
		t.Fatal(err)
	}

	path, found = s.CheckNew()
	if !found {
		t.Error("CheckNew() did not find file1")
	}
	if path != file1 {
		t.Errorf("CheckNew() = %s, want %s", path, file1)
	}

	// 3. Add file to dir2 (older) and dir1 (newer)
	// We need to be careful with sleep to ensure distinct timestamps
	time.Sleep(100 * time.Millisecond)
	fileOld := filepath.Join(dir2, "old.jpg")
	if err := os.WriteFile(fileOld, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)
	fileNew := filepath.Join(dir1, "new.jpeg")
	if err := os.WriteFile(fileNew, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	path, found = s.CheckNew()
	if !found {
		t.Error("CheckNew() did not find any file")
	}
	// It should pick the NEWEST one (fileNew)
	if path != fileNew {
		t.Errorf("CheckNew() picked %s, want newest %s", path, fileNew)
	}

	// 4. Repeated check - should be empty
	path, found = s.CheckNew()
	if found {
		t.Errorf("Repeat CheckNew() found file again: %s", path)
	}

	// 5. Ignore non-images
	time.Sleep(10 * time.Millisecond)
	fileTxt := filepath.Join(dir2, "readme.txt")
	if err := os.WriteFile(fileTxt, []byte("text"), 0644); err != nil {
		t.Fatal(err)
	}
	path, found = s.CheckNew()
	if found {
		t.Errorf("CheckNew() matches .txt file: %s", path)
	}
}
