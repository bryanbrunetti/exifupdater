# EXIF Updater - Example Usage

This document provides detailed examples of how to use the EXIF Updater tool in its three different modes.

## Quick Start

```bash
# 1. Scan your Google Takeout data
./exifupdater -scan ~/Downloads/takeout-20240101-001234

# 2. Update EXIF timestamps (if needed)
./exifupdater -update ~/Downloads/takeout-20240101-001234

# 3. Organize photos into structured directories
./exifupdater -sort -dest ~/organized-photos ~/Downloads/takeout-20240101-001234
```

## Mode 1: Scan - Analyze Your Collection

### Basic Scanning

```bash
# Scan all media files for missing EXIF timestamps
./exifupdater -scan ~/google-takeout
```

**Example Output:**
```
EXIF Updater - Multi-mode photo organization tool

Scanning directory: /Users/john/google-takeout
Found 3,247 media files to check
Using 8 workers for scanning...

[==============================] 3247/3247 (100.0%) | Elapsed: 2m15s

=== SCAN RESULTS ===
Total media files scanned: 3247
Files missing ALL timestamp data: 1,892
Files with some timestamp data: 1,355
Percentage missing timestamps: 58.3%
```

### Understanding Scan Results

- **Files missing ALL timestamp data**: These files have no EXIF timestamp information and would benefit from the update mode
- **Files with some timestamp data**: These files have at least one timestamp field populated
- **Log file**: A timestamped log file `missing_timestamps_YYYYMMDD_HHMMSS.log` is created with paths to all problematic files

### When to Use Scan Mode

- Before processing a new Google Takeout archive
- To assess the scope of EXIF issues in your collection
- To verify results after running update mode

## Mode 2: Update - Fix EXIF Timestamps

### Preview Updates (Recommended First Step)

```bash
# See what would be updated without making changes
./exifupdater -update --dry-run ~/google-takeout
```

**Example Output:**
```
EXIF Updater - Multi-mode photo organization tool

🔍 DRY RUN MODE: No files will be modified

UPDATE MODE: Updating EXIF timestamps from JSON metadata...
Found 1,247 JSON files to process
[DRY RUN] Would update EXIF for /path/to/IMG_1234.jpg
[DRY RUN] Would delete JSON file /path/to/IMG_1234.jpg.json
[DRY RUN] Would update EXIF for /path/to/VID_5678.mp4
[DRY RUN] Would delete JSON file /path/to/VID_5678.mp4.json
[==============================] 1247/1247 (100.0%) | Elapsed: 45s
Update complete! Processed 1,247 JSON files.
```

### Actual Updates

```bash
# Update EXIF timestamps from JSON metadata
./exifupdater -update ~/google-takeout
```

### Keep JSON Files

```bash
# Update timestamps but preserve the JSON metadata files
./exifupdater -update --keep-json ~/google-takeout
```

### When to Use Update Mode

- After scanning shows files missing EXIF timestamps
- Before organizing files (sort mode works better with proper timestamps)
- When you want to fix EXIF data in-place without reorganizing

## Mode 3: Sort - Organize Your Photos

### Preview Organization (Always Recommended First)

```bash
# See how files would be organized
./exifupdater -sort --dry-run --dest ~/organized-photos ~/google-takeout
```

**Example Output:**
```
EXIF Updater - Multi-mode photo organization tool

🔍 DRY RUN MODE: No files will be modified

SORT MODE: Organizing files into date-based structure with album symlinks...
[DRY RUN] Would create directory: /Users/john/organized-photos
Found 1,247 JSON files to process
[DRY RUN] Would move file: /path/to/IMG_1234.jpg -> /Users/john/organized-photos/2023/01/15/IMG_1234.jpg
[DRY RUN] Would create directory: /Users/john/organized-photos/Family Vacation
[DRY RUN] Would create symlink: /Users/john/organized-photos/Family Vacation/IMG_1234.jpg -> ../2023/01/15/IMG_1234.jpg
[==============================] 1247/1247 (100.0%) | Elapsed: 1m12s
Sort complete! Processed 1,247 JSON files.
```

### Move Files (Default Behavior)

```bash
# Move files from source to organized destination
./exifupdater -sort --dest ~/organized-photos ~/google-takeout
```

### Copy Files (Preserve Originals)

```bash
# Copy files instead of moving them
./exifupdater -sort --keep-files --dest ~/organized-photos ~/google-takeout
```

### Resulting Directory Structure

After running sort mode, you'll get:

```
~/organized-photos/
├── 2023/
│   ├── 01/
│   │   ├── 15/
│   │   │   ├── IMG_1234.jpg
│   │   │   └── IMG_1235.jpg
│   │   └── 16/
│   │       └── VID_5678.mp4
│   └── 02/
│       └── 10/
│           └── IMG_9876.jpg
├── 2024/
│   └── 03/
│       └── 22/
│           └── IMG_0001.heic
├── Family Vacation/           # Album from metadata.json
│   ├── IMG_1234.jpg -> ../2023/01/15/IMG_1234.jpg
│   ├── IMG_1235.jpg -> ../2023/01/15/IMG_1235.jpg
│   └── VID_5678.mp4 -> ../2023/01/16/VID_5678.mp4
└── Birthday Party 2023/       # Another album
    └── IMG_9876.jpg -> ../2023/02/10/IMG_9876.jpg
```

