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

func main() {
	// --- 1. Setup and Command-Line Parsing ---
	fmt.Println("Starting EXIF timestamp updater...")

	keepJSON := flag.Bool("keep-json", false, "Keep JSON files after processing (don't delete them)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <directory>\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "  directory  The root directory to scan for JSON files\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		log.Fatal("Error: No directory specified")
	}

	dir := flag.Arg(0)
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("Error: Directory does not exist: %s", dir)
		}
		log.Fatalf("Error: Could not access directory %s: %v", dir, err)
	}
	if !info.IsDir() {
		log.Fatalf("Error: Provided path is not a directory: %s", dir)
	}

	if _, err := exec.LookPath("exiftool"); err != nil {
		log.Fatalf("Error: 'exiftool' command not found. Please ensure it is installed and in your system's PATH.")
	}

	// --- 2. Worker Pool Initialization ---
	numWorkers := runtime.NumCPU()
	log.Printf("Initializing worker pool with %d workers.", numWorkers)

	jobs := make(chan string, numWorkers)
	var wg sync.WaitGroup

	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(i, &wg, jobs, keepJSON)
	}

	// --- 3. Directory Traversal ---
	go func() {
		defer close(jobs)
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
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
			log.Printf("Error walking the path %s: %v\n", dir, err)
		}
	}()

	// --- 4. Wait for Completion ---
	wg.Wait()
	fmt.Println("Processing complete.")
}

// worker defines the work each goroutine will perform.
func worker(id int, wg *sync.WaitGroup, jobs <-chan string, keepJSON *bool) {
	defer wg.Done()

	// Each worker gets its own persistent exiftool process.
	et, err := NewExifTool()
	if err != nil {
		log.Printf("Worker %d: Failed to start exiftool: %v", id, err)
		return
	}
	defer et.Close()

	log.Printf("Worker %d: Exiftool process started.", id)

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

		// --- 3. Convert Timestamp ---
		timestamp, err := strconv.ParseInt(meta.PhotoTakenTime.Timestamp, 10, 64)
		if err != nil {
			log.Printf("Worker %d: Could not parse timestamp in %s: %v", id, jsonPath, err)
			continue
		}
		t := time.Unix(timestamp, 0)
		formattedTime := t.Format("2006:01:02 15:04:05")
		dateTimeArg := fmt.Sprintf("-DateTimeOriginal=%s", formattedTime)

		// --- 4. Execute command via stay-open process ---
		log.Printf("Worker %d: Updating %s", id, imagePath)
		output, err := et.Execute("-overwrite_original", dateTimeArg, imagePath)
		if err != nil {
			log.Printf("Worker %d: Exiftool command failed for '%s': %v\nOutput: %s", id, imagePath, err, output)
			continue
		}

		// --- 5. Optionally delete JSON file on success ---
		if !*keepJSON {
			if err := os.Remove(jsonPath); err != nil {
				log.Printf("Worker %d: Warning: Could not delete JSON file %s: %v", id, jsonPath, err)
			} else {
				log.Printf("Worker %d: Successfully processed and deleted %s", id, jsonPath)
			}
		} else {
			log.Printf("Worker %d: Successfully processed %s (kept JSON file as requested)", id, jsonPath)
		}
	}
	log.Printf("Worker %d: Shutting down.", id)
}
