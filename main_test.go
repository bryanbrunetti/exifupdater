package main

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewExifTool tests the creation of a new ExifTool instance
func TestNewExifTool(t *testing.T) {
	et, err := NewExifTool()
	if err != nil {
		t.Fatalf("Failed to create ExifTool: %v", err)
	}
	defer et.Close()

	// Verify the command was created with correct arguments
	expectedArgs := []string{"exiftool", "-stay_open", "True", "-@", "-"}
	if len(et.cmd.Args) != len(expectedArgs) {
		t.Fatalf("Expected %d args, got %d", len(expectedArgs), len(et.cmd.Args))
	}
	for i, arg := range expectedArgs {
		if et.cmd.Args[i] != arg {
			t.Errorf("Expected arg %d to be %q, got %q", i, arg, et.cmd.Args[i])
		}
	}
}

// TestExifTool_Execute tests the Execute method
func TestExifTool_Execute(t *testing.T) {
	et, err := NewExifTool()
	if err != nil {
		t.Fatalf("Failed to create ExifTool: %v", err)
	}
	defer et.Close()

	// Test a simple command - use a command that should work without side effects
	output, err := et.Execute("-ver")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Clean up any whitespace
	output = strings.TrimSpace(output)

	// If output is empty, try to get more information
	if output == "" {
		t.Log("First attempt returned empty output, trying with debug...")

		// Try to get version using a different approach
		output, err = et.Execute("-ver", "-ver")
		if err != nil {
			t.Fatalf("Second attempt failed: %v", err)
		}
		output = strings.TrimSpace(output)
	}

	// Check if we got any output
	if output == "" {
		// Try a direct command execution to see if exiftool works at all
		cmd := exec.Command("exiftool", "-ver")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Direct exiftool command failed: %v\nOutput: %s", err, out)
		}
		t.Fatalf("ExifTool returned empty output but direct command works. Output: %q", strings.TrimSpace(string(out)))
	}

	// Check if output looks like a version number (e.g., "12.00" or similar)
	if !strings.ContainsAny(output, ".") && !strings.ContainsAny(output, "0123456789") {
		t.Errorf("Output %q doesn't look like a version number (missing numbers)", output)
	}
}

// TestPhotoMetadata_Unmarshal tests JSON unmarshaling of photoMetadata
func TestPhotoMetadata_Unmarshal(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    photoMetadata
		wantErr bool
	}{
		{
			name: "valid metadata",
			json: `{"title":"test.jpg","photoTakenTime":{"timestamp":"1617235200"}}`,
			want: photoMetadata{
				Title: "test.jpg",
				PhotoTakenTime: struct {
					Timestamp string `json:"timestamp"`
				}{
					Timestamp: "1617235200",
				},
			},
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			json:    `{invalid json}`,
			want:    photoMetadata{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got photoMetadata
			err := json.Unmarshal([]byte(tt.json), &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Title != tt.want.Title {
				t.Errorf("Expected Title %q, got %q", tt.want.Title, got.Title)
			}
			if !tt.wantErr && got.PhotoTakenTime.Timestamp != tt.want.PhotoTakenTime.Timestamp {
				t.Errorf("Expected Timestamp %q, got %q", tt.want.PhotoTakenTime.Timestamp, got.PhotoTakenTime.Timestamp)
			}
		})
	}
}

