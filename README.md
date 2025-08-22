# EXIF Updater

A multi-mode photo organization tool for processing Google Photos Takeout data. This tool provides three distinct modes to scan, update, and organize your photo collection with proper EXIF timestamps and structured directories.

## Features

- **Three distinct modes**: scan, update, and sort for modular workflow
- **Scan Mode**: Analyze photos to report missing EXIF timestamp data
- **Update Mode**: Fix missing EXIF timestamps using Google Takeout JSON metadata
- **Sort Mode**: Organize photos into date-based directories with album symlinks
- **Parallel Processing**: Multi-worker architecture for efficient batch operations
- **Progress Tracking**: Real-time progress bars with ETA calculations
- **Dry-run Support**: Preview all operations before making changes
- **Flexible Options**: Keep originals, preserve JSON files, and more

## Prerequisites

- Go 1.16 or later
- [exiftool](https://github.com/exiftool/exiftool) (required for scan and update modes)

## Installation

### Run Directly from Repository

```bash
go run github.com/bryanbrunetti/exifupdater@latest -scan ~/google-takeout
```

### Build Locally

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

### Command Structure

```bash
exifupdater [mode] [options] <source_directory>
```

### Modes (choose exactly one)

- `-scan`: Scan files and report how many are missing EXIF timestamp data
- `-update`: Update EXIF timestamps from JSON metadata files  
- `-sort`: Sort files into `<year>/<month>/<day>` structure with album symlinks

### Options

- `-dest string`: Destination directory (required for sort mode)
- `-dry-run`: Show what would be done without making any changes
- `-keep-files`: Copy files instead of moving them (preserves originals)
- `-keep-json`: Keep JSON files after processing (don't delete them)

## Examples

### 1. Scan Mode - Analyze Your Collection

Check how many photos are missing EXIF timestamp data:

```bash
# Basic scan
./exifupdater -scan ~/google-takeout

# Example output:
# EXIF Updater - Multi-mode photo organization tool
# 
# Scanning directory: /Users/you/google-takeout
# Found 1247 media files to check
# Using 10 workers for scanning...
# 
# [===================>          ] 1247/1247 (100.0%) | Elapsed: 1m23s
# 
# === SCAN RESULTS ===
# Total media files scanned: 1247
# Files missing ALL timestamp data: 892
# Files with some timestamp data: 355
# Percentage missing timestamps: 71.5%
```

### 2. Update Mode - Fix EXIF Timestamps

Update EXIF timestamps from Google Takeout JSON metadata:

```bash
# Preview changes first
./exifupdater -update --dry-run ~/google-takeout

# Actually update the files
./exifupdater -update ~/google-takeout

# Keep JSON files after updating
./exifupdater -update --keep-json ~/google-takeout
```

### 3. Sort Mode - Organize Your Photos

Sort photos into a structured directory layout:

```bash
# Preview the organization
./exifupdater -sort --dry-run --dest ~/organized-photos ~/google-takeout

# Organize photos (moves files)
./exifupdater -sort --dest ~/organized-photos ~/google-takeout

# Copy files instead of moving (preserves originals)
./exifupdater -sort --keep-files --dest ~/organized-photos ~/google-takeout
```

## Typical Workflow

For processing Google Takeout data, use this recommended workflow:

```bash
# 1. First, scan to understand your collection
./exifupdater -scan ~/google-takeout

# 2. Update EXIF timestamps (if needed based on scan results)
./exifupdater -update --keep-json ~/google-takeout

# 3. Organize into structured directories
./exifupdater -sort --dest ~/organized-photos ~/google-takeout
```

## Directory Structure

The sort mode creates this organized structure:

```
/organized-photos/
├── 2023/
│   ├── 01/
│   │   └── 15/
│   │       ├── IMG_1234.jpg
│   │       └── VID_5678.mp4
│   └── 02/
│       └── 10/
│           └── IMG_9876.jpg
├── Family Vacation/              # Album from metadata.json
│   ├── IMG_1234.jpg -> ../2023/01/15/IMG_1234.jpg
│   └── VID_5678.mp4 -> ../2023/01/15/VID_5678.mp4
└── Birthday Party/               # Another album
    └── IMG_9876.jpg -> ../2023/02/10/IMG_9876.jpg
```

### Key Features:

- **Date-based Structure**: Main storage organized by `YYYY/MM/DD`
- **Album Directories**: Named after the "title" field in `metadata.json` files
- **Symbolic Links**: Album files link back to the date-based structure
- **No Duplicates**: Files with identical content are automatically handled

## How Each Mode Works

### Scan Mode

1. Recursively finds all media files in the source directory
2. Uses multiple workers to check each file's EXIF timestamp fields
3. Analyzes: DateTimeOriginal, MediaCreateDate, CreationDate, TrackCreateDate, CreateDate, DateTimeDigitized, GPSDateStamp, DateTime
4. Reports statistics and creates a log file of problematic files
5. Shows real-time progress with ETA calculations

### Update Mode

1. Finds all JSON metadata files from Google Takeout
2. Reads timestamp information from each JSON file
3. Locates corresponding image/video files using smart fallback logic
4. Updates EXIF timestamps using exiftool
5. Optionally removes JSON files after successful processing

### Sort Mode

1. Processes JSON metadata to extract timestamps and filenames
2. Creates date-based directory structure (`YYYY/MM/DD`)
3. Moves or copies media files to organized locations
4. Reads `metadata.json` files to identify album names
5. Creates album directories with symbolic links back to date structure

## Metadata Structure

The tool expects Google Takeout JSON files with this structure:

```json
{
  "title": "IMG_1234.jpg",
  "photoTakenTime": {
    "timestamp": "1640995200"
  }
}
```

Optional `metadata.json` files for albums:

```json
{
  "title": "Family Vacation 2023"
}
```

## Advanced Features

### Smart File Matching

The tool handles various filename edge cases:
- Truncated filenames (48, 47, 46 character limits)
- Different extension cases (.jpg vs .JPG)
- Various media formats (photos and videos)

### Duplicate Handling

- Files already at destination are skipped
- Album symlinks are still created for existing files
- Identical files are detected and handled appropriately

### Error Handling

- Detailed logging for troubleshooting
- Graceful handling of missing files or corrupted metadata
- Continue processing even if individual files fail

## Testing

Run the test suite:

```bash
# Run all tests
go test -v

# Run specific test
go test -v -run TestGetDateFromTimestamp

# Generate coverage report
go test -coverprofile=coverage.out && go tool cover -html=coverage.out
```

## Troubleshooting

### Common Issues

1. **"exiftool command not found"**: Install exiftool and ensure it's in your PATH
2. **"You must specify exactly one mode"**: Use only one of `-scan`, `-update`, or `-sort`
3. **Permission denied**: Ensure write permissions to destination directory
4. **No JSON files found**: Verify you're pointing to the correct Google Takeout directory

### Getting Help

1. Start with `-scan` to understand your collection
2. Always use `--dry-run` first to preview operations
3. Check the detailed log output for specific issues
4. Ensure your source directory structure matches Google Takeout format

### Mode-Specific Tips

**Scan Mode:**
- No destination directory needed
- Creates a timestamped log file of problematic files
- Safe to run multiple times

**Update Mode:**  
- Works in-place on your files
- Use `--keep-json` to preserve metadata files
- Use `--dry-run` to verify operations first

**Sort Mode:**
- Requires `-dest` destination directory
- Use `--keep-files` to copy instead of move
- Creates comprehensive directory structure with albums

## Performance

- **Multi-threaded**: Uses all available CPU cores by default
- **Memory efficient**: Processes files in batches
- **Progress tracking**: Real-time updates with ETA calculations
- **Optimized I/O**: Minimal file system operations per file

## License

This project is open source. See the repository for license details.