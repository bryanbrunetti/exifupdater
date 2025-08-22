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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type photoMetadata struct {
	Title          string `json:"title"`
	PhotoTakenTime struct {
		Timestamp string `json:"timestamp"`
	} `json:"photoTakenTime"`
	Timestamp string `json:"timestamp"` // Legacy field
}

// ExifTool represents a persistent exiftool process
type ExifTool struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
}

// NewExifTool starts a new persistent exiftool process
func NewExifTool() (*ExifTool, error) {
	cmd := exec.Command("exiftool", "-stay_open", "True", "-@", "-")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, err
	}

	return &ExifTool{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
	}, nil
}

// Execute runs a command through the persistent exiftool process
func (et *ExifTool) Execute(args ...string) (string, error) {
	// Write command arguments
	for _, arg := range args {
		if _, err := fmt.Fprintln(et.stdin, arg); err != nil {
			return "", err
		}
	}

	// End command
	if _, err := fmt.Fprintln(et.stdin, "-execute"); err != nil {
		return "", err
	}

	// Read response
	var output strings.Builder
	for {
		line, err := et.stdout.ReadString('\n')
		if err != nil {
			return "", err
		}

		if strings.TrimSpace(line) == "{ready}" {
			break
		}

		output.WriteString(line)
	}

	return strings.TrimSpace(output.String()), nil
}

// Close terminates the persistent exiftool process
func (et *ExifTool) Close() error {
	if _, err := fmt.Fprintln(et.stdin, "-stay_open"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(et.stdin, "False"); err != nil {
		return err
	}

	et.stdin.Close()
	return et.cmd.Wait()
}

func checkTruncatedName(dir, originalTitle string) string {
	for _, length := range []int{48, 47, 46} {
		if len(originalTitle) > length {
			truncated := originalTitle[:length]
			fullPath := filepath.Join(dir, truncated)
			if _, err := os.Stat(fullPath); err == nil {
				return fullPath
			}
		}
	}
	return ""
}

func findFileWithFallbacks(dir, originalTitle string) string {
	// Try the original title first
	fullPath := filepath.Join(dir, originalTitle)
	if _, err := os.Stat(fullPath); err == nil {
		return fullPath
	}

	// Try different extensions and cases
	baseName := strings.TrimSuffix(originalTitle, filepath.Ext(originalTitle))
	extensions := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".heic", ".mp4", ".mov", ".avi", ".mkv"}

	for _, ext := range extensions {
		variants := []string{
			baseName + ext,
			baseName + strings.ToUpper(ext),
		}

		for _, variant := range variants {
			fullPath := filepath.Join(dir, variant)
			if _, err := os.Stat(fullPath); err == nil {
				return fullPath
			}
		}
	}

	// Try truncated names
	if truncatedPath := checkTruncatedName(dir, originalTitle); truncatedPath != "" {
		return truncatedPath
	}

	return ""
}

func ensureDirectory(path string, dryRun bool) error {
	if dryRun {
		log.Printf("[DRY RUN] Would create directory: %s", path)
		return nil
	}
	return os.MkdirAll(path, 0755)
}

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

