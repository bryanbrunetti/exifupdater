package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// photoMetadata defines the structure for the relevant fields in the JSON file.
type photoMetadata struct {
	Title          string `json:"title"`
	PhotoTakenTime struct {
		Timestamp string `json:"timestamp"`
	} `json:"photoTakenTime"`
}

// ExifTool manages a persistent exiftool process for efficient batch processing.
type ExifTool struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
}

// NewExifTool starts an exiftool process in stay-open mode.
func NewExifTool() (*ExifTool, error) {
	// Use "-" as the argument to -@ to read from stdin
	cmd := exec.Command("exiftool", "-stay_open", "True", "-@", "-")
	log.Printf("Starting exiftool with args: %v", cmd.Args)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %v", err)
	}

	// Start reading stderr in a goroutine
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("exiftool stderr: %s", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Error reading stderr: %v", err)
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting command: %v", err)
	}

	return &ExifTool{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
	}, nil
}

// Execute sends a command to the running exiftool process.
func (et *ExifTool) Execute(args ...string) (string, error) {
	log.Printf("Executing exiftool with args: %v", args)
	// Write arguments to the process, one per line.
	for _, arg := range args {
		log.Printf("Sending arg: %q", arg)
		if _, err := fmt.Fprintln(et.stdin, arg); err != nil {
			return "", fmt.Errorf("writing arg %q: %v", arg, err)
		}
	}

	// Tell exiftool to execute the command.
	if _, err := fmt.Fprintln(et.stdin, "-execute"); err != nil {
		return "", fmt.Errorf("writing execute command: %v", err)
	}

	// Read the output until the {ready} signal.
	var output strings.Builder
	for et.stdout.Scan() {
		line := et.stdout.Text()
		log.Printf("Read line: %q", line)
		if strings.HasPrefix(line, "{ready}") {
			break
		}
		output.WriteString(line)
		output.WriteString("\n")
	}

	if err := et.stdout.Err(); err != nil {
		return "", fmt.Errorf("reading output: %v", err)
	}

	result := output.String()
	log.Printf("Command result: %q", result)
	return result, nil
}

// Close gracefully shuts down the exiftool process.
func (et *ExifTool) Close() error {
	if _, err := fmt.Fprintln(et.stdin, "-stay_open"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(et.stdin, "False"); err != nil {
		return err
	}
	if err := et.stdin.Close(); err != nil {
		return err
	}
	return et.cmd.Wait()
}

// checkTruncatedName checks if a truncated version of the filename exists
func checkTruncatedName(dir, basename, ext, originalTitle string, length int) string {
	if len(basename) > length {
		truncatedBasename := basename[:length]
		truncatedPath := filepath.Join(dir, truncatedBasename+ext)
		if _, err := os.Stat(truncatedPath); err == nil {
			log.Printf("Found match for '%s' with %d-char truncated name: %s", originalTitle, length, filepath.Base(truncatedPath))
			return truncatedPath
		}
	}
	return ""
}

// findFileWithFallbacks checks for a file's existence using several common naming variations.
func findFileWithFallbacks(dir, originalTitle string) string {
	// 1. Check original filename
	originalPath := filepath.Join(dir, originalTitle)
	if _, err := os.Stat(originalPath); err == nil {
		return originalPath
	}

	ext := filepath.Ext(originalTitle)
	basename := strings.TrimSuffix(originalTitle, ext)

	// Check for truncated filenames
	if path := checkTruncatedName(dir, basename, ext, originalTitle, 48); path != "" {
		return path
	}
	if path := checkTruncatedName(dir, basename, ext, originalTitle, 47); path != "" {
		return path
	}
	if path := checkTruncatedName(dir, basename, ext, originalTitle, 46); path != "" {
		return path
	}

	// Check for numbered suffix like ..._1.jpg -> ...(1).jpg
	reNum := regexp.MustCompile(`_(\d+)$`)
	if matches := reNum.FindStringSubmatch(basename); len(matches) > 1 {
		baseWithoutNum := strings.TrimSuffix(basename, matches[0])
		numberedPath := filepath.Join(dir, fmt.Sprintf("%s(%s)%s", baseWithoutNum, matches[1], ext))
		if _, err := os.Stat(numberedPath); err == nil {
			log.Printf("Found match for '%s' with numbered name: %s", originalTitle, filepath.Base(numberedPath))
			return numberedPath
		}
	}

	// Check for apostrophes or quotes
	if strings.ContainsAny(originalTitle, "'\"") {
		replacer := strings.NewReplacer("'", "_", "\"", "_")
		replacedTitle := replacer.Replace(originalTitle)
		replacedPath := filepath.Join(dir, replacedTitle)
		if _, err := os.Stat(replacedPath); err == nil {
			log.Printf("Found match for '%s' with replaced quotes: %s", originalTitle, replacedTitle)
			return replacedPath
		}
	}

	// Check for different extension cases
	if path := checkExtensionCase(dir, basename, ext, originalTitle, strings.ToLower, "downcased"); path != "" {
		return path
	}
	if path := checkExtensionCase(dir, basename, ext, originalTitle, strings.ToUpper, "upcased"); path != "" {
		return path
	}

	return "" // Return empty string if no file is found
}

// checkExtensionCase checks if a file exists with the extension transformed by the given case function
func checkExtensionCase(dir, basename, ext, originalTitle string, caseFunc func(string) string, caseName string) string {
	newExt := caseFunc(ext)
	if ext != newExt {
		newPath := filepath.Join(dir, basename+newExt)
		if _, err := os.Stat(newPath); err == nil {
			log.Printf("Found match for '%s' with %s extension: %s", originalTitle, caseName, filepath.Base(newPath))
			return newPath
		}
	}
	return ""
}

// ensureDirectory creates a directory if it doesn't exist
func ensureDirectory(path string, dryRun bool) error {
	if dryRun {
		log.Printf("[DRY RUN] Would create directory: %s", path)
		return nil
	}
	return os.MkdirAll(path, 0755)
}

// moveOrCopyFile moves or copies a file from src to dest, creating directories as needed
func moveOrCopyFile(src, dest string, dryRun, keepFiles bool) error {
	if dryRun {
		if keepFiles {
			log.Printf("[DRY RUN] Would copy file: %s -> %s", src, dest)
		} else {
			log.Printf("[DRY RUN] Would move file: %s -> %s", src, dest)
		}
		return nil
	}

	// Create destination directory if it doesn't exist
	destDir := filepath.Dir(dest)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating destination directory %s: %v", destDir, err)
	}

	if keepFiles {
		// Copy the file
		return copyFile(src, dest)
	} else {
		// Move the file
		return os.Rename(src, dest)
	}
}

