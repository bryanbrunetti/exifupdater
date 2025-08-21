# Example Usage

This document demonstrates how to use the updated EXIF timestamp updater with its new directory organization and album features.

## Sample Directory Structure

Let's say you have Google Takeout data that looks like this:

```
google-takeout/
├── Takeout/
│   └── Google Photos/
│       ├── Family Vacation 2023/
│       │   ├── IMG_1234.jpg
│       │   ├── IMG_1234.jpg.json
│       │   ├── VID_5678.mp4
│       │   ├── VID_5678.mp4.json
│       │   └── metadata.json
│       ├── Birthday Party/
│       │   ├── IMG_9876.jpg
│       │   ├── IMG_9876.jpg.json
│       │   └── metadata.json
│       └── Random Photos/
│           ├── IMG_4321.jpg
│           ├── IMG_4321.jpg.json
│           ├── IMG_8765.jpg
│           └── IMG_8765.jpg.json
```

### Sample JSON Files

**IMG_1234.jpg.json:**
```json
{
  "title": "IMG_1234.jpg",
  "description": "",
  "imageViews": "0",
  "creationTime": {
    "timestamp": "1672531200",
    "formatted": "Jan 1, 2023, 12:00:00 AM UTC"
  },
  "photoTakenTime": {
    "timestamp": "1672531200",
    "formatted": "Jan 1, 2023, 12:00:00 AM UTC"
  },
  "geoData": {
    "latitude": 0.0,
    "longitude": 0.0,
    "altitude": 0.0,
    "latitudeSpan": 0.0,
    "longitudeSpan": 0.0
  }
}
```

**Family Vacation 2023/metadata.json:**
```json
{
  "title": "Family Vacation 2023",
  "description": "Our summer trip to the beach",
  "access": "",
  "creationTime": {
    "timestamp": "1672531200",
    "formatted": "Jan 1, 2023, 12:00:00 AM UTC"
  }
}
```

**Birthday Party/metadata.json:**
```json
{
  "title": "Birthday Party",
  "description": "Sarah's 10th birthday celebration",
  "access": "",
  "creationTime": {
    "timestamp": "1675209600",
    "formatted": "Feb 1, 2023, 12:00:00 AM UTC"
  }
}
```

## Running the Tool

### Step 1: Scan to Understand Your Collection

First, scan your files to see how many need EXIF timestamp updates:

```bash
./exifupdater --scan ~/google-takeout/Takeout/Google\ Photos/
```

**Expected output with progress bar:**
```
Starting EXIF timestamp updater...
Scanning directory: /Users/you/google-takeout/Takeout/Google Photos/
Checking for missing EXIF timestamp data...
Looking for: DateTimeOriginal, MediaCreateDate, CreationDate, TrackCreateDate, CreateDate, DateTimeDigitized, GPSDateStamp, DateTime

Found 1247 media files to check
Analyzing files...
Using 10 workers for scanning...

[===================>          ] 892/1247 (71.5%) | Elapsed: 1m23s | ETA: 42s

=== SCAN RESULTS ===
Total media files scanned: 1247
Files missing ALL timestamp data: 892
Files with some timestamp data: 355
Percentage missing timestamps: 71.5%

Files missing timestamps would benefit from EXIF timestamp updating.
```

### Step 2: Preview with Dry Run

Next, run with `--dry-run` to see what would happen:

```bash
./exifupdater --dry-run --dest ~/organized-photos ~/google-takeout/Takeout/Google\ Photos/
```

**Expected output:**
```
Starting EXIF timestamp updater...
DRY RUN MODE: No files will be modified
Worker 1: [DRY RUN] Would update EXIF for /Users/you/google-takeout/Takeout/Google Photos/Family Vacation 2023/IMG_1234.jpg
Worker 1: [DRY RUN] Would move file: /Users/you/google-takeout/Takeout/Google Photos/Family Vacation 2023/IMG_1234.jpg -> /Users/you/organized-photos/ALL_PHOTOS/2023/01/01/IMG_1234.jpg
Worker 1: [DRY RUN] Would create directory: /Users/you/organized-photos/Family Vacation 2023
Worker 1: [DRY RUN] Would create symlink: /Users/you/organized-photos/Family Vacation 2023/IMG_1234.jpg -> ../ALL_PHOTOS/2023/01/01/IMG_1234.jpg
Worker 1: [DRY RUN] Would delete JSON file /Users/you/google-takeout/Takeout/Google Photos/Family Vacation 2023/IMG_1234.jpg.json
Worker 2: [DRY RUN] Would update EXIF for /Users/you/google-takeout/Takeout/Google Photos/Birthday Party/IMG_9876.jpg
...
```