func copyFile(src, dest string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func createSymlink(oldname, newname string, dryRun bool) error {
	if dryRun {
		log.Printf("[DRY RUN] Would create symlink: %s -> %s", newname, oldname)
		return nil
	}

	// Remove existing symlink if it exists
	if _, err := os.Lstat(newname); err == nil {
		if err := os.Remove(newname); err != nil {
			return fmt.Errorf("removing existing symlink %s: %v", newname, err)
		}
	}

	return os.Symlink(oldname, newname)
}

func getDateFromTimestamp(timestamp int64) (year, month, day string) {
	t := time.Unix(timestamp, 0).UTC()
	return fmt.Sprintf("%04d", t.Year()), fmt.Sprintf("%02d", int(t.Month())), fmt.Sprintf("%02d", t.Day())
}

// Progress bar for tracking operations
type progressBar struct {
	total     int64
	current   int64
	startTime time.Time
	mutex     sync.RWMutex
}

func newProgressBar(total int) *progressBar {
	return &progressBar{
		total:     int64(total),
		current:   0,
		startTime: time.Now(),
	}
}

func (pb *progressBar) update() {
	atomic.AddInt64(&pb.current, 1)
}

func (pb *progressBar) display(current int64) {
	pb.mutex.Lock()
	defer pb.mutex.Unlock()

	elapsed := time.Since(pb.startTime)
	percentage := float64(current) / float64(pb.total) * 100

	// Create progress bar
	width := 30
	filled := int(float64(width) * float64(current) / float64(pb.total))
	bar := "[" + strings.Repeat("=", filled) + strings.Repeat(" ", width-filled) + "]"

	// Calculate ETA
	eta := ""
	if current > 0 && current < pb.total {
		avgTimePerFile := elapsed.Seconds() / float64(current)
		remaining := float64(pb.total - current)
		etaSeconds := avgTimePerFile * remaining
		etaDuration := time.Duration(etaSeconds * float64(time.Second))
		eta = fmt.Sprintf(" | ETA: %s", formatDuration(etaDuration))
	}

	fmt.Printf("\r%s %d/%d (%.1f%%) | Elapsed: %s%s",
		bar, current, pb.total, percentage, formatDuration(elapsed), eta)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) - 60*minutes
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	} else {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) - 60*hours
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
}

// SCAN MODE FUNCTIONS

type scanResult struct {
	filePath string
	missing  bool
}

func isMediaFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	mediaExts := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".heic", ".mp4", ".mov", ".avi", ".mkv", ".webm", ".m4v"}

	for _, mediaExt := range mediaExts {
		if ext == mediaExt {
			return true
		}
	}
	return false
}

func isMissingTimestamps(et *ExifTool, filePath string) bool {
	output, err := et.Execute(
		"-DateTimeOriginal",
		"-MediaCreateDate",
		"-CreationDate",
		"-TrackCreateDate",
		"-CreateDate",
		"-DateTimeDigitized",
		"-GPSDateStamp",
		"-DateTime",
		"-s", "-S",
		filePath,
	)

	if err != nil {
		return true
	}

	// Check if any timestamp field has a value
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.Contains(line, ":") && len(line) >= 10 {
			if line != "-" && !strings.Contains(strings.ToLower(line), "error") {
				return false
			}
		}
	}

	return true
}