// copyFile copies a file from src to dest
func copyFile(src, dest string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source file %s: %v", src, err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("creating destination file %s: %v", dest, err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("copying file content: %v", err)
	}

	// Copy file permissions
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return fmt.Errorf("getting source file info: %v", err)
	}

	return os.Chmod(dest, sourceInfo.Mode())
}

// createSymlink creates a symbolic link
func createSymlink(oldname, newname string, dryRun bool) error {
	if dryRun {
		// Check if symlink already exists in dry run mode
		if linkInfo, err := os.Lstat(newname); err == nil {
			if linkInfo.Mode()&os.ModeSymlink != 0 {
				if target, err := os.Readlink(newname); err == nil && target == oldname {
					log.Printf("[DRY RUN] Symlink already exists and is correct: %s -> %s", newname, oldname)
					return nil
				}
			}
			log.Printf("[DRY RUN] Would replace existing file/symlink: %s -> %s", newname, oldname)
		} else {
			log.Printf("[DRY RUN] Would create symlink: %s -> %s", newname, oldname)
		}
		return nil
	}

	// Check if symlink already exists and points to the correct target
	if linkInfo, err := os.Lstat(newname); err == nil {
		if linkInfo.Mode()&os.ModeSymlink != 0 {
			// It's a symlink, check if it points to the right place
			if target, err := os.Readlink(newname); err == nil && target == oldname {
				// Symlink already exists and points to the correct target
				return nil
			}
		}
		// Remove existing file/symlink if it exists but doesn't point to the right place
		if err := os.Remove(newname); err != nil {
			return fmt.Errorf("removing existing file/symlink %s: %v", newname, err)
		}
	}

	return os.Symlink(oldname, newname)
}

// getDateFromTimestamp extracts year, month, day from Unix timestamp
func getDateFromTimestamp(timestamp int64) (year, month, day string) {
	t := time.Unix(timestamp, 0).UTC()
	return fmt.Sprintf("%04d", t.Year()), fmt.Sprintf("%02d", int(t.Month())), fmt.Sprintf("%02d", t.Day())
}

