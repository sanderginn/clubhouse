# Phase 4, Step 1: Bandcamp JSON-LD Extraction

## Overview

Improve Bandcamp metadata extraction reliability by parsing JSON-LD structured data instead of relying on `bc-page-properties`.

## Detailed Description

Modify `backend/internal/services/links/bandcamp.go` to:

1. Add JSON-LD extraction as the primary method for getting item type and ID
2. Keep existing `bc-page-properties` parsing as fallback
3. Extract richer metadata from JSON-LD (album art, artist, release date)

### Why JSON-LD?

The current `bc-page-properties` extraction is fragile:
- Bandcamp can change the data attribute format
- Not all pages have consistent `bc-page-properties`
- JSON-LD is a web standard that Bandcamp uses for SEO

### JSON-LD Structure on Bandcamp Pages

Bandcamp album pages contain:
```html
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "MusicAlbum",
  "name": "Album Title",
  "byArtist": {
    "@type": "MusicGroup",
    "name": "Artist Name"
  },
  "image": "https://f4.bcbits.com/img/a123456_10.jpg",
  "albumRelease": [{
    "@type": "MusicRelease",
    "additionalProperty": [{
      "@type": "PropertyValue",
      "name": "item_id",
      "value": 123456789
    }]
  }],
  "track": {
    "@type": "ItemList",
    "numberOfItems": 10,
    "itemListElement": [...]
  }
}
</script>
```

Track pages have similar structure with `@type": "MusicRecording"`.

### Embed URL Format

Once we have the item type and ID:
- Album: `https://bandcamp.com/EmbeddedPlayer/album={id}/size=large/`
- Track: `https://bandcamp.com/EmbeddedPlayer/track={id}/size=large/`

## Files to Modify

| File | Changes |
|------|---------|
| `backend/internal/services/links/bandcamp.go` | Add JSON-LD extraction, update extraction logic |
| `backend/internal/services/links/bandcamp_test.go` | Add tests for JSON-LD parsing |

## Expected Outcomes

1. JSON-LD extraction successfully parses album and track pages
2. Falls back to `bc-page-properties` when JSON-LD is unavailable
3. Extracts richer metadata (title, artist, image) from JSON-LD
4. Handles malformed JSON-LD gracefully
5. All existing tests pass
6. New tests cover JSON-LD scenarios

## Implementation