func performScan(sourceDir string) {
	timestamp := time.Now().Format("20060102_150405")
	logFileName := fmt.Sprintf("missing_timestamps_%s.log", timestamp)
	logFile, err := os.Create(logFileName)
	if err != nil {
		log.Fatalf("Error creating log file: %v", err)
	}
	defer logFile.Close()

	fmt.Fprintf(logFile, "# Files Missing ALL Timestamp Data\n")
	fmt.Fprintf(logFile, "# Scan Date: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(logFile, "# Source Directory: %s\n", sourceDir)
	fmt.Fprintf(logFile, "#\n")

	fmt.Printf("Scanning directory: %s\n", sourceDir)

	var allFiles []string
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && isMediaFile(info.Name()) {
			allFiles = append(allFiles, path)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error scanning directory: %v", err)
	}

	totalFiles := len(allFiles)
	fmt.Printf("Found %d media files to check\n", totalFiles)

	if totalFiles == 0 {
		fmt.Println("No media files found.")
		return
	}

	numWorkers := runtime.NumCPU()
	fmt.Printf("Using %d workers for scanning...\n\n", numWorkers)

	pb := newProgressBar(totalFiles)
	jobs := make(chan string, numWorkers*2)
	results := make(chan scanResult, totalFiles)

	var wg sync.WaitGroup
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go scanWorker(i, &wg, jobs, results, pb)
	}

	go func() {
		defer close(jobs)
		for _, filePath := range allFiles {
			jobs <- filePath
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var missingFilePaths []string
	for result := range results {
		if result.missing {
			missingFilePaths = append(missingFilePaths, result.filePath)
		}
		pb.display(int64(len(missingFilePaths) + totalFiles - len(missingFilePaths)))
	}

	pb.display(int64(totalFiles))
	fmt.Println()

	missingFiles := len(missingFilePaths)

	if len(missingFilePaths) > 0 {
		for _, filePath := range missingFilePaths {
			fmt.Fprintf(logFile, "%s\n", filePath)
		}
	}

	fmt.Printf("\n=== SCAN RESULTS ===\n")
	fmt.Printf("Total media files scanned: %d\n", totalFiles)
	fmt.Printf("Files missing ALL timestamp data: %d\n", missingFiles)
	fmt.Printf("Files with some timestamp data: %d\n", totalFiles-missingFiles)

	if totalFiles > 0 {
		percentage := float64(missingFiles) / float64(totalFiles) * 100
		fmt.Printf("Percentage missing timestamps: %.1f%%\n", percentage)
	}
}

func scanWorker(id int, wg *sync.WaitGroup, jobs <-chan string, results chan<- scanResult, pb *progressBar) {
	defer wg.Done()

	et, err := NewExifTool()
	if err != nil {
		log.Printf("Worker %d: Failed to start exiftool: %v", id, err)
		return
	}
	defer et.Close()

	for filePath := range jobs {
		missing := isMissingTimestamps(et, filePath)
		results <- scanResult{
			filePath: filePath,
			missing:  missing,
		}
		pb.update()
	}
}

// UPDATE MODE FUNCTIONS

func performUpdate(sourceDir string, keepJSON, dryRun bool) {
	fmt.Println("UPDATE MODE: Updating EXIF timestamps from JSON metadata...")

	var jsonFiles []string
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Warning: Skipping path due to error: %s: %v", path, err)
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".json" && filepath.Base(path) != "metadata.json" {
			jsonFiles = append(jsonFiles, path)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error scanning for JSON files: %v", err)
	}

	totalFiles := len(jsonFiles)
	fmt.Printf("Found %d JSON files to process\n", totalFiles)

	if totalFiles == 0 {
		fmt.Println("No JSON files found to process.")
		return
	}

	pb := newProgressBar(totalFiles)
	numWorkers := runtime.NumCPU()
	jobs := make(chan string, numWorkers)
	var wg sync.WaitGroup

	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go updateWorker(i, &wg, jobs, keepJSON, dryRun, pb)
	}

	go func() {
		defer close(jobs)
		for _, jsonPath := range jsonFiles {
			jobs <- jsonPath
		}
	}()

	wg.Wait()
	pb.display(int64(totalFiles))
	fmt.Println()
	fmt.Printf("Update complete! Processed %d JSON files.\n", totalFiles)
}

func updateWorker(id int, wg *sync.WaitGroup, jobs <-chan string, keepJSON, dryRun bool, pb *progressBar) {
	defer wg.Done()

	var et *ExifTool
	var err error

	if !dryRun {
		et, err = NewExifTool()
		if err != nil {
			log.Printf("Worker %d: Failed to start exiftool: %v", id, err)
			return
		}
		defer et.Close()
	}

	for jsonPath := range jobs {
		file, err := os.Open(jsonPath)
		if err != nil {
			log.Printf("Worker %d: Error opening %s: %v", id, jsonPath, err)
			continue
		}

		byteValue, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			log.Printf("Worker %d: Error reading %s: %v", id, jsonPath, err)
			continue
		}

		var meta photoMetadata
		if err := json.Unmarshal(byteValue, &meta); err != nil {
			log.Printf("Worker %d: Error unmarshaling %s: %v", id, jsonPath, err)
			continue
		}

		timestampStr := meta.PhotoTakenTime.Timestamp
		if timestampStr == "" {
			timestampStr = meta.Timestamp // Try legacy field
		}

		if meta.Title == "" || timestampStr == "" {
			continue
		}

		imagePath := findFileWithFallbacks(filepath.Dir(jsonPath), meta.Title)
		if imagePath == "" {
			continue
		}

		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			continue
		}

		if !dryRun {
			t := time.Unix(timestamp, 0)
			formattedTime := t.Format("2006:01:02 15:04:05")
			dateTimeArg := fmt.Sprintf("-CreateDate=%s -DateTimeOriginal=%s", formattedTime, formattedTime)

			_, err := et.Execute("-overwrite_original", dateTimeArg, imagePath)
			if err != nil {
				log.Printf("Worker %d: Exiftool command failed for '%s': %v", id, imagePath, err)
				continue
			}
		} else {
			log.Printf("[DRY RUN] Would update EXIF for %s", imagePath)
		}

		if !keepJSON && !dryRun {
			if err := os.Remove(jsonPath); err != nil {
				log.Printf("Worker %d: Warning: Could not delete JSON file %s: %v", id, jsonPath, err)
			}
		} else if !keepJSON && dryRun {
			log.Printf("[DRY RUN] Would delete JSON file %s", jsonPath)
		}

		pb.update()
	}
}

