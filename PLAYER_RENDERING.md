# Player Rendering & Link Metadata Architecture

This document analyzes the current link metadata fetching and embed player rendering implementation, identifies issues with the current approach, and provides recommendations for improvement.

## Table of Contents

- [Problem Statement](#problem-statement)
- [Current Architecture](#current-architecture)
- [Provider Analysis](#provider-analysis)
  - [YouTube](#youtube)
  - [Spotify](#spotify)
  - [SoundCloud](#soundcloud)
  - [Bandcamp](#bandcamp)
- [CORS Testing Results](#cors-testing-results)
- [Recommendations](#recommendations)
- [Proposed Architecture](#proposed-architecture)
- [Implementation Guide](#implementation-guide)
- [Code to Remove](#code-to-remove)

---

## Problem Statement

The current implementation fetches link metadata on the backend during post creation. This approach has several issues:

1. **Service Blocking**: Services like Bandcamp detect server requests as scrapers and block them, even with the azuretls client that mimics Chrome's TLS fingerprint.

2. **Synchronous Delays**: Metadata fetching happens synchronously during post creation, blocking the user until all fetches complete.

3. **Unnecessary Server Load**: For some providers (YouTube, Spotify), the backend performs HTTP requests when simple URL parsing would suffice.

4. **Single Point of Failure**: If the server IP gets rate-limited or blocked, all users lose embed functionality.

---

## Current Architecture

### Backend Flow

```
POST /api/v1/posts (with link)
    │
    ▼
PostService.CreatePost()
    │
    ├─► fetchLinkMetadata(ctx, links)
    │       │
    │       ├─► ExtractEmbed(ctx, url)
    │       │       └─► SpotifyExtractor (URL parsing only)
    │       │
    │       └─► FetchMetadata(ctx, url)
    │               ├─► HTTP fetch (2MB max, 5s timeout)
    │               ├─► extractHTMLMeta() - OpenGraph, Twitter Cards
    │               ├─► detectProvider()
    │               ├─► parseRecipe() - JSON-LD/microdata
    │               └─► extractEmbed() via registry
    │                       ├─► BandcampExtractor.ExtractFromHTML()
    │                       ├─► SoundCloudExtractor.Extract() → oEmbed API
    │                       └─► YouTubeExtractor.Extract()
    │
    ▼
INSERT INTO links (url, metadata JSONB)
    │
    ▼
Return post to client
```

### Frontend Flow

```
PostCard receives post with metadata
    │
    ├─► Detect embed provider from metadata.embed.provider
    │
    ├─► YouTube?  → <YouTubeEmbed embedUrl={...} />
    ├─► Spotify?  → <SpotifyEmbed embedUrl={...} height={...} />
    ├─► SoundCloud? → <SoundCloudEmbed embedUrl={...} /> + Widget API
    └─► Bandcamp? → <BandcampEmbed embed={...} />
```

### Key Files

**Backend:**
- `backend/internal/services/links/metadata.go` - Main fetcher logic
- `backend/internal/services/links/embed.go` - Embed interfaces
- `backend/internal/services/links/embed_registry.go` - Embed router
- `backend/internal/services/links/youtube.go` - YouTube extractor
- `backend/internal/services/links/spotify.go` - Spotify extractor
- `backend/internal/services/links/soundcloud.go` - SoundCloud extractor
- `backend/internal/services/links/bandcamp.go` - Bandcamp extractor
- `backend/internal/services/links/bandcamp_fetch.go` - azuretls client for WAF bypass
- `backend/internal/services/link_metadata.go` - Called during post creation
- `backend/internal/middleware/csp.go` - Content Security Policy

**Frontend:**
- `frontend/src/lib/components/embeds/YouTubeEmbed.svelte`
- `frontend/src/lib/components/embeds/SpotifyEmbed.svelte`
- `frontend/src/lib/components/embeds/SoundCloudEmbed.svelte`
- `frontend/src/lib/components/embeds/BandcampEmbed.svelte`
- `frontend/src/lib/embeds/soundcloudApi.ts` - Widget API loader
- `frontend/src/lib/embeds/controller.ts` - Seeking interface
- `frontend/src/components/PostCard.svelte` - Integration point

---

## Provider Analysis

### YouTube

**Embed URL Format:**
```
https://www.youtube-nocookie.com/embed/{videoId}
```

**Source URL Patterns:**
- `youtube.com/watch?v={id}`
- `youtu.be/{id}`
- `youtube.com/embed/{id}`
- `youtube.com/shorts/{id}`
- `youtube.com/v/{id}`

**Current Implementation:**
The backend `YouTubeExtractor` parses the URL to extract the video ID. No HTTP request is made.

**Analysis:**
- ✅ No HTTP request needed
- ✅ Video ID is in the URL
- ❌ Unnecessarily routed through backend

**Recommendation:** Move to frontend. Pure URL parsing can be done client-side.

---

### Spotify

**Embed URL Format:**
```
https://open.spotify.com/embed/{type}/{id}
```

Where `{type}` is: `track`, `album`, `playlist`, `artist`, `show`, or `episode`.

**Source URL Pattern:**
```
https://open.spotify.com/{type}/{id}
```

**Embed Heights by Type:**
| Type | Height |
|------|--------|
| track | 152px |
| album | 380px |
| playlist | 380px |
| artist | 380px |
| show | 232px |
| episode | 232px |

**Current Implementation:**
The backend `SpotifyExtractor` parses the URL to extract type and ID. No HTTP request is made.

**Analysis:**
- ✅ No HTTP request needed
- ✅ Type and ID are in the URL
- ❌ Unnecessarily routed through backend

**Recommendation:** Move to frontend. Pure URL parsing can be done client-side.

---

### SoundCloud

**Embed URL Format:**
```
https://w.soundcloud.com/player/?url=https://api.soundcloud.com/tracks/{trackId}&...
```

**Source URL Pattern:**
```
https://soundcloud.com/{artist}/{track}
```

**The Problem:**
The numeric track ID is NOT in the public URL. The URL contains the artist name and track slug, but the embed player requires the numeric API ID.

**Resolution Methods:**

1. **oEmbed API** (current backend approach):
   ```
   GET https://soundcloud.com/oembed?format=json&url={soundcloudUrl}
   ```
   Returns JSON with `html` field containing the iframe embed code.

2. **Direct URL in Widget** (untested):
   Some reports suggest the widget may accept public URLs directly, but this is unreliable.

**CORS Status:** ✅ **Allowed** (tested February 2026)

The oEmbed endpoint returns proper CORS headers, allowing frontend JavaScript to call it directly.

**Current Implementation:**
Backend calls the oEmbed API and extracts the iframe URL from the response.

**Analysis:**
- ✅ oEmbed API has CORS enabled
- ✅ No authentication required
- ❌ Currently routed through backend unnecessarily

**Recommendation:** Move to frontend. Call oEmbed directly from client-side JavaScript.

---

### Bandcamp

**Embed URL Format:**
```
https://bandcamp.com/EmbeddedPlayer/album={albumId}/size=large/bgcol=ffffff/linkcol=333333/...
```
or
```
https://bandcamp.com/EmbeddedPlayer/track={trackId}/size=large/...
```

**Source URL Patterns:**
```
https://{artist}.bandcamp.com/album/{album-slug}
https://{artist}.bandcamp.com/track/{track-slug}
```

**The Problem:**
The numeric album/track ID is NOT in the public URL. You must fetch the HTML page and extract the ID.

**ID Extraction Methods:**

1. **JSON-LD Structured Data** (recommended):
   ```html
   <script type="application/ld+json">
   {
     "@type": "MusicAlbum",
     "albumRelease": [{
       "additionalProperty": [
         {"name": "item_id", "value": 3879992644}
       ]
     }]
   }
   </script>
   ```

   For albums: `albumRelease[0].additionalProperty` where `name === "item_id"`
   For tracks: `track.itemListElement[n].item.additionalProperty` where `name === "track_id"`

2. **bc-page-properties meta tag** (current implementation):
   ```html
   <meta name="bc-page-properties" content="{...JSON with IDs...}">
   ```

**CORS Status:** ❌ **Blocked**

Bandcamp does not send `Access-Control-Allow-Origin` headers. Additionally, Bandcamp employs WAF (Web Application Firewall) that blocks requests that don't look like real browsers.

**Current Implementation:**
Backend uses `azuretls-client` which mimics Chrome's TLS/HTTP2 fingerprint to bypass the WAF, then parses the `bc-page-properties` meta tag.

**Analysis:**
- ❌ CORS blocked - cannot fetch from frontend
- ❌ WAF blocks standard HTTP clients
- ❌ No public API or oEmbed endpoint
- ✅ azuretls workaround functional (for now)

**Recommendation:** Keep on backend. This is the only provider that truly requires server-side fetching. Consider switching to JSON-LD parsing for more stable extraction.

---

## CORS Testing Results

Tested from browser (about:blank) on February 5, 2026:

### SoundCloud oEmbed

```javascript
fetch('https://soundcloud.com/oembed?format=json&url=https://soundcloud.com/rick-astley-official/never-gonna-give-you-up-4')
```

**Result:** ✅ Success
```json
{
  "success": true,
  "status": 200,
  "corsAllowed": true,
  "data": {
    "title": "Never Gonna Give You Up by Rick Astley",
    "author_name": "Rick Astley",
    "html": "<iframe ... src=\"https://w.soundcloud.com/player/?visual=true&url=https%3A%2F%2Fapi.soundcloud.com%2Ftracks%2F1242868615&show_artwork=true\"></iframe>"
  }
}
```

### Summary Table

| Provider | Endpoint | CORS | Frontend Viable |
|----------|----------|------|-----------------|
| YouTube | N/A (URL parsing) | N/A | ✅ Yes |
| Spotify | N/A (URL parsing) | N/A | ✅ Yes |
| SoundCloud | oEmbed API | ✅ Allowed | ✅ Yes |
| Bandcamp | Page fetch required | ❌ Blocked | ❌ No |

---

## Recommendations

### 1. Move YouTube, Spotify, and SoundCloud to Frontend

These three providers can be handled entirely client-side:

| Provider | Method | HTTP Request |
|----------|--------|--------------|
| YouTube | URL regex parsing | None |
| Spotify | URL regex parsing | None |
| SoundCloud | oEmbed API call | 1 request (CORS allowed) |

**Benefits:**
- Instant embeds for YouTube/Spotify (no network delay)
- Reduced backend load
- No risk of server IP being blocked by these services
- Embeds work even if backend metadata fetching is disabled

### 2. Make Backend Metadata Fetching Asynchronous

For links that still need backend processing (Bandcamp, regular articles, recipes):

**Current (synchronous):**
```
User submits post → Wait for all metadata fetches → Return post
```

**Proposed (asynchronous):**
```
User submits post → Return post immediately → Fetch metadata in background → Push via WebSocket
```

**Benefits:**
- Post creation is never blocked by slow/failing fetches
- Better user experience
- Graceful degradation (if fetch fails, just show URL)

### 3. Improve Bandcamp Extraction

Switch from `bc-page-properties` meta tag parsing to JSON-LD structured data:

- JSON-LD is a schema.org standard, less likely to change
- More reliable data structure
- Contains additional metadata (title, artist, description, image)

### 4. Summary of Backend Requirements

| Content Type | Backend Required | Reason |
|--------------|------------------|--------|
| YouTube embeds | ❌ No | URL parsing only |
| Spotify embeds | ❌ No | URL parsing only |
| SoundCloud embeds | ❌ No | oEmbed has CORS |
| Bandcamp embeds | ✅ Yes | CORS blocked, WAF |
| Article previews | ✅ Yes | Need to fetch OpenGraph tags |
| Recipe metadata | ✅ Yes | Need to parse JSON-LD/microdata |
| Generic link previews | ✅ Yes | Need to fetch meta tags |

---

## Proposed Architecture

### New Flow

```
User posts link
    │
    ▼
Backend: Validate, store post with raw URL
    │
    ├─► Return post immediately (no blocking)
    │
    ▼
Frontend receives post
    │
    ├─► Is YouTube URL? → Parse URL → Render YouTubeEmbed immediately
    ├─► Is Spotify URL? → Parse URL → Render SpotifyEmbed immediately
    ├─► Is SoundCloud URL? → Call oEmbed → Render SoundCloudEmbed
    └─► Other URL? → Show placeholder, wait for backend

    ▼ (async, in parallel)

Backend worker: Queue metadata fetch job
    │
    ├─► Is Bandcamp? → Fetch with azuretls, parse JSON-LD
    ├─► Is article? → Fetch, parse OpenGraph
    ├─► Is recipe? → Fetch, parse JSON-LD/microdata
    └─► Store metadata in DB

    ▼

WebSocket: Push link_metadata_updated event
    │
    ▼

Frontend: Update post with metadata
    │
    ├─► Bandcamp? → Render BandcampEmbed
    └─► Article? → Render LinkPreviewCard
```

### WebSocket Event

```json
{
  "type": "link_metadata_updated",
  "data": {
    "post_id": "uuid",
    "link_id": "uuid",
    "metadata": {
      "title": "...",
      "description": "...",
      "image": "...",
      "embed": {
        "type": "iframe",
        "provider": "bandcamp",
        "embedUrl": "https://bandcamp.com/EmbeddedPlayer/album=...",
        "height": 470
      }
    }
  }
}
```

---

## Implementation Guide

### Phase 1: Frontend URL Parsers

Create `frontend/src/lib/embeds/urlParsers.ts`:

```typescript
// YouTube
export function parseYouTubeUrl(url: string): string | null {
  const patterns = [
    /youtube\.com\/watch\?v=([^&]+)/,
    /youtu\.be\/([^?]+)/,
    /youtube\.com\/embed\/([^?]+)/,
    /youtube\.com\/shorts\/([^?]+)/,
    /youtube\.com\/v\/([^?]+)/,
  ];

  for (const pattern of patterns) {
    const match = url.match(pattern);
    if (match) {
      return `https://www.youtube-nocookie.com/embed/${match[1]}`;
    }
  }
  return null;
}

// Spotify
interface SpotifyEmbed {
  embedUrl: string;
  height: number;
}

const SPOTIFY_HEIGHTS: Record<string, number> = {
  track: 152,
  album: 380,
  playlist: 380,
  artist: 380,
  show: 232,
  episode: 232,
};

export function parseSpotifyUrl(url: string): SpotifyEmbed | null {
  const match = url.match(/open\.spotify\.com\/(track|album|playlist|artist|show|episode)\/([^?]+)/);
  if (!match) return null;

  const [, type, id] = match;
  return {
    embedUrl: `https://open.spotify.com/embed/${type}/${id}`,
    height: SPOTIFY_HEIGHTS[type] ?? 380,
  };
}

// SoundCloud
export async function fetchSoundCloudEmbed(url: string): Promise<{ embedUrl: string; height: number } | null> {
  try {
    const oembedUrl = `https://soundcloud.com/oembed?format=json&url=${encodeURIComponent(url)}`;
    const response = await fetch(oembedUrl);
    if (!response.ok) return null;

    const data = await response.json();

    // Extract iframe src from HTML
    const srcMatch = data.html?.match(/src="([^"]+)"/);
    if (!srcMatch) return null;

    return {
      embedUrl: srcMatch[1],
      height: data.height ?? 166,
    };
  } catch {
    return null;
  }
}

// URL detection helpers
export function isYouTubeUrl(url: string): boolean {
  return /(?:youtube\.com|youtu\.be)/.test(url);
}

export function isSpotifyUrl(url: string): boolean {
  return /open\.spotify\.com/.test(url);
}

export function isSoundCloudUrl(url: string): boolean {
  return /soundcloud\.com/.test(url);
}

export function isBandcampUrl(url: string): boolean {
  return /\.bandcamp\.com/.test(url);
}
```

### Phase 2: Update PostCard Component

Modify `frontend/src/components/PostCard.svelte` to check frontend parsers first:

```svelte
<script>
  import { parseYouTubeUrl, parseSpotifyUrl, fetchSoundCloudEmbed, isYouTubeUrl, isSpotifyUrl, isSoundCloudUrl } from '../lib/embeds/urlParsers';

  // For YouTube/Spotify: parse immediately, no async needed
  $: youtubeEmbedUrl = link?.url && isYouTubeUrl(link.url)
    ? parseYouTubeUrl(link.url)
    : metadata?.embed?.embedUrl;

  $: spotifyEmbed = link?.url && isSpotifyUrl(link.url)
    ? parseSpotifyUrl(link.url)
    : metadata?.embed;

  // For SoundCloud: fetch oEmbed on mount if needed
  let soundCloudEmbed: { embedUrl: string; height: number } | null = null;

  onMount(async () => {
    if (link?.url && isSoundCloudUrl(link.url) && !metadata?.embed?.embedUrl) {
      soundCloudEmbed = await fetchSoundCloudEmbed(link.url);
    }
  });
</script>
```

### Phase 3: Async Backend Metadata Fetching

1. **Modify PostService.CreatePost():**
   - Skip synchronous metadata fetching
   - Queue a background job for metadata fetch

2. **Create metadata worker:**
   - Process queued fetch jobs
   - Store metadata in database
   - Publish WebSocket event on completion

3. **Add WebSocket event handler:**
   - Frontend listens for `link_metadata_updated`
   - Updates post store reactively

### Phase 4: Bandcamp JSON-LD Extraction

Update `backend/internal/services/links/bandcamp.go` to parse JSON-LD instead of `bc-page-properties`:

```go
func extractBandcampIDFromJSONLD(body []byte) (itemType string, itemID int64, err error) {
    // Find <script type="application/ld+json"> content
    // Parse JSON
    // Navigate to albumRelease[0].additionalProperty
    // Find object where name == "item_id"
    // Return type ("album" or "track") and ID
}
```

---

## Content Security Policy

The current CSP in `backend/internal/middleware/csp.go` already allows the required iframe sources:

```
frame-src 'self'
  https://www.youtube-nocookie.com
  https://open.spotify.com
  https://w.soundcloud.com
  https://bandcamp.com;
```

No CSP changes are needed for the proposed architecture.

---

## Migration Path

1. **Phase 1** (Quick Win): Add frontend URL parsers for YouTube/Spotify
   - No backend changes needed
   - Immediate improvement for most common embeds

2. **Phase 2**: Add frontend SoundCloud oEmbed
   - No backend changes needed
   - Removes another source of backend load

3. **Phase 3**: Make backend metadata fetching async
   - Requires backend worker + WebSocket event
   - Improves UX for all link types

4. **Phase 4**: Improve Bandcamp extraction
   - Switch to JSON-LD parsing
   - More stable long-term

Each phase can be deployed independently and provides incremental value.

---

## Code to Remove

After implementing the proposed architecture, the following backend code becomes obsolete and can be removed:

### Backend Files to Delete

| File | Reason |
|------|--------|
| `backend/internal/services/links/youtube.go` | YouTube extraction moved to frontend URL parsing |
| `backend/internal/services/links/spotify.go` | Spotify extraction moved to frontend URL parsing |
| `backend/internal/services/links/soundcloud.go` | SoundCloud extraction moved to frontend oEmbed call |

### Backend Files to Simplify

| File | Changes |
|------|---------|
| `backend/internal/services/links/embed_registry.go` | Remove `YouTubeExtractor`, `SoundCloudExtractor` from `embedExtractors` slice |
| `backend/internal/services/links/embed_extractors.go` | Remove `SpotifyExtractor` from `defaultEmbedExtractors` slice |
| `backend/internal/services/link_metadata.go` | Remove synchronous `fetchLinkMetadata()` call from post creation; replace with async job queue |
| `backend/internal/services/post_service.go` | Remove blocking metadata fetch; add job enqueue for background processing |

### Test Files to Update/Remove

| File | Action |
|------|--------|
| `backend/internal/services/links/youtube_test.go` | Delete (if exists) |
| `backend/internal/services/links/spotify_test.go` | Delete (if exists) |
| `backend/internal/services/links/soundcloud_test.go` | Delete (if exists) |
| `backend/internal/services/links/embed_registry_test.go` | Update to remove tests for deleted extractors |
| Integration tests for post creation | Update to not expect synchronous metadata |

### What Stays on Backend

| File | Reason |
|------|--------|
| `backend/internal/services/links/bandcamp.go` | Bandcamp requires server-side fetch (CORS blocked) |
| `backend/internal/services/links/bandcamp_fetch.go` | azuretls client for WAF bypass |
| `backend/internal/services/links/metadata.go` | Still needed for OpenGraph, Twitter Cards, recipe parsing |
| `backend/internal/services/links/embed.go` | Interfaces still used by Bandcamp |
| `backend/internal/services/links/embed_validation.go` | Still validates Bandcamp embed URLs |
| `backend/internal/middleware/csp.go` | CSP still needed (no changes required) |

### Frontend Files to Add

| File | Purpose |
|------|---------|
| `frontend/src/lib/embeds/urlParsers.ts` | YouTube/Spotify URL parsing, SoundCloud oEmbed fetch |

### Frontend Files to Modify

| File | Changes |
|------|---------|
| `frontend/src/components/PostCard.svelte` | Use frontend parsers before falling back to backend metadata |
| `frontend/src/stores/postStore.ts` | Add handler for `link_metadata_updated` WebSocket event |

### Summary: Lines of Code Impact

| Category | Approximate Change |
|----------|-------------------|
| Backend code removed | ~400-500 lines (3 extractors + tests) |
| Backend code simplified | ~100-200 lines (registry, post service) |
| Backend code added | ~150-200 lines (async worker, WebSocket event) |
| Frontend code added | ~100-150 lines (URL parsers, WebSocket handler) |
| **Net change** | Roughly neutral, but significantly simpler architecture |

### Dependency Changes

The following Go dependencies may become unused after removing extractors:

- If `soundcloud.go` is the only file making oEmbed HTTP calls with certain patterns, check if any HTTP client utilities can be simplified
- No external dependencies should need removal (azuretls is still used for Bandcamp)

### Database Changes

**No schema changes required.** The `links` table continues to store metadata as JSONB. The only difference is:

- For YouTube/Spotify/SoundCloud: `metadata.embed` may be `null` in the database (frontend generates embed URL)
- For Bandcamp/articles/recipes: `metadata` populated asynchronously instead of synchronously

Consider adding an index if you need to query links by whether metadata has been fetched:

```sql
-- Optional: index for finding links pending metadata fetch
CREATE INDEX idx_links_metadata_null ON links ((metadata IS NULL)) WHERE metadata IS NULL;
```

---

## Appendix: Embed URL Reference

### YouTube
```
https://www.youtube-nocookie.com/embed/{videoId}
```

### Spotify
```
https://open.spotify.com/embed/{type}/{id}
```
Types: `track`, `album`, `playlist`, `artist`, `show`, `episode`

### SoundCloud
```
https://w.soundcloud.com/player/?url=https://api.soundcloud.com/tracks/{trackId}&visual=true&show_artwork=true
```

### Bandcamp
```
https://bandcamp.com/EmbeddedPlayer/album={albumId}/size=large/bgcol=ffffff/linkcol=333333/tracklist=false/transparent=true/
```
or
```
https://bandcamp.com/EmbeddedPlayer/track={trackId}/size=large/bgcol=ffffff/linkcol=333333/transparent=true/
```

Parameters:
- `size`: small, medium, large, venti
- `bgcol`: background color (hex without #)
- `linkcol`: link color (hex without #)
- `tracklist`: true/false
- `artwork`: small (for minimal display)
- `transparent`: true/false
- `minimal`: true/false
