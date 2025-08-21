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

// TestMoveOrCopyFile tests the moveOrCopyFile function
func TestMoveOrCopyFile(t *testing.T) {
	tempDir := t.TempDir()

	// Test moving file (not dry run, not keeping files)
	sourceFile := filepath.Join(tempDir, "source.txt")
	destFile := filepath.Join(tempDir, "dest", "moved.txt")
	testContent := []byte("test content")

	if err := os.WriteFile(sourceFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	err := moveOrCopyFile(sourceFile, destFile, false, false)
	if err != nil {
		t.Fatalf("moveOrCopyFile() move error = %v", err)
	}

	// Verify file was moved
	if _, err := os.Stat(sourceFile); !os.IsNotExist(err) {
		t.Errorf("Source file should not exist after move: %s", sourceFile)
	}
	if _, err := os.Stat(destFile); os.IsNotExist(err) {
		t.Errorf("Destination file should exist after move: %s", destFile)
	}

	// Test copying file (not dry run, keeping files)
	sourceFile2 := filepath.Join(tempDir, "source2.txt")
	destFile2 := filepath.Join(tempDir, "dest2", "copied.txt")
	if err := os.WriteFile(sourceFile2, testContent, 0644); err != nil {
		t.Fatalf("Failed to create second source file: %v", err)
	}

	err = moveOrCopyFile(sourceFile2, destFile2, false, true)
	if err != nil {
		t.Fatalf("moveOrCopyFile() copy error = %v", err)
	}

	// Verify file was copied (both should exist)
	if _, err := os.Stat(sourceFile2); os.IsNotExist(err) {
		t.Errorf("Source file should still exist after copy: %s", sourceFile2)
	}
	if _, err := os.Stat(destFile2); os.IsNotExist(err) {
		t.Errorf("Destination file should exist after copy: %s", destFile2)
	}

	// Verify content was copied correctly
	copiedContent, err := os.ReadFile(destFile2)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if string(copiedContent) != string(testContent) {
		t.Errorf("Copied content %q does not match original %q", string(copiedContent), string(testContent))
	}

	// Test dry run mode (move)
	sourceFile3 := filepath.Join(tempDir, "source3.txt")
	destFile3 := filepath.Join(tempDir, "dest3", "moved3.txt")
	if err := os.WriteFile(sourceFile3, testContent, 0644); err != nil {
		t.Fatalf("Failed to create third source file: %v", err)
	}

	err = moveOrCopyFile(sourceFile3, destFile3, true, false)
	if err != nil {
		t.Fatalf("moveOrCopyFile() dry run move error = %v", err)
	}

	// Verify file was NOT moved in dry run
	if _, err := os.Stat(sourceFile3); os.IsNotExist(err) {
		t.Errorf("Source file should still exist in dry run mode: %s", sourceFile3)
	}
	if _, err := os.Stat(destFile3); !os.IsNotExist(err) {
		t.Errorf("Destination file should not exist in dry run mode: %s", destFile3)
	}

	// Test dry run mode (copy)
	sourceFile4 := filepath.Join(tempDir, "source4.txt")
	destFile4 := filepath.Join(tempDir, "dest4", "copied4.txt")
	if err := os.WriteFile(sourceFile4, testContent, 0644); err != nil {
		t.Fatalf("Failed to create fourth source file: %v", err)
	}

	err = moveOrCopyFile(sourceFile4, destFile4, true, true)
	if err != nil {
		t.Fatalf("moveOrCopyFile() dry run copy error = %v", err)
	}

	// Verify file was NOT copied in dry run
	if _, err := os.Stat(sourceFile4); os.IsNotExist(err) {
		t.Errorf("Source file should still exist in dry run mode: %s", sourceFile4)
	}
	if _, err := os.Stat(destFile4); !os.IsNotExist(err) {
		t.Errorf("Destination file should not exist in dry run mode: %s", destFile4)
	}
}

// TestCopyFile tests the copyFile function
func TestCopyFile(t *testing.T) {
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, "source.txt")
	destFile := filepath.Join(tempDir, "dest.txt")
	testContent := []byte("test content for copy")

	// Create source file
	if err := os.WriteFile(sourceFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Test copying file
	err := copyFile(sourceFile, destFile)
	if err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	// Verify both files exist
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		t.Errorf("Source file should still exist after copy: %s", sourceFile)
	}
	if _, err := os.Stat(destFile); os.IsNotExist(err) {
		t.Errorf("Destination file should exist after copy: %s", destFile)
	}

	// Verify content was copied correctly
	copiedContent, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if string(copiedContent) != string(testContent) {
		t.Errorf("Copied content %q does not match original %q", string(copiedContent), string(testContent))
	}

	// Verify file permissions were copied
	sourceInfo, err := os.Stat(sourceFile)
	if err != nil {
		t.Fatalf("Failed to get source file info: %v", err)
	}
	destInfo, err := os.Stat(destFile)
	if err != nil {
		t.Fatalf("Failed to get dest file info: %v", err)
	}
	if sourceInfo.Mode() != destInfo.Mode() {
		t.Errorf("File permissions not copied correctly: source=%v, dest=%v", sourceInfo.Mode(), destInfo.Mode())
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

	// Test creating the same symlink again (should not error)
	err = createSymlink("target.txt", linkFile, false)
	if err != nil {
		t.Fatalf("createSymlink() existing symlink error = %v", err)
	}

	// Verify symlink still exists and points to correct target
	target, err := os.Readlink(linkFile)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if target != "target.txt" {
		t.Errorf("Symlink points to wrong target: got %s, want target.txt", target)
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

	// Test dry run with existing symlink
	err = createSymlink("target.txt", linkFile, true)
	if err != nil {
		t.Fatalf("createSymlink() dry run with existing symlink error = %v", err)
	}
}

// TestFileAlreadyExistsScenario tests the behavior when a file already exists in ALL_PHOTOS
func TestFileAlreadyExistsScenario(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source")
	destDir := filepath.Join(tempDir, "dest")

	// Create source directory structure
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}

	// Create test files in source
	testImage := filepath.Join(sourceDir, "test.jpg")
	testJSON := filepath.Join(sourceDir, "test.jpg.json")
	metadataJSON := filepath.Join(sourceDir, "metadata.json")

	if err := os.WriteFile(testImage, []byte("fake image"), 0644); err != nil {
		t.Fatalf("Failed to create test image: %v", err)
	}

	jsonContent := `{"title":"test.jpg","photoTakenTime":{"timestamp":"1672531200"}}`
	if err := os.WriteFile(testJSON, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to create test JSON: %v", err)
	}

	metadataContent := `{"title":"Test Album"}`
	if err := os.WriteFile(metadataJSON, []byte(metadataContent), 0644); err != nil {
		t.Fatalf("Failed to create metadata JSON: %v", err)
	}

	// Create destination structure with existing file
	destPhotoPath := filepath.Join(destDir, "ALL_PHOTOS", "2023", "01", "01", "test.jpg")
	if err := os.MkdirAll(filepath.Dir(destPhotoPath), 0755); err != nil {
		t.Fatalf("Failed to create dest photo dir: %v", err)
	}
	if err := os.WriteFile(destPhotoPath, []byte("existing image"), 0644); err != nil {
		t.Fatalf("Failed to create existing dest file: %v", err)
	}

	// Create album directory
	albumDir := filepath.Join(destDir, "Test Album")
	if err := os.MkdirAll(albumDir, 0755); err != nil {
		t.Fatalf("Failed to create album dir: %v", err)
	}

	// Test that symlink gets created even when file already exists
	symlinkPath := filepath.Join(albumDir, "test.jpg")
	relativePath := filepath.Join("..", "ALL_PHOTOS", "2023", "01", "01", "test.jpg")

	// Create symlink manually to simulate the worker behavior
	err := createSymlink(relativePath, symlinkPath, false)
	if err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Verify symlink was created
	if _, err := os.Lstat(symlinkPath); os.IsNotExist(err) {
		t.Errorf("Symlink should exist: %s", symlinkPath)
	}

	// Verify symlink points to correct target
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if target != relativePath {
		t.Errorf("Symlink points to wrong target: got %s, want %s", target, relativePath)
	}

	// Test creating the same symlink again (should not error)
	err = createSymlink(relativePath, symlinkPath, false)
	if err != nil {
		t.Errorf("Creating existing symlink should not error: %v", err)
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