// SORT MODE FUNCTIONS

func performSort(sourceDir, destDir string, keepFiles, dryRun bool) {
	fmt.Println("SORT MODE: Organizing files into date-based structure with album symlinks...")

	if err := ensureDirectory(destDir, dryRun); err != nil {
		log.Fatalf("Error: Could not create destination directory %s: %v", destDir, err)
	}

	var jsonFiles []string
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Warning: Skipping path due to error: %s: %v", path, err)
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".json" && filepath.Base(path) != "metadata.json" {
			jsonFiles = append(jsonFiles, path)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error scanning for JSON files: %v", err)
	}

	totalFiles := len(jsonFiles)
	fmt.Printf("Found %d JSON files to process\n", totalFiles)

	if totalFiles == 0 {
		fmt.Println("No JSON files found to process.")
		return
	}

	pb := newProgressBar(totalFiles)
	numWorkers := runtime.NumCPU()
	jobs := make(chan string, numWorkers)
	var wg sync.WaitGroup

	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go sortWorker(i, &wg, jobs, destDir, keepFiles, dryRun, pb)
	}

	go func() {
		defer close(jobs)
		for _, jsonPath := range jsonFiles {
			jobs <- jsonPath
		}
	}()

	wg.Wait()
	pb.display(int64(totalFiles))
	fmt.Println()
	fmt.Printf("Sort complete! Processed %d JSON files.\n", totalFiles)
}

func sortWorker(id int, wg *sync.WaitGroup, jobs <-chan string, destDir string, keepFiles, dryRun bool, pb *progressBar) {
	defer wg.Done()

	for jsonPath := range jobs {
		file, err := os.Open(jsonPath)
		if err != nil {
			log.Printf("Worker %d: Error opening %s: %v", id, jsonPath, err)
			continue
		}

		byteValue, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			log.Printf("Worker %d: Error reading %s: %v", id, jsonPath, err)
			continue
		}

		var meta photoMetadata
		if err := json.Unmarshal(byteValue, &meta); err != nil {
			log.Printf("Worker %d: Error unmarshaling %s: %v", id, jsonPath, err)
			continue
		}

		timestampStr := meta.PhotoTakenTime.Timestamp
		if timestampStr == "" {
			timestampStr = meta.Timestamp
		}

		if meta.Title == "" || timestampStr == "" {
			continue
		}

		imagePath := findFileWithFallbacks(filepath.Dir(jsonPath), meta.Title)
		if imagePath == "" {
			continue
		}

		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			continue
		}

		year, month, day := getDateFromTimestamp(timestamp)
		filename := filepath.Base(imagePath)

		// Create destination path: <dest>/<year>/<month>/<day>/<filename>
		datePath := filepath.Join(destDir, year, month, day)
		destPath := filepath.Join(datePath, filename)

		// Check if file already exists at destination
		fileAlreadyExists := false
		if _, err := os.Stat(destPath); err == nil {
			fileAlreadyExists = true
		}

		// Move/copy file to date-based structure
		if !fileAlreadyExists {
			if err := moveOrCopyFile(imagePath, destPath, dryRun, keepFiles); err != nil {
				log.Printf("Worker %d: Error moving/copying file %s to %s: %v", id, imagePath, destPath, err)
				continue
			}
		}

		// Handle album creation if metadata.json exists
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

		// Create album directory and symlink
		if albumName != "" {
			albumDir := filepath.Join(destDir, albumName)
			if err := ensureDirectory(albumDir, dryRun); err != nil {
				log.Printf("Worker %d: Error creating album directory %s: %v", id, albumDir, err)
			} else {
				// Create relative path for symlink: ../<year>/<month>/<day>/<filename>
				relativePath := filepath.Join("..", year, month, day, filename)
				symlinkPath := filepath.Join(albumDir, filename)

				if err := createSymlink(relativePath, symlinkPath, dryRun); err != nil {
					log.Printf("Worker %d: Error creating symlink %s -> %s: %v", id, symlinkPath, relativePath, err)
				}
			}
		}

		pb.update()
	}
}

