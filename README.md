# EXIF Timestamp Updater
This tool fixes missing EXIF timestamps from photos and videos exported from Google Photos via Takeout.

## Features
- Processes multiple image files in parallel using worker goroutines
- Handles various filename variations and edge cases
- Supports different image formats through exiftool
- Can optionally keep or delete JSON files after processing
- Efficiently processes large numbers of files

## Prerequisites

- Go 1.16 or later
- [exiftool](https://github.com/exiftool/exiftool) (must be installed and available in system PATH)

## Installation

1. Clone this repository:
   ```bash
   git clone https://github.com/bryanbrunetti/exifupdater.git
   cd ExifUpdater
   ```

2. Build the application:
   ```bash
   go build -o exifupdater
   ```

## Usage

```
Usage: exifupdater [options] [directory]
  directory  The root directory to scan for JSON files (default: current directory)

Options:
  -keep-json    Keep JSON files after processing (don't delete them)
```

### Examples

Process files in the current directory and delete JSON files after processing:
```bash
./exifupdater
```

Process files in a specific directory and keep JSON files:
```bash
./exifupdater -keep-json /path/to/photos
```

## How It Works

1. The tool scans the specified directory (or current directory) for `.json` file containing metadata from Google Takeout
2. For each JSON file found, it looks for the corresponding image file
3. It reads the timestamp from the JSON file
4. Updates the EXIF data of the matching image file using exiftool
5. Optionally deletes the JSON file after successful processing

The tool handles various filename variations including:
- Truncated filenames
- Numbered suffixes (e.g., `_1.jpg` → `(1).jpg`)
- Different quote styles
- Different filename cases

## Error Handling

- Files that can't be processed are logged with appropriate error messages
- The tool continues processing other files if an error occurs
- Detailed logs are printed to help diagnose any issues
