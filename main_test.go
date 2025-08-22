package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewExifTool(t *testing.T) {
	et, err := NewExifTool()
	if err != nil {
		t.Skipf("Skipping test: exiftool not available: %v", err)
	}
	defer et.Close()

	if et.cmd == nil {
		t.Error("NewExifTool() did not initialize cmd")
	}
	if et.stdin == nil {
		t.Error("NewExifTool() did not initialize stdin")
	}
	if et.stdout == nil {
		t.Error("NewExifTool() did not initialize stdout")
	}
}

func TestExifTool_Execute(t *testing.T) {
	et, err := NewExifTool()
	if err != nil {
		t.Skipf("Skipping test: exiftool not available: %v", err)
	}
	defer et.Close()

	// Test with -ver flag to get version
	output, err := et.Execute("-ver")
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if output == "" {
		t.Error("Execute() returned empty output")
	}
}

func TestPhotoMetadata_Unmarshal(t *testing.T) {
	jsonData := []byte(`{
		"title": "test.jpg",
		"photoTakenTime": {
			"timestamp": "1640995200"
		}
	}`)

	var meta photoMetadata
	err := json.Unmarshal(jsonData, &meta)
	if err != nil {
		t.Errorf("Unmarshal() error = %v", err)
	}

	if meta.Title != "test.jpg" {
		t.Errorf("Title = %v, want test.jpg", meta.Title)
	}

	if meta.PhotoTakenTime.Timestamp != "1640995200" {
		t.Errorf("Timestamp = %v, want 1640995200", meta.PhotoTakenTime.Timestamp)
	}
}

func TestFindFileWithFallbacks(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create a test file
	testFile := "test_image.jpg"
	testPath := filepath.Join(tempDir, testFile)
	if err := os.WriteFile(testPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test finding the original file
	got := findFileWithFallbacks(tempDir, testFile)
	if got != testPath {
		t.Errorf("findFileWithFallbacks() = %v, want %v", got, testPath)
	}

	// Test with non-existent file
	got = findFileWithFallbacks(tempDir, "nonexistent.jpg")
	if got != "" {
		t.Errorf("findFileWithFallbacks() with non-existent file = %v, want empty string", got)
	}
}

func TestCheckTruncatedName(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create a file with a long name that would be truncated
	longName := "this_is_a_very_long_filename_that_would_be_truncated_by_google_takeout_system.jpg"
	truncatedName := longName[:48] // Truncate to 48 characters
	path := filepath.Join(tempDir, truncatedName)

	if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test with original long name that should find the truncated version
	got := checkTruncatedName(tempDir, longName)
	if got != path {
		t.Errorf("checkTruncatedName() = %v, want %v", got, path)
	}

	// Test with name that shouldn't match (too short to be truncated)
	got = checkTruncatedName(tempDir, "short.jpg")
	if got != "" {
		t.Errorf("checkTruncatedName() with short name = %v, want empty string", got)
	}
}

func TestGetDateFromTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		timestamp int64
		wantYear  string
		wantMonth string
		wantDay   string
	}{
		{
			name:      "New Year 2023",
			timestamp: 1672531200, // 2023-01-01 00:00:00 UTC
			wantYear:  "2023",
			wantMonth: "01",
			wantDay:   "01",
		},
		{
			name:      "Christmas 2022",
			timestamp: 1671926400, // 2022-12-25 00:00:00 UTC
			wantYear:  "2022",
			wantMonth: "12",
			wantDay:   "25",
		},
		{
			name:      "Unix Epoch",
			timestamp: 0, // 1970-01-01 00:00:00 UTC
			wantYear:  "1970",
			wantMonth: "01",
			wantDay:   "01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotYear, gotMonth, gotDay := getDateFromTimestamp(tt.timestamp)
			if gotYear != tt.wantYear {
				t.Errorf("getDateFromTimestamp() year = %v, want %v", gotYear, tt.wantYear)
			}
			if gotMonth != tt.wantMonth {
				t.Errorf("getDateFromTimestamp() month = %v, want %v", gotMonth, tt.wantMonth)
			}
			if gotDay != tt.wantDay {
				t.Errorf("getDateFromTimestamp() day = %v, want %v", gotDay, tt.wantDay)
			}
		})
	}
}

func TestProgressBar(t *testing.T) {
	pb := newProgressBar(100)

	if pb.total != 100 {
		t.Errorf("newProgressBar() total = %v, want 100", pb.total)
	}

	if pb.current != 0 {
		t.Errorf("newProgressBar() current = %v, want 0", pb.current)
	}

	// Test update
	pb.update()
	if pb.current != 1 {
		t.Errorf("After update() current = %v, want 1", pb.current)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "Seconds",
			duration: 45 * time.Second,
			want:     "45s",
		},
		{
			name:     "Minutes and seconds",
			duration: 2*time.Minute + 30*time.Second,
			want:     "2m30s",
		},
		{
			name:     "Hours and minutes",
			duration: 1*time.Hour + 15*time.Minute,
			want:     "1h15m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsMediaFile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{"image.jpg", true},
		{"image.JPG", true},
		{"video.mp4", true},
		{"video.MP4", true},
		{"photo.heic", true},
		{"document.pdf", false},
		{"text.txt", false},
		{"", false},
		{"noextension", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := isMediaFile(tt.filename)
			if got != tt.want {
				t.Errorf("isMediaFile(%v) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestEnsureDirectory(t *testing.T) {
	tempDir := t.TempDir()
	testPath := filepath.Join(tempDir, "new", "nested", "directory")

	// Test dry run
	err := ensureDirectory(testPath, true)
	if err != nil {
		t.Errorf("ensureDirectory() dry run error = %v", err)
	}

	// Directory should not exist after dry run
	if _, err := os.Stat(testPath); !os.IsNotExist(err) {
		t.Error("ensureDirectory() dry run created directory when it shouldn't have")
	}

	// Test actual creation
	err = ensureDirectory(testPath, false)
	if err != nil {
		t.Errorf("ensureDirectory() error = %v", err)
	}

	// Directory should exist now
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Error("ensureDirectory() failed to create directory")
	}
}
