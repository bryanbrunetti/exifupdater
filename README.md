# EXIF Timestamp Updater

This tool fixes missing EXIF timestamps from photos and videos exported from Google Photos via Takeout, and organizes them into a structured directory layout with album support.

## Features

- Processes multiple image files in parallel using worker goroutines
- Organizes photos into date-based directory structure
- Creates album directories with symbolic links based on metadata
- Handles various filename variations and edge cases
- Supports different image formats through exiftool
- Can optionally keep or delete JSON files after processing
- Can copy files instead of moving them (preserving originals)
- Dry-run mode to preview changes without making modifications
- Scan mode to analyze files for missing EXIF timestamp data (multi-worker with progress bar and ETA)
- Efficiently processes large numbers of files

## Prerequisites

- Go 1.16 or later
- [exiftool](https://github.com/exiftool/exiftool) (must be installed and available in system PATH)

## Run Remotely

You can build and run this tool directly off this repository with:

```
go run github.com/bryanbrunetti/exifupdater@latest
```

## Installation

1. Clone this repository:
   ```bash
   git clone https://github.com/bryanbrunetti/exifupdater.git
   cd exifupdater
   ```

2. Build the application:
   ```bash
   go build -o exifupdater
   ```

## Usage

```
Usage: exifupdater [options] <source_directory>
  source_directory  The root directory to scan for JSON files

Options:
  -dest string
        Destination directory for organized photos (required)
  -dry-run
        Show what would be done without making any changes
  -keep-files
        Copy files instead of moving them (preserves originals)
  -keep-json
        Keep JSON files after processing (don't delete them)
  -scan
        Scan files to report how many are missing EXIF timestamp data

The destination directory will be organized as:
  <dest>/ALL_PHOTOS/<year>/<month>/<day>/<filename>
  <dest>/<album_name>/<filename> (symlinks to ALL_PHOTOS)

Scan mode analyzes files for missing EXIF timestamp data:
  DateTimeOriginal, MediaCreateDate, CreationDate, TrackCreateDate,
  CreateDate, DateTimeDigitized, GPSDateStamp, DateTime
```

### Examples

**Scan files to see how many need EXIF timestamp updates:**
```bash
./exifupdater --scan ~/google-takeout
```

**Example scan output with progress bar:**
```
Starting EXIF timestamp updater...
Scanning directory: /Users/you/google-takeout
Found 1247 media files to check
Using 10 workers for scanning...

[===================>          ] 892/1247 (71.5%) | Elapsed: 1m23s | ETA: 42s

=== SCAN RESULTS ===
Total media files scanned: 1247
Files missing ALL timestamp data: 892
Files with some timestamp data: 355
Percentage missing timestamps: 71.5%
```

**Process files and organize them (dry-run first to preview):**
```bash
# Preview changes without making modifications
./exifupdater --dry-run --dest ~/organized-photos ~/google-takeout

# Actually process and organize the files
./exifupdater --dest ~/organized-photos ~/google-takeout
```

**Process files and keep JSON metadata:**
```bash
./exifupdater --keep-json --dest ~/organized-photos ~/google-takeout
```

**Process files by copying instead of moving (preserves originals):**
```bash
./exifupdater --keep-files --dest ~/organized-photos ~/google-takeout
```

**Combine options (copy files and keep JSON):**
```bash
./exifupdater --keep-files --keep-json --dest ~/organized-photos ~/google-takeout
```

## Directory Structure

The tool creates an organized directory structure in the destination folder:

```
/organized/photos/
├── ALL_PHOTOS/
│   └── 2023/
│       └── 01/
│           └── 15/
│               ├── IMG_1234.jpg
│               └── VID_5678.mp4
├── Family Vacation/
│   ├── IMG_1234.jpg -> ../ALL_PHOTOS/2023/01/15/IMG_1234.jpg
│   └── VID_5678.mp4 -> ../ALL_PHOTOS/2023/01/15/VID_5678.mp4
└── Birthday Party/
    └── IMG_9876.jpg -> ../ALL_PHOTOS/2023/02/10/IMG_9876.jpg
```

### Key Features:

- **ALL_PHOTOS**: Main storage organized by date (YYYY/MM/DD)
- **Album directories**: Named after the "title" field in `metadata.json` files
- **Symbolic links**: Files in album directories link to the main storage location
- **Smart duplicate handling**: Files with the same name at the destination are skipped, but album symlinks are still created

## How It Works

### Normal Processing Mode

1. The tool scans the specified source directory for `.json` files containing metadata from Google Takeout
2. For each JSON file found, it looks for the corresponding image/video file
3. It reads the timestamp from the JSON file and updates the EXIF data using exiftool
4. The file is moved (or copied with `--keep-files`) to the organized structure: `<dest>/ALL_PHOTOS/<year>/<month>/<day>/<filename>`
5. If a `metadata.json` file exists in the same directory with a "title" field:
   - Creates an album directory named after the title
   - Creates a symbolic link from the album to the organized file location
6. Optionally deletes the JSON file after successful processing

### Scan Mode (`--scan`)

1. The tool scans the specified directory for all media files (photos and videos)
2. Uses multiple worker processes (based on CPU cores) to analyze files in parallel for better performance
3. Displays a real-time progress bar with elapsed time and estimated time to completion (ETA)
4. For each media file, it checks for the presence of EXIF timestamp fields:
   - DateTimeOriginal, MediaCreateDate, CreationDate, TrackCreateDate
   - CreateDate, DateTimeDigitized, GPSDateStamp, DateTime
5. Files missing ALL of these timestamp fields are counted as needing updates
6. Provides a summary report showing total files vs files needing timestamp updates

The tool handles various filename variations including:
- Truncated filenames (48, 47, 46 character limits)
- Numbered suffixes (e.g., `_1.jpg` → `(1).jpg`)
- Different quote styles and characters
- Different extension cases

## Metadata Structure

The tool expects JSON files with this structure:
```json
{
  "title": "filename.jpg",
  "photoTakenTime": {
    "timestamp": "1640995200"
  }
}
```

And optional `metadata.json` files in the same directory:
```json
{
  "title": "Album Name"
}
```

## Error Handling

- Files that can't be processed are logged with appropriate error messages
- **Files already existing at the destination are skipped, but album symlinks are still created**
- The tool continues processing other files if an error occurs
- Detailed logs are printed to help diagnose any issues
- Dry-run mode allows you to preview all operations before execution

## Testing

The project includes a comprehensive test suite to ensure reliability. To run the tests:

```bash
# Run all tests
make test
# or
go test -v

# Run a specific test
make test TEST=TestNewExifTool
# or
go test -v -run TestNewExifTool
```

### Test Coverage

To generate a coverage report:

```bash
make cover
# or
go test -coverprofile=coverage.out && go tool cover -html=coverage.out
```

### Test Structure

- `TestNewExifTool`: Verifies the ExifTool instance is created with the correct arguments
- `TestExifTool_Execute`: Tests command execution against exiftool
- `TestPhotoMetadata_Unmarshal`: Tests JSON parsing of photo metadata
- `TestFindFileWithFallbacks`: Tests the file finding logic with various edge cases
- `TestCheckTruncatedName`: Tests the filename truncation logic

### Test Dependencies

- Tests require `exiftool` to be installed and available in the system PATH
- The test suite will be skipped if exiftool is not found

## Best Practices

1. **Start with --scan** to understand how many files need timestamp updates
2. **Always run with --dry-run first** to preview what will happen
3. **Make backups** of your original Google Takeout files before processing (or use `--keep-files`)
4. **Use absolute paths** for source and destination directories
5. **Check disk space** before processing large collections (especially when using `--keep-files`)
6. **Review the logs** for any files that couldn't be processed

## Troubleshooting

### Common Issues

1. **"exiftool command not found"**: Install exiftool and ensure it's in your PATH
2. **Permission denied**: Ensure you have write permissions to the destination directory
3. **Files not found**: Check the filename variations - the tool handles many cases but some edge cases might exist
4. **Symlink creation fails**: Ensure the filesystem supports symbolic links
5. **"File already exists at destination"**: The file is already organized, but album symlinks will still be created/verified
6. **Scan shows 0% missing timestamps**: Your files may already have proper EXIF data and don't need processing

### Getting Help

If you encounter issues:
1. Start with `--scan` to understand your file collection
2. Run with `--dry-run` to see what the tool would do
3. Check the detailed log output for specific error messages
4. Verify your source directory structure matches Google Takeout format
5. Ensure sufficient disk space and permissions