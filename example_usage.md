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

### Step 1: Preview with Dry Run

First, always run with `--dry-run` to see what would happen:

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

### Step 2: Actually Process the Files

If the dry run looks good, run without the `--dry-run` flag:

```bash
./exifupdater --dest ~/organized-photos ~/google-takeout/Takeout/Google\ Photos/
```

### Step 3: Keep JSON Files (Optional)

If you want to preserve the JSON metadata files:

```bash
./exifupdater --keep-json --dest ~/organized-photos ~/google-takeout/Takeout/Google\ Photos/
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

1. **Date-based organization**: Files are organized by the date they were taken (from EXIF timestamp)
2. **Album preservation**: Albums are recreated as directories with symbolic links
3. **EXIF timestamp fixing**: All processed files get their EXIF timestamps updated
4. **Duplicate handling**: Files with the same name at the destination are skipped
5. **Safe preview**: Dry-run mode lets you see exactly what will happen

## Tips

- **Always backup your original files first**
- **Use absolute paths** to avoid confusion
- **Run dry-run first** to catch any issues
- **Check available disk space** before processing large collections
- **Review the logs** for any files that couldn't be processed

## Troubleshooting

If you see errors like:
- `Image file 'filename.jpg' not found`: The JSON file exists but the corresponding image/video is missing
- `File already exists at destination`: A file with the same name already exists in the organized structure
- `Error creating symlink`: Your filesystem might not support symbolic links (rare on modern systems)