### When to Use Sort Mode

- When you want to organize photos chronologically
- To create album directories based on Google Photos albums
- After updating EXIF timestamps (for best results)

## Comprehensive Workflows

### Workflow 1: Complete Google Takeout Processing

```bash
# Step 1: Understand your data
./exifupdater -scan ~/google-takeout

# Step 2: Fix missing timestamps (if scan showed issues)
./exifupdater -update --dry-run ~/google-takeout  # Preview first
./exifupdater -update --keep-json ~/google-takeout  # Keep JSON for reference

# Step 3: Organize into structured directories
./exifupdater -sort --dry-run --dest ~/Photos ~/google-takeout  # Preview
./exifupdater -sort --dest ~/Photos ~/google-takeout
```

### Workflow 2: Conservative Approach (Preserve Everything)

```bash
# Scan without making changes
./exifupdater -scan ~/google-takeout

# Update timestamps but keep JSON files
./exifupdater -update --keep-json ~/google-takeout

# Organize by copying (preserve original structure)
./exifupdater -sort --keep-files --dest ~/Photos-Organized ~/google-takeout
```

### Workflow 3: Quick Organization (Skip Updates)

If your photos already have good EXIF data:

```bash
# Quick scan to confirm
./exifupdater -scan ~/google-takeout

# Skip update mode and go straight to organization
./exifupdater -sort --dest ~/Photos ~/google-takeout
```

## Common Scenarios

### Large Collections (10,000+ photos)

```bash
# Use dry-run first to estimate time and space requirements
./exifupdater -sort --dry-run --dest /Volumes/ExternalDrive/Photos ~/google-takeout

# Monitor progress with the built-in progress bars
./exifupdater -sort --dest /Volumes/ExternalDrive/Photos ~/google-takeout
```

### Multiple Takeout Archives

```bash
# Process each archive separately
./exifupdater -update ~/takeout-2023
./exifupdater -update ~/takeout-2024

# Then organize all into one destination
./exifupdater -sort --dest ~/All-Photos ~/takeout-2023
./exifupdater -sort --dest ~/All-Photos ~/takeout-2024
```

### Network Storage

```bash
# Copy mode recommended for network storage
./exifupdater -sort --keep-files --dest /mnt/nas/Photos ~/google-takeout
```

## Tips and Best Practices

### Before You Start

1. **Always use dry-run first**: `--dry-run` shows exactly what will happen
2. **Check available disk space**: Sort mode with `--keep-files` doubles storage requirements
3. **Backup important data**: Though the tool is safe, backups are always wise

### During Processing

1. **Monitor progress bars**: Built-in ETA helps plan your time
2. **Check logs**: Any errors or warnings are displayed in real-time
3. **Don't interrupt**: Let each mode complete fully for best results

### After Processing

1. **Verify results**: Quick spot-check of organized directories
2. **Clean up**: Remove original takeout files if you used move operations
3. **Test album links**: Verify symbolic links work on your system

### Performance Tips

- **SSD storage**: Significantly faster than traditional hard drives
- **Local processing**: Network storage can slow operations considerably
- **Sufficient RAM**: Tool is memory-efficient but benefits from adequate RAM

## Troubleshooting Examples

### Problem: "No JSON files found"

```bash
# Check your directory structure
ls -la ~/google-takeout/
# Should show directories like "Google Photos", "Takeout", etc.

# Look for actual JSON files
find ~/google-takeout -name "*.json" | head -10
```

### Problem: "exiftool command not found"

```bash
# Install exiftool (macOS with Homebrew)
brew install exiftool

# Install exiftool (Ubuntu/Debian)
sudo apt-get install exiftool

# Verify installation
which exiftool
```

### Problem: Symlinks not working

```bash
# Check if your filesystem supports symlinks
ln -s /tmp/test /tmp/testlink
ls -la /tmp/testlink

# Some Windows filesystems and cloud storage don't support symlinks
# Use --keep-files to copy instead
```

### Problem: Running out of space

```bash
# Check space requirements first
du -sh ~/google-takeout  # Source size
df -h ~/destination      # Available space

# Use copy mode only if you have 2x the space
./exifupdater -sort --keep-files --dest ~/Photos ~/google-takeout
```

## Advanced Usage

### Custom Destination Structure

The tool creates `YYYY/MM/DD` structure, but you can process multiple times:

```bash
# Organize by year first
./exifupdater -sort --dest ~/Photos-by-Year ~/google-takeout

# Then manually reorganize subsets as needed
```

### Selective Processing

```bash
# Process only a subset of your takeout
./exifupdater -scan ~/google-takeout/Google\ Photos/Photos\ from\ 2023

# Update just that subset
./exifupdater -update ~/google-takeout/Google\ Photos/Photos\ from\ 2023
```

### Integration with Other Tools

```bash
# After organizing, generate thumbnails
find ~/organized-photos -name "*.jpg" -exec convert {} -resize 200x200 {}_thumb.jpg \;

# Create a photo index
find ~/organized-photos -name "*.jpg" > photo-inventory.txt
```