// TestFindFileWithFallbacks tests the findFileWithFallbacks function
func TestFindFileWithFallbacks(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Create test files
	testFiles := []string{
		"test.jpg",
		"TEST.JPG",
		"long_filename_that_would_be_truncated.jpg",
		"file_with_quotes'.jpg",
	}

	for _, f := range testFiles {
		path := filepath.Join(tempDir, f)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
	}

	tests := []struct {
		name     string
		dir      string
		filename string
		want     string
	}{
		{
			name:     "exact match",
			dir:      tempDir,
			filename: "test.jpg",
			want:     filepath.Join(tempDir, "test.jpg"),
		},
		{
			name:     "case insensitive match",
			dir:      tempDir,
			filename: "TEST.JPG",
			want:     filepath.Join(tempDir, "TEST.JPG"),
		},
		{
			name:     "truncated filename",
			dir:      tempDir,
			filename: "long_filename_that_would_be_truncated.jpg",
			want:     filepath.Join(tempDir, "long_filename_that_would_be_truncated.jpg"),
		},
		{
			name:     "file with quotes",
			dir:      tempDir,
			filename: "file_with_quotes'.jpg",
			want:     filepath.Join(tempDir, "file_with_quotes'.jpg"),
		},
		{
			name:     "non-existent file",
			dir:      tempDir,
			filename: "nonexistent.jpg",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findFileWithFallbacks(tt.dir, tt.filename)
			if got != tt.want {
				t.Errorf("findFileWithFallbacks() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCheckTruncatedName tests the checkTruncatedName function
func TestCheckTruncatedName(t *testing.T) {
	tempDir := t.TempDir()
	longName := "this_is_a_very_long_filename_that_will_be_truncated.jpg"
	truncatedName := longName[:48] + ".jpg"
	path := filepath.Join(tempDir, truncatedName)

	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file %s: %v", path, err)
	}

	// Test with length that should match
	got := checkTruncatedName(tempDir, strings.TrimSuffix(longName, ".jpg"), ".jpg", longName, 48)
	if got != path {
		t.Errorf("checkTruncatedName() = %v, want %v", got, path)
	}

	// Test with length that shouldn't match
	got = checkTruncatedName(tempDir, "short.jpg", ".jpg", "short.jpg", 10)
	if got != "" {
		t.Errorf("checkTruncatedName() with short name = %v, want empty string", got)
	}
}

// TestGetDateFromTimestamp tests the getDateFromTimestamp function
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
			name:      "Mid year date",
			timestamp: 1688169600, // 2023-07-01 00:00:00 UTC
			wantYear:  "2023",
			wantMonth: "07",
			wantDay:   "01",
		},
		{
			name:      "December date",
			timestamp: 1701388800, // 2023-12-01 00:00:00 UTC
			wantYear:  "2023",
			wantMonth: "12",
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

// TestEnsureDirectory tests the ensureDirectory function
func TestEnsureDirectory(t *testing.T) {
	tempDir := t.TempDir()
	testDir := filepath.Join(tempDir, "test", "nested", "directory")

	// Test creating directory (not dry run)
	err := ensureDirectory(testDir, false)
	if err != nil {
		t.Fatalf("ensureDirectory() error = %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Errorf("Directory was not created: %s", testDir)
	}

	// Test dry run mode
	testDir2 := filepath.Join(tempDir, "dry", "run", "test")
	err = ensureDirectory(testDir2, true)
	if err != nil {
		t.Fatalf("ensureDirectory() dry run error = %v", err)
	}

	// Verify directory was NOT created in dry run
	if _, err := os.Stat(testDir2); !os.IsNotExist(err) {
		t.Errorf("Directory should not exist in dry run mode: %s", testDir2)
	}
}

// TestMoveFile tests the moveFile function
func TestMoveFile(t *testing.T) {
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, "source.txt")
	destFile := filepath.Join(tempDir, "dest", "moved.txt")

	// Create source file
	if err := os.WriteFile(sourceFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Test moving file (not dry run)
	err := moveFile(sourceFile, destFile, false)
	if err != nil {
		t.Fatalf("moveFile() error = %v", err)
	}

	// Verify file was moved
	if _, err := os.Stat(sourceFile); !os.IsNotExist(err) {
		t.Errorf("Source file should not exist after move: %s", sourceFile)
	}
	if _, err := os.Stat(destFile); os.IsNotExist(err) {
		t.Errorf("Destination file should exist after move: %s", destFile)
	}

	// Test dry run mode
	sourceFile2 := filepath.Join(tempDir, "source2.txt")
	destFile2 := filepath.Join(tempDir, "dest2", "moved2.txt")
	if err := os.WriteFile(sourceFile2, []byte("test content 2"), 0644); err != nil {
		t.Fatalf("Failed to create second source file: %v", err)
	}

	err = moveFile(sourceFile2, destFile2, true)
	if err != nil {
		t.Fatalf("moveFile() dry run error = %v", err)
	}

	// Verify file was NOT moved in dry run
	if _, err := os.Stat(sourceFile2); os.IsNotExist(err) {
		t.Errorf("Source file should still exist in dry run mode: %s", sourceFile2)
	}
	if _, err := os.Stat(destFile2); !os.IsNotExist(err) {
		t.Errorf("Destination file should not exist in dry run mode: %s", destFile2)
	}
}

// TestCreateSymlink tests the createSymlink function
func TestCreateSymlink(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "target.txt")
	linkFile := filepath.Join(tempDir, "link.txt")

	// Create target file
	if err := os.WriteFile(targetFile, []byte("target content"), 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	// Test creating symlink (not dry run)
	err := createSymlink("target.txt", linkFile, false)
	if err != nil {
		t.Fatalf("createSymlink() error = %v", err)
	}

	// Verify symlink was created
	if _, err := os.Lstat(linkFile); os.IsNotExist(err) {
		t.Errorf("Symlink should exist: %s", linkFile)
	}

	// Test dry run mode
	linkFile2 := filepath.Join(tempDir, "link2.txt")
	err = createSymlink("target.txt", linkFile2, true)
	if err != nil {
		t.Fatalf("createSymlink() dry run error = %v", err)
	}

	// Verify symlink was NOT created in dry run
	if _, err := os.Lstat(linkFile2); !os.IsNotExist(err) {
		t.Errorf("Symlink should not exist in dry run mode: %s", linkFile2)
	}
}

// TestMain sets up and tears down any test dependencies
func TestMain(m *testing.M) {
	// Check if exiftool is installed
	_, err := exec.LookPath("exiftool")
	if err != nil {
		// Skip tests if exiftool is not installed
		log.Printf("Skipping tests: exiftool not found in PATH")
		os.Exit(0)
	}

	// Run tests
	code := m.Run()
	os.Exit(code)
}
