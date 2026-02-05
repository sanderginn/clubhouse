# Phase 2, Step 1: SoundCloud oEmbed Fetcher

## Overview

Add an async function to fetch SoundCloud embed information via their public oEmbed API.

## Detailed Description

Extend `frontend/src/lib/embeds/urlParsers.ts` with:

- `fetchSoundCloudEmbed(url: string): Promise<{ embedUrl: string; height: number } | null>`

### SoundCloud oEmbed API

SoundCloud provides a public oEmbed endpoint that doesn't require authentication:

```
GET https://soundcloud.com/oembed?format=json&url={encodedUrl}
```

### Response Format

```json
{
  "version": 1.0,
  "type": "rich",
  "provider_name": "SoundCloud",
  "provider_url": "https://soundcloud.com",
  "height": 166,
  "width": "100%",
  "title": "Track Title",
  "description": "Track description...",
  "thumbnail_url": "https://...",
  "html": "<iframe width=\"100%\" height=\"166\" scrolling=\"no\" frameborder=\"no\" src=\"https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/tracks/123456&...\"></iframe>",
  "author_name": "Artist Name",
  "author_url": "https://soundcloud.com/artist"
}
```

### Extraction Logic

1. Call the oEmbed API with the SoundCloud URL
2. Parse the `html` field to extract the iframe `src` attribute
3. Return the embed URL and height from the response

### Error Handling

- Network errors: return `null`
- Non-200 responses: return `null`
- Invalid/missing HTML in response: return `null`
- Timeout: 5 second timeout, return `null` on timeout

## Files to Modify

| File | Changes |
|------|---------|
| `frontend/src/lib/embeds/urlParsers.ts` | Add `fetchSoundCloudEmbed` async function |
| `frontend/src/lib/embeds/urlParsers.test.ts` | Add tests with mocked fetch |

## Expected Outcomes

1. Function successfully fetches embed info for valid SoundCloud URLs
2. Correctly extracts iframe src from HTML response
3. Returns appropriate height from oEmbed response
4. Gracefully handles errors (returns null, doesn't throw)
5. Respects timeout
6. All tests pass

## Test Cases

```typescript
describe('fetchSoundCloudEmbed', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches and parses oEmbed response for track URL', async () => {
    const mockResponse = {
      height: 166,
      html: '<iframe width="100%" height="166" scrolling="no" frameborder="no" src="https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/tracks/123456&color=%23ff5500"></iframe>'
    };

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse)
    });

    const result = await fetchSoundCloudEmbed('https://soundcloud.com/artist/track-name');

    expect(fetch).toHaveBeenCalledWith(
      expect.stringContaining('soundcloud.com/oembed'),
      expect.any(Object)
    );
    expect(result).toEqual({
      embedUrl: 'https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/tracks/123456&color=%23ff5500',
      height: 166
    });
  });

  it('fetches and parses oEmbed response for playlist URL', async () => {
    const mockResponse = {
      height: 450,
      html: '<iframe width="100%" height="450" scrolling="no" frameborder="no" src="https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/playlists/789"></iframe>'
    };

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse)
    });

    const result = await fetchSoundCloudEmbed('https://soundcloud.com/artist/sets/playlist-name');

    expect(result).toEqual({
      embedUrl: 'https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/playlists/789',
      height: 450
    });
  });

  it('returns null for non-200 response', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 404
    });

    const result = await fetchSoundCloudEmbed('https://soundcloud.com/nonexistent/track');

    expect(result).toBeNull();
  });

  it('returns null when HTML is missing from response', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ height: 166 }) // No html field
    });

    const result = await fetchSoundCloudEmbed('https://soundcloud.com/artist/track');

    expect(result).toBeNull();
  });

  it('returns null when iframe src cannot be extracted', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        height: 166,
        html: '<div>No iframe here</div>'
      })
    });

    const result = await fetchSoundCloudEmbed('https://soundcloud.com/artist/track');

    expect(result).toBeNull();
  });

  it('returns null on network error', async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error('Network error'));

    const result = await fetchSoundCloudEmbed('https://soundcloud.com/artist/track');

    expect(result).toBeNull();
  });

  it('returns null on timeout', async () => {
    global.fetch = vi.fn().mockImplementation(() =>
      new Promise((_, reject) =>
        setTimeout(() => reject(new Error('Timeout')), 10000)
      )
    );

    // Use fake timers if needed, or test that AbortController is used
    const result = await fetchSoundCloudEmbed('https://soundcloud.com/artist/track');

    expect(result).toBeNull();
  }, 10000);

  it('encodes URL parameter correctly', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        height: 166,
        html: '<iframe src="https://w.soundcloud.com/player/?url=test"></iframe>'
      })
    });

    await fetchSoundCloudEmbed('https://soundcloud.com/artist/track with spaces');

    expect(fetch).toHaveBeenCalledWith(
      expect.stringContaining(encodeURIComponent('https://soundcloud.com/artist/track with spaces')),
      expect.any(Object)
    );
  });
});
```

## Implementation Notes

```typescript
export async function fetchSoundCloudEmbed(url: string): Promise<{ embedUrl: string; height: number } | null> {
  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 5000);

    const oembedUrl = `https://soundcloud.com/oembed?format=json&url=${encodeURIComponent(url)}`;
    const response = await fetch(oembedUrl, { signal: controller.signal });

    clearTimeout(timeoutId);

    if (!response.ok) {
      return null;
    }

    const data = await response.json();

    if (!data.html) {
      return null;
    }

    // Extract src from iframe HTML
    const srcMatch = data.html.match(/src="([^"]+)"/);
    if (!srcMatch || !srcMatch[1]) {
      return null;
    }

    return {
      embedUrl: srcMatch[1],
      height: data.height || 166 // Default height if not provided
    };
  } catch {
    return null;
  }
}
```

## Verification

```bash
# Run the tests
cd frontend && npm run test -- urlParsers

# Verify TypeScript compiles
cd frontend && npm run check
```

## Notes

- SoundCloud's oEmbed API is public and doesn't require API keys
- The height varies: tracks are typically 166px, playlists can be 450px+
- Consider caching responses in memory for the session to avoid redundant API calls
- The `html` field contains a full iframe tag; we only need the `src` attribute
- CORS: SoundCloud's oEmbed endpoint supports CORS, so direct browser fetch works