// performScan scans all non-JSON files and reports how many are missing EXIF timestamp data
func performScan(sourceDir string) {
	fmt.Printf("Scanning directory: %s\n", sourceDir)
	fmt.Println("Checking for missing EXIF timestamp data...")
	fmt.Println("Looking for: DateTimeOriginal, MediaCreateDate, CreationDate, TrackCreateDate, CreateDate, DateTimeDigitized, GPSDateStamp, DateTime")
	fmt.Println()

	// Start exiftool process
	et, err := NewExifTool()
	if err != nil {
		log.Fatalf("Failed to start exiftool for scanning: %v", err)
	}
	defer et.Close()

	var totalFiles, missingFiles int
	var filesToCheck []string

	// Collect all non-JSON files
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Warning: Skipping path due to error: %s: %v", path, err)
			return nil
		}

		if !info.IsDir() && filepath.Ext(strings.ToLower(path)) != ".json" {
			// Skip common non-media files
			ext := strings.ToLower(filepath.Ext(path))
			if isMediaFile(ext) {
				filesToCheck = append(filesToCheck, path)
				totalFiles++
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error walking directory: %v", err)
	}

	fmt.Printf("Found %d media files to check\n", totalFiles)
	fmt.Println("Analyzing files...")

	// Check each file for EXIF timestamp data
	for i, filePath := range filesToCheck {
		if i%100 == 0 && i > 0 {
			fmt.Printf("Processed %d/%d files...\n", i, totalFiles)
		}

		if isMissingTimestamps(et, filePath) {
			missingFiles++
		}
	}

	// Report results
	fmt.Printf("\n=== SCAN RESULTS ===\n")
	fmt.Printf("Total media files scanned: %d\n", totalFiles)
	fmt.Printf("Files missing ALL timestamp data: %d\n", missingFiles)
	fmt.Printf("Files with some timestamp data: %d\n", totalFiles-missingFiles)

	if totalFiles > 0 {
		percentage := float64(missingFiles) / float64(totalFiles) * 100
		fmt.Printf("Percentage missing timestamps: %.1f%%\n", percentage)
	}

	if missingFiles > 0 {
		fmt.Printf("\nFiles missing timestamps would benefit from EXIF timestamp updating.\n")
	} else {
		fmt.Printf("\nAll files have some form of timestamp data.\n")
	}
}

// isMediaFile checks if the file extension indicates a media file
func isMediaFile(ext string) bool {
	mediaExtensions := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true,
		".tiff": true, ".tif": true, ".webp": true, ".heic": true, ".heif": true,
		".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".wmv": true,
		".flv": true, ".webm": true, ".m4v": true, ".3gp": true, ".mpg": true,
		".mpeg": true, ".m2v": true, ".mts": true, ".m2ts": true,
		".cr2": true, ".nef": true, ".arw": true, ".dng": true, ".orf": true,
		".rw2": true, ".pef": true, ".sr2": true, ".x3f": true,
	}
	return mediaExtensions[ext]
}

// isMissingTimestamps checks if a file is missing all EXIF timestamp fields
func isMissingTimestamps(et *ExifTool, filePath string) bool {
	// Get EXIF data for timestamp fields
	output, err := et.Execute(
		"-DateTimeOriginal",
		"-MediaCreateDate",
		"-CreationDate",
		"-TrackCreateDate",
		"-CreateDate",
		"-DateTimeDigitized",
		"-GPSDateStamp",
		"-DateTime",
		"-s", // short output format
		"-S", // very short output format
		filePath,
	)

	if err != nil {
		log.Printf("Warning: Could not read EXIF data from %s: %v", filePath, err)
		return true // Assume missing if we can't read it
	}

	// Check if any timestamp field has a value
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for lines with timestamp data (not just field names)
		if strings.Contains(line, ":") && len(line) > 20 {
			// If we find any timestamp data, file is not missing all timestamps
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" && strings.TrimSpace(parts[1]) != "-" {
				return false
			}
		}
	}

	return true // No valid timestamp data found
}