```go
package links

import (
    "encoding/json"
    "regexp"
    "strconv"
    "strings"
)

// BandcampJSONLD represents the JSON-LD structure on Bandcamp pages
type BandcampJSONLD struct {
    Type     string `json:"@type"`
    Name     string `json:"name"`
    ByArtist struct {
        Name string `json:"name"`
    } `json:"byArtist"`
    Image        string `json:"image"`
    AlbumRelease []struct {
        AdditionalProperty []struct {
            Name  string      `json:"name"`
            Value interface{} `json:"value"` // Can be int or string
        } `json:"additionalProperty"`
    } `json:"albumRelease"`
    // For tracks
    InAlbum struct {
        AdditionalProperty []struct {
            Name  string      `json:"name"`
            Value interface{} `json:"value"`
        } `json:"additionalProperty"`
    } `json:"inAlbum"`
}

// extractBandcampFromJSONLD attempts to extract item type and ID from JSON-LD
func extractBandcampFromJSONLD(body []byte) (itemType string, itemID int64, metadata map[string]interface{}, err error) {
    // Find JSON-LD script tag
    jsonLDRegex := regexp.MustCompile(`<script type="application/ld\+json">\s*(\{[\s\S]*?\})\s*</script>`)
    matches := jsonLDRegex.FindSubmatch(body)
    if len(matches) < 2 {
        return "", 0, nil, fmt.Errorf("no JSON-LD found")
    }

    var ld BandcampJSONLD
    if err := json.Unmarshal(matches[1], &ld); err != nil {
        return "", 0, nil, fmt.Errorf("failed to parse JSON-LD: %w", err)
    }

    // Determine item type
    switch ld.Type {
    case "MusicAlbum":
        itemType = "album"
        // Get album ID from albumRelease
        for _, release := range ld.AlbumRelease {
            for _, prop := range release.AdditionalProperty {
                if prop.Name == "item_id" {
                    itemID = toInt64(prop.Value)
                    break
                }
            }
            if itemID != 0 {
                break
            }
        }
    case "MusicRecording":
        itemType = "track"
        // Get track ID from inAlbum or direct property
        for _, prop := range ld.InAlbum.AdditionalProperty {
            if prop.Name == "item_id" {
                itemID = toInt64(prop.Value)
                break
            }
        }
    default:
        return "", 0, nil, fmt.Errorf("unsupported JSON-LD type: %s", ld.Type)
    }

    if itemID == 0 {
        return "", 0, nil, fmt.Errorf("item_id not found in JSON-LD")
    }

    // Build metadata from JSON-LD
    metadata = map[string]interface{}{
        "title":     ld.Name,
        "artist":    ld.ByArtist.Name,
        "image":     ld.Image,
        "site_name": "Bandcamp",
    }

    return itemType, itemID, metadata, nil
}

// toInt64 converts interface{} to int64
func toInt64(v interface{}) int64 {
    switch val := v.(type) {
    case float64:
        return int64(val)
    case int:
        return int64(val)
    case int64:
        return val
    case string:
        i, _ := strconv.ParseInt(val, 10, 64)
        return i
    default:
        return 0
    }
}

// ExtractBandcampMetadata extracts metadata from a Bandcamp page
// Updated to try JSON-LD first, then fall back to bc-page-properties
func ExtractBandcampMetadata(body []byte, url string) (map[string]interface{}, error) {
    var itemType string
    var itemID int64
    var metadata map[string]interface{}

    // Try JSON-LD first
    itemType, itemID, metadata, err := extractBandcampFromJSONLD(body)

    // Fall back to bc-page-properties if JSON-LD fails
    if err != nil || itemID == 0 {
        itemType, itemID, err = extractBandcampFromProperties(body)
        if err != nil {
            return nil, fmt.Errorf("failed to extract Bandcamp ID: %w", err)
        }
        // Build basic metadata without JSON-LD enrichment
        metadata = map[string]interface{}{
            "site_name": "Bandcamp",
        }
        // Try to get title from og:title
        if title := extractOGTag(body, "og:title"); title != "" {
            metadata["title"] = title
        }
        if image := extractOGTag(body, "og:image"); image != "" {
            metadata["image"] = image
        }
    }

    // Build embed URL
    embedURL := buildBandcampEmbedURL(itemType, itemID)
    metadata["embed"] = map[string]interface{}{
        "provider": "bandcamp",
        "embedUrl": embedURL,
        "height":   120, // Bandcamp embed height
    }

    return metadata, nil
}

func buildBandcampEmbedURL(itemType string, itemID int64) string {
    return fmt.Sprintf("https://bandcamp.com/EmbeddedPlayer/%s=%d/size=large/bgcol=ffffff/linkcol=0687f5/tracklist=false/artwork=small/transparent=true/", itemType, itemID)
}

// extractBandcampFromProperties is the existing bc-page-properties extraction
// Keep this as fallback
func extractBandcampFromProperties(body []byte) (itemType string, itemID int64, err error) {
    // ... existing implementation ...
}
```

## Test Cases

```go
func TestExtractBandcampFromJSONLD_Album(t *testing.T) {
    html := []byte(`
<!DOCTYPE html>
<html>
<head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "MusicAlbum",
  "name": "Test Album",
  "byArtist": {
    "@type": "MusicGroup",
    "name": "Test Artist"
  },
  "image": "https://f4.bcbits.com/img/a123456_10.jpg",
  "albumRelease": [{
    "@type": "MusicRelease",
    "additionalProperty": [{
      "@type": "PropertyValue",
      "name": "item_id",
      "value": 987654321
    }]
  }]
}
</script>
</head>
</html>
`)

    itemType, itemID, metadata, err := extractBandcampFromJSONLD(html)

    require.NoError(t, err)
    assert.Equal(t, "album", itemType)
    assert.Equal(t, int64(987654321), itemID)
    assert.Equal(t, "Test Album", metadata["title"])
    assert.Equal(t, "Test Artist", metadata["artist"])
    assert.Equal(t, "https://f4.bcbits.com/img/a123456_10.jpg", metadata["image"])
}

func TestExtractBandcampFromJSONLD_Track(t *testing.T) {
    html := []byte(`
