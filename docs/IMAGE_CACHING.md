# Image Caching System

## Overview

MTGSim now implements automatic image caching to avoid re-downloading card images from Scryfall. Images are cached locally and reused across runs.

## How It Works

### Cache Structure

```
.cache/scryfall/
├── images/
│   ├── <hash>.jpg  (cached card images)
│   └── ...
└── <cardname>.json (card metadata)
```

### Caching Process

1. **URL Retrieval**: When a card's image URL is needed, it's fetched from:
   - In-memory cache (fastest)
   - Card database cache (fast)
   - Scryfall API (slower, cached locally)

2. **Image Download**: The image file is downloaded and cached:
   - Each image URL is converted to a unique hash-based filename
   - Images are stored in `.cache/scryfall/images/`
   - Downloads happen asynchronously to not block the main process
   - **Only happens once per unique image** - subsequent runs reuse the cached file

3. **Reuse**: Future references to the same image:
   - Check if image is already cached locally
   - If yes, use the cached file immediately
   - If no, download and cache

## Performance Benefits

- **First Run**: Images are downloaded as cards are processed
- **Subsequent Runs**: Images are loaded from cache (no network request)
- **Bandwidth Savings**: Significant reduction in network usage for frequently played cards
- **Speed**: Faster card display in dashboard/UI with cached images

## Cache Location

All images are cached in: `.cache/scryfall/images/`

To clear the image cache:
```bash
rm -rf .cache/scryfall/images/
```

To clear all Scryfall caches (metadata + images):
```bash
rm -rf .cache/scryfall/
```

## Implementation Details

### New Methods in `pkg/scryfall/client.go`

- **`DownloadAndCacheImage(url string) (string, error)`**
  - Downloads an image from a URL and caches it locally
  - Returns the local file path if successful
  - Returns empty string and error if download fails
  - Automatically checks if image already exists in cache

- **`GetCachedImagePath(url string) string`**
  - Returns the cached file path for a URL if it exists
  - Returns empty string if not cached
  - Useful for checking if an image is already cached

- **`urlToCacheKey(url string) string`**
  - Converts a Scryfall image URL to a safe cache filename
  - Preserves file extension (.jpg or .png)
  - Uses URL hash for unique identification

### Integration Points

1. **mtgsim-edh**: Images are cached asynchronously during game simulations
2. **mtgsim**: Images are cached during card library enrichment
3. **Fallback**: If caching fails, URLs are still available for web display

## Compliance

- Follows [Scryfall's Fan Content Policy](https://scryfall.com/docs/api)
- Proper User-Agent headers
- No modifications to images
- Original images preserved without cropping or watermarking

## Troubleshooting

### Images not caching
- Check disk space in `.cache/` directory
- Verify permissions on `.cache/scryfall/images/`
- Check network connectivity to Scryfall CDN (cards.scryfall.io)

### Cache issues
- Delete the image cache: `rm -rf .cache/scryfall/images/`
- Re-run simulations to rebuild cache

### Memory usage
- Images are downloaded and cached to disk, not held in memory
- No impact on application memory usage