// MAIN FUNCTION

func main() {
	fmt.Println("EXIF Updater - Multi-mode photo organization tool")
	fmt.Println()

	// Mode flags (mutually exclusive)
	scanMode := flag.Bool("scan", false, "Scan files to report how many are missing EXIF timestamp data")
	updateMode := flag.Bool("update", false, "Update EXIF timestamps from JSON metadata files")
	sortMode := flag.Bool("sort", false, "Sort files into date-based directory structure with album symlinks")

	// Options
	keepJSON := flag.Bool("keep-json", false, "Keep JSON files after processing (don't delete them)")
	keepFiles := flag.Bool("keep-files", false, "Copy files instead of moving them (preserves originals)")
	dryRun := flag.Bool("dry-run", false, "Show what would be done without making any changes")
	var destDir string
	flag.StringVar(&destDir, "dest", "", "Destination directory (required for sort mode)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [mode] [options] <source_directory>\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "\nModes (choose exactly one):\n")
		fmt.Fprintf(os.Stderr, "  -scan    Scan files and report how many are missing EXIF timestamp data\n")
		fmt.Fprintf(os.Stderr, "  -update  Update EXIF timestamps from JSON metadata files\n")
		fmt.Fprintf(os.Stderr, "  -sort    Sort files into <year>/<month>/<day> structure with album symlinks\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -scan ~/google-takeout\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "  %s -update ~/google-takeout\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "  %s -sort -dest ~/organized-photos ~/google-takeout\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "  %s -sort -keep-files -dest ~/organized-photos ~/google-takeout\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "\nThe sort mode organizes files as:\n")
		fmt.Fprintf(os.Stderr, "  <dest>/<year>/<month>/<day>/<filename>\n")
		fmt.Fprintf(os.Stderr, "  <dest>/<album_name>/<filename> (symlinks to date structure)\n")
	}
	flag.Parse()

	// Validate arguments
	if flag.NArg() == 0 {
		flag.Usage()
		log.Fatal("Error: No source directory specified")
	}

	// Count how many modes are selected
	modeCount := 0
	if *scanMode {
		modeCount++
	}
	if *updateMode {
		modeCount++
	}
	if *sortMode {
		modeCount++
	}

	if modeCount == 0 {
		flag.Usage()
		log.Fatal("Error: You must specify exactly one mode (-scan, -update, or -sort)")
	}

	if modeCount > 1 {
		flag.Usage()
		log.Fatal("Error: You can only specify one mode at a time")
	}

	// Validate destination directory for sort mode
	if *sortMode && destDir == "" {
		flag.Usage()
		log.Fatal("Error: Destination directory (-dest) is required for sort mode")
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

	// Check if exiftool is available (except for sort-only mode)
	if *scanMode || *updateMode {
		if _, err := exec.LookPath("exiftool"); err != nil {
			log.Fatalf("Error: 'exiftool' command not found. Please ensure it is installed and in your system's PATH.")
		}
	}

	if *dryRun {
		fmt.Println("🔍 DRY RUN MODE: No files will be modified")
		fmt.Println()
	}

	// Execute the selected mode
	switch {
	case *scanMode:
		performScan(sourceDir)
	case *updateMode:
		performUpdate(sourceDir, *keepJSON, *dryRun)
	case *sortMode:
		performSort(sourceDir, destDir, *keepFiles, *dryRun)
	}
}