<!DOCTYPE html>
<html>
<head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "MusicRecording",
  "name": "Test Track",
  "byArtist": {
    "@type": "MusicGroup",
    "name": "Test Artist"
  },
  "image": "https://f4.bcbits.com/img/a123456_10.jpg",
  "inAlbum": {
    "@type": "MusicAlbum",
    "additionalProperty": [{
      "@type": "PropertyValue",
      "name": "item_id",
      "value": 111222333
    }]
  }
}
</script>
</head>
</html>
`)

    itemType, itemID, metadata, err := extractBandcampFromJSONLD(html)

    require.NoError(t, err)
    assert.Equal(t, "track", itemType)
    assert.Equal(t, int64(111222333), itemID)
    assert.Equal(t, "Test Track", metadata["title"])
}

func TestExtractBandcampFromJSONLD_NoJSONLD(t *testing.T) {
    html := []byte(`<!DOCTYPE html><html><head></head></html>`)

    _, _, _, err := extractBandcampFromJSONLD(html)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "no JSON-LD found")
}

func TestExtractBandcampFromJSONLD_InvalidJSON(t *testing.T) {
    html := []byte(`
<script type="application/ld+json">
{ invalid json here }
</script>
`)

    _, _, _, err := extractBandcampFromJSONLD(html)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to parse JSON-LD")
}

func TestExtractBandcampFromJSONLD_MissingItemID(t *testing.T) {
    html := []byte(`
<script type="application/ld+json">
{
  "@type": "MusicAlbum",
  "name": "Test Album",
  "albumRelease": []
}
</script>
`)

    _, _, _, err := extractBandcampFromJSONLD(html)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "item_id not found")
}

func TestExtractBandcampMetadata_FallbackToProperties(t *testing.T) {
    // HTML with bc-page-properties but no JSON-LD
    html := []byte(`
<!DOCTYPE html>
<html>
<head>
<meta property="og:title" content="Fallback Album">
<meta property="og:image" content="https://example.com/image.jpg">
</head>
<body>
<div data-bc-page-properties='{"item_type":"album","item_id":123456}'></div>
</body>
</html>
`)

    metadata, err := ExtractBandcampMetadata(html, "https://artist.bandcamp.com/album/test")

    require.NoError(t, err)
    assert.Equal(t, "Fallback Album", metadata["title"])
    embed := metadata["embed"].(map[string]interface{})
    assert.Contains(t, embed["embedUrl"], "album=123456")
}

func TestBuildBandcampEmbedURL(t *testing.T) {
    tests := []struct {
        itemType string
        itemID   int64
        expected string
    }{
        {"album", 123456, "https://bandcamp.com/EmbeddedPlayer/album=123456/size=large/bgcol=ffffff/linkcol=0687f5/tracklist=false/artwork=small/transparent=true/"},
        {"track", 789012, "https://bandcamp.com/EmbeddedPlayer/track=789012/size=large/bgcol=ffffff/linkcol=0687f5/tracklist=false/artwork=small/transparent=true/"},
    }

    for _, tt := range tests {
        result := buildBandcampEmbedURL(tt.itemType, tt.itemID)
        assert.Equal(t, tt.expected, result)
    }
}

func TestToInt64(t *testing.T) {
    tests := []struct {
        input    interface{}
        expected int64
    }{
        {float64(123456), 123456},
        {int(789), 789},
        {int64(999), 999},
        {"12345", 12345},
        {"invalid", 0},
        {nil, 0},
    }

    for _, tt := range tests {
        result := toInt64(tt.input)
        assert.Equal(t, tt.expected, result)
    }
}
```

## Verification

```bash
# Run Bandcamp tests
cd backend && go test ./internal/services/links/bandcamp_test.go -v

# Run all link service tests
cd backend && go test ./internal/services/links/... -v

# Manual testing with real URLs
# 1. Start dev server
# 2. Post a Bandcamp album URL
# 3. Verify embed appears correctly
# 4. Check logs for extraction method used
```

## Notes

- The JSON-LD regex needs to handle minified JSON (no whitespace)
- Some Bandcamp pages might have multiple JSON-LD blocks - take the first MusicAlbum/MusicRecording
- The `value` field in `additionalProperty` can be either int or string depending on the page
- Keep the existing `bc-page-properties` code as fallback for older pages or edge cases
- Consider adding logging to track which extraction method succeeds (for monitoring)
- The JSON-LD may contain additional useful fields (release date, genre, duration) for future enrichment
