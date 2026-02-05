# Phase 1, Step 1: URL Detection Helpers and YouTube Parser

## Overview

Create the foundational URL parser module with detection helpers for all supported providers and implement the YouTube URL parser.

## Detailed Description

Create a new file `frontend/src/lib/embeds/urlParsers.ts` that will contain:

1. **URL Detection Functions** - Simple boolean functions to identify link providers:
   - `isYouTubeUrl(url: string): boolean`
   - `isSpotifyUrl(url: string): boolean`
   - `isSoundCloudUrl(url: string): boolean`
   - `isBandcampUrl(url: string): boolean`

2. **YouTube Parser** - Extract video ID and return embed URL:
   - `parseYouTubeUrl(url: string): string | null`

### YouTube URL Patterns to Support

| Pattern | Example |
|---------|---------|
| Standard watch | `https://www.youtube.com/watch?v=dQw4w9WgXcQ` |
| Short URL | `https://youtu.be/dQw4w9WgXcQ` |
| Embed URL | `https://www.youtube.com/embed/dQw4w9WgXcQ` |
| Shorts | `https://www.youtube.com/shorts/dQw4w9WgXcQ` |
| Legacy | `https://www.youtube.com/v/dQw4w9WgXcQ` |
| With timestamp | `https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=120` |
| With playlist | `https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLxyz` |

### Output Format

The `parseYouTubeUrl` function should return the privacy-enhanced embed URL:
```
https://www.youtube-nocookie.com/embed/{videoId}
```

Return `null` if the URL is not a valid YouTube URL or video ID cannot be extracted.

## Files to Create

| File | Description |
|------|-------------|
| `frontend/src/lib/embeds/urlParsers.ts` | URL detection and parsing functions |
| `frontend/src/lib/embeds/urlParsers.test.ts` | Unit tests |

## Expected Outcomes

1. All URL detection functions correctly identify their respective providers
2. YouTube parser extracts video IDs from all supported URL formats
3. YouTube parser returns privacy-enhanced embed URLs
4. Invalid URLs return `null` without throwing errors
5. All tests pass

## Test Cases

### URL Detection Tests

```typescript
describe('isYouTubeUrl', () => {
  it('returns true for youtube.com URLs', () => {
    expect(isYouTubeUrl('https://www.youtube.com/watch?v=abc123')).toBe(true);
    expect(isYouTubeUrl('https://youtube.com/watch?v=abc123')).toBe(true);
  });

  it('returns true for youtu.be URLs', () => {
    expect(isYouTubeUrl('https://youtu.be/abc123')).toBe(true);
  });

  it('returns false for non-YouTube URLs', () => {
    expect(isYouTubeUrl('https://vimeo.com/123')).toBe(false);
    expect(isYouTubeUrl('https://spotify.com/track/abc')).toBe(false);
  });
});

describe('isSpotifyUrl', () => {
  it('returns true for open.spotify.com URLs', () => {
    expect(isSpotifyUrl('https://open.spotify.com/track/abc')).toBe(true);
  });

  it('returns false for non-Spotify URLs', () => {
    expect(isSpotifyUrl('https://youtube.com/watch?v=abc')).toBe(false);
  });
});

describe('isSoundCloudUrl', () => {
  it('returns true for soundcloud.com URLs', () => {
    expect(isSoundCloudUrl('https://soundcloud.com/artist/track')).toBe(true);
  });

  it('returns false for non-SoundCloud URLs', () => {
    expect(isSoundCloudUrl('https://youtube.com/watch?v=abc')).toBe(false);
  });
});

describe('isBandcampUrl', () => {
  it('returns true for bandcamp.com URLs', () => {
    expect(isBandcampUrl('https://artist.bandcamp.com/album/title')).toBe(true);
  });

  it('returns false for non-Bandcamp URLs', () => {
    expect(isBandcampUrl('https://youtube.com/watch?v=abc')).toBe(false);
  });
});
```

### YouTube Parser Tests

```typescript
describe('parseYouTubeUrl', () => {
  it('parses standard watch URLs', () => {
    expect(parseYouTubeUrl('https://www.youtube.com/watch?v=dQw4w9WgXcQ'))
      .toBe('https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ');
  });

  it('parses short URLs', () => {
    expect(parseYouTubeUrl('https://youtu.be/dQw4w9WgXcQ'))
      .toBe('https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ');
  });

  it('parses embed URLs', () => {
    expect(parseYouTubeUrl('https://www.youtube.com/embed/dQw4w9WgXcQ'))
      .toBe('https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ');
  });

  it('parses shorts URLs', () => {
    expect(parseYouTubeUrl('https://www.youtube.com/shorts/dQw4w9WgXcQ'))
      .toBe('https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ');
  });

  it('parses legacy v/ URLs', () => {
    expect(parseYouTubeUrl('https://www.youtube.com/v/dQw4w9WgXcQ'))
      .toBe('https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ');
  });

  it('handles URLs with extra parameters', () => {
    expect(parseYouTubeUrl('https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=120&list=PLxyz'))
      .toBe('https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ');
  });

  it('handles URLs without www', () => {
    expect(parseYouTubeUrl('https://youtube.com/watch?v=dQw4w9WgXcQ'))
      .toBe('https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ');
  });

  it('returns null for invalid YouTube URLs', () => {
    expect(parseYouTubeUrl('https://youtube.com/channel/abc')).toBeNull();
    expect(parseYouTubeUrl('https://youtube.com/')).toBeNull();
  });

  it('returns null for non-YouTube URLs', () => {
    expect(parseYouTubeUrl('https://vimeo.com/123456')).toBeNull();
    expect(parseYouTubeUrl('not a url')).toBeNull();
  });
});
```

## Verification

```bash
# Run the tests
cd frontend && npm run test -- urlParsers

# Verify TypeScript compiles
cd frontend && npm run check
```

## Notes

- Use URL constructor for parsing when possible to handle edge cases
- Video IDs are typically 11 characters but don't hardcode this constraint
- The detection functions should be fast (no network calls)
- Export all functions for use in PostCard.svelte (next step)