### Step 3: Actually Process the Files

If the dry run looks good, run without the `--dry-run` flag:

```bash
./exifupdater --dest ~/organized-photos ~/google-takeout/Takeout/Google\ Photos/
```

### Step 4: Alternative Options

Keep JSON metadata files:
```bash
./exifupdater --keep-json --dest ~/organized-photos ~/google-takeout/Takeout/Google\ Photos/
```

Copy files instead of moving them (preserves originals):
```bash
./exifupdater --keep-files --dest ~/organized-photos ~/google-takeout/Takeout/Google\ Photos/
```

Combine options (copy files and keep JSON):
```bash
./exifupdater --keep-files --keep-json --dest ~/organized-photos ~/google-takeout/Takeout/Google\ Photos/
```

## Resulting Directory Structure

After processing, your `~/organized-photos` directory will look like this:

```
organized-photos/
├── ALL_PHOTOS/
│   ├── 2023/
│   │   ├── 01/
│   │   │   └── 01/
│   │   │       ├── IMG_1234.jpg
│   │   │       └── VID_5678.mp4
│   │   └── 02/
│   │       └── 01/
│   │           └── IMG_9876.jpg
│   └── 2023/
│       └── 03/
│           └── 15/
│               ├── IMG_4321.jpg
│               └── IMG_8765.jpg
├── Family Vacation 2023/
│   ├── IMG_1234.jpg -> ../ALL_PHOTOS/2023/01/01/IMG_1234.jpg
│   └── VID_5678.mp4 -> ../ALL_PHOTOS/2023/01/01/VID_5678.mp4
└── Birthday Party/
    └── IMG_9876.jpg -> ../ALL_PHOTOS/2023/02/01/IMG_9876.jpg
```

**Note:** Photos from the "Random Photos" folder won't have album symlinks because there was no `metadata.json` file with a title.

## Key Features Demonstrated

1. **Analysis capability**: Scan mode helps you understand your collection before processing
2. **Real-time progress tracking**: Progress bar with ETA shows scanning progress for large collections
3. **Multi-worker performance**: Parallel processing speeds up scanning significantly
4. **Date-based organization**: Files are organized by the date they were taken (from EXIF timestamp)
5. **Album preservation**: Albums are recreated as directories with symbolic links
6. **EXIF timestamp fixing**: All processed files get their EXIF timestamps updated
7. **Smart duplicate handling**: Files with the same name at the destination are skipped, but album symlinks are still created
8. **Safe preview**: Dry-run mode lets you see exactly what will happen
9. **Flexible file handling**: Choose to move files (default) or copy them (--keep-files)

## Tips

- **Start with `--scan`** to understand your collection and how many files need processing
- **Watch the progress bar** during scanning to estimate completion time for large collections
- **Always backup your original files first** (or use `--keep-files` to preserve originals)
- **Use absolute paths** to avoid confusion
- **Run dry-run first** to catch any issues
- **Check available disk space** before processing large collections (especially with `--keep-files`)
- **Review the logs** for any files that couldn't be processed
- **Consider `--keep-files`** if you want to preserve the original Google Takeout structure

## Troubleshooting

If you see errors like:
- `Image file 'filename.jpg' not found`: The JSON file exists but the corresponding image/video is missing
- `File already exists at destination`: A file with the same name already exists in the organized structure, but album symlinks will still be created/verified
- `Error creating symlink`: Your filesystem might not support symbolic links (rare on modern systems)
- `Error copying file`: Insufficient disk space or permission issues when using `--keep-files`
- `Scan shows 0% missing timestamps`: Your files already have proper EXIF data and may not need processing

## Workflow Recommendations

### For Large Collections
1. **Scan first**: `./exifupdater --scan ~/google-takeout` to understand scope
2. **Dry run**: `./exifupdater --dry-run --dest ~/organized --keep-files ~/google-takeout`
3. **Process**: `./exifupdater --dest ~/organized --keep-files ~/google-takeout`

### For Small Collections or Testing
1. **Scan first**: `./exifupdater --scan ~/test-photos`
2. **Dry run**: `./exifupdater --dry-run --dest ~/test-organized ~/test-photos`
3. **Process**: `./exifupdater --dest ~/test-organized ~/test-photos`