func main() {
	// --- 1. Setup and Command-Line Parsing ---
	fmt.Println("Starting EXIF timestamp updater...")

	keepJSON := flag.Bool("keep-json", false, "Keep JSON files after processing (don't delete them)")
	keepFiles := flag.Bool("keep-files", false, "Copy files instead of moving them (preserves originals)")
	dryRun := flag.Bool("dry-run", false, "Show what would be done without making any changes")
	scanOnly := flag.Bool("scan", false, "Scan files to report how many are missing EXIF timestamp data")
	var destDir string
	flag.StringVar(&destDir, "dest", "", "Destination directory for organized photos")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <source_directory>\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "  source_directory  The root directory to scan\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nThe destination directory will be organized as:\n")
		fmt.Fprintf(os.Stderr, "  <dest>/ALL_PHOTOS/<year>/<month>/<day>/<filename>\n")
		fmt.Fprintf(os.Stderr, "  <dest>/<album_name>/<filename> (symlinks to ALL_PHOTOS)\n")
		fmt.Fprintf(os.Stderr, "\nScan mode analyzes files for missing EXIF timestamp data:\n")
		fmt.Fprintf(os.Stderr, "  DateTimeOriginal, MediaCreateDate, CreationDate, TrackCreateDate,\n")
		fmt.Fprintf(os.Stderr, "  CreateDate, DateTimeDigitized, GPSDateStamp, DateTime\n")
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		log.Fatal("Error: No source directory specified")
	}

	if !*scanOnly && destDir == "" {
		flag.Usage()
		log.Fatal("Error: Destination directory (-dest) is required (not needed for --scan mode)")
	}

	sourceDir := flag.Arg(0)
	info, err := os.Stat(sourceDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("Error: Source directory does not exist: %s", sourceDir)
		}
		log.Fatalf("Error: Could not access source directory %s: %v", sourceDir, err)
	}
	if !info.IsDir() {
		log.Fatalf("Error: Provided source path is not a directory: %s", sourceDir)
	}

	// Check if exiftool is available
	if _, err := exec.LookPath("exiftool"); err != nil {
		log.Fatalf("Error: 'exiftool' command not found. Please ensure it is installed and in your system's PATH.")
	}

	// Handle scan mode
	if *scanOnly {
		performScan(sourceDir)
		return
	}

	// Create destination directory if it doesn't exist
	if err := ensureDirectory(destDir, *dryRun); err != nil {
		log.Fatalf("Error: Could not create destination directory %s: %v", destDir, err)
	}

	if *dryRun {
		log.Printf("DRY RUN MODE: No files will be modified")
	}

	// --- 2. Worker Pool Initialization ---
	numWorkers := runtime.NumCPU()
	log.Printf("Initializing worker pool with %d workers.", numWorkers)

	jobs := make(chan string, numWorkers)
	var wg sync.WaitGroup

	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(i, &wg, jobs, keepJSON, keepFiles, destDir, dryRun)
	}

	// --- 3. Directory Traversal ---
	go func() {
		defer close(jobs)
		err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Printf("Warning: Skipping path due to error: %s: %v", path, err)
				return nil
			}
			if !info.IsDir() && filepath.Ext(path) == ".json" {
				jobs <- path
			}
			return nil
		})
		if err != nil {
			log.Printf("Error walking the path %s: %v\n", sourceDir, err)
		}
	}()

	// --- 4. Wait for Completion ---
	wg.Wait()
	fmt.Println("Processing complete.")
}

// worker defines the work each goroutine will perform.
func worker(id int, wg *sync.WaitGroup, jobs <-chan string, keepJSON, keepFiles *bool, destDir string, dryRun *bool) {
	defer wg.Done()

	var et *ExifTool
	var err error

	// Only start exiftool if not in dry run mode
	if !*dryRun {
		et, err = NewExifTool()
		if err != nil {
			log.Printf("Worker %d: Failed to start exiftool: %v", id, err)
			return
		}
		defer et.Close()
		log.Printf("Worker %d: Exiftool process started.", id)
	}

	for jsonPath := range jobs {
		// --- 1. Read and Parse JSON ---
		file, err := os.Open(jsonPath)
		if err != nil {
			log.Printf("Worker %d: Error opening %s: %v", id, jsonPath, err)
			continue
		}

		byteValue, err := io.ReadAll(file)
		file.Close() // Close file immediately after read.
		if err != nil {
			log.Printf("Worker %d: Error reading %s: %v", id, jsonPath, err)
			continue
		}

		var meta photoMetadata
		if err := json.Unmarshal(byteValue, &meta); err != nil {
			log.Printf("Worker %d: Error unmarshaling %s: %v", id, jsonPath, err)
			continue
		}

		if meta.Title == "" || meta.PhotoTakenTime.Timestamp == "" {
			log.Printf("Worker %d: Skipping %s, missing title or timestamp.", id, jsonPath)
			continue
		}

		// --- 2. Find the target file using fallback logic ---
		imagePath := findFileWithFallbacks(filepath.Dir(jsonPath), meta.Title)
		if imagePath == "" {
			log.Printf("Worker %d: Image file '%s' not found for json '%s' after all checks. Skipping.", id, meta.Title, jsonPath)
			continue
		}

		// --- 3. Convert Timestamp and determine date structure ---
		timestamp, err := strconv.ParseInt(meta.PhotoTakenTime.Timestamp, 10, 64)
		if err != nil {
			log.Printf("Worker %d: Could not parse timestamp in %s: %v", id, jsonPath, err)
			continue
		}

		year, month, day := getDateFromTimestamp(timestamp)
		filename := filepath.Base(imagePath)

		// Create destination path: <dest>/ALL_PHOTOS/<year>/<month>/<day>/<filename>
		allPhotosPath := filepath.Join(destDir, "ALL_PHOTOS", year, month, day)
		destPath := filepath.Join(allPhotosPath, filename)

		// Check if file already exists at destination
		fileAlreadyExists := false
		if _, err := os.Stat(destPath); err == nil {
			fileAlreadyExists = true
			if *dryRun {
				log.Printf("Worker %d: [DRY RUN] File already exists at destination %s, skipping file operations but checking album symlinks", id, destPath)
			} else {
				log.Printf("Worker %d: File already exists at destination %s, skipping file operations but checking album symlinks", id, destPath)
			}
		}

		// --- 4. Update EXIF data and move/copy file (only if file doesn't already exist) ---
		if !fileAlreadyExists {
			if !*dryRun {
				t := time.Unix(timestamp, 0)
				formattedTime := t.Format("2006:01:02 15:04:05")
				dateTimeArg := fmt.Sprintf("-CreateDate=%s -DateTimeOriginal=%s", formattedTime, formattedTime)

				log.Printf("Worker %d: Updating EXIF for %s", id, imagePath)
				output, err := et.Execute("-overwrite_original", dateTimeArg, imagePath)
				if err != nil {
					log.Printf("Worker %d: Exiftool command failed for '%s': %v\nOutput: %s", id, imagePath, err, output)
					continue
				}
			} else {
				log.Printf("Worker %d: [DRY RUN] Would update EXIF for %s", id, imagePath)
			}

			// --- 5. Move or copy file to organized structure ---
			if err := moveOrCopyFile(imagePath, destPath, *dryRun, *keepFiles); err != nil {
				if *keepFiles {
					log.Printf("Worker %d: Error copying file %s to %s: %v", id, imagePath, destPath, err)
				} else {
					log.Printf("Worker %d: Error moving file %s to %s: %v", id, imagePath, destPath, err)
				}
				continue
			}
		}

		// --- 6. Read metadata.json from the same directory for album info ---
		metadataJsonPath := filepath.Join(filepath.Dir(jsonPath), "metadata.json")
		albumName := ""

		if metadataFile, err := os.Open(metadataJsonPath); err == nil {
			var metadataContent map[string]interface{}
			decoder := json.NewDecoder(metadataFile)
			if err := decoder.Decode(&metadataContent); err == nil {
				if title, ok := metadataContent["title"].(string); ok && title != "" {
					albumName = title
				}
			}
			metadataFile.Close()
		}

		// --- 7. Create album directory and symlink ---
		if albumName != "" {
			albumDir := filepath.Join(destDir, albumName)
			if err := ensureDirectory(albumDir, *dryRun); err != nil {
				log.Printf("Worker %d: Error creating album directory %s: %v", id, albumDir, err)
			} else {
				// Create relative path for symlink: ../ALL_PHOTOS/<year>/<month>/<day>/<filename>
				relativePath := filepath.Join("..", "ALL_PHOTOS", year, month, day, filename)
				symlinkPath := filepath.Join(albumDir, filename)

				if err := createSymlink(relativePath, symlinkPath, *dryRun); err != nil {
					log.Printf("Worker %d: Error creating symlink %s -> %s: %v", id, symlinkPath, relativePath, err)
				} else if !*dryRun {
					log.Printf("Worker %d: Created/verified symlink in album '%s': %s", id, albumName, filename)
				}
			}
		}

		// --- 8. Handle JSON file (only if file operations were performed) ---
		if !fileAlreadyExists && !*keepJSON {
			if *dryRun {
				log.Printf("Worker %d: [DRY RUN] Would delete JSON file %s", id, jsonPath)
			} else {
				if err := os.Remove(jsonPath); err != nil {
					log.Printf("Worker %d: Warning: Could not delete JSON file %s: %v", id, jsonPath, err)
				}
			}
		}

		if *dryRun {
			log.Printf("Worker %d: [DRY RUN] Successfully processed %s", id, jsonPath)
		} else {
			if fileAlreadyExists {
				log.Printf("Worker %d: Successfully processed %s (file already existed, checked album symlinks)", id, jsonPath)
			} else if *keepFiles {
				log.Printf("Worker %d: Successfully processed %s -> %s (copied)", id, jsonPath, destPath)
			} else {
				log.Printf("Worker %d: Successfully processed %s -> %s (moved)", id, jsonPath, destPath)
			}
		}
	}

	if !*dryRun {
		log.Printf("Worker %d: Shutting down.", id)
	} else {
		log.Printf("Worker %d: [DRY RUN] Shutting down.", id)
	}
}
