# Phase 1, Step 2: Spotify URL Parser

## Overview

Add Spotify URL parsing to the urlParsers module, returning embed URLs with appropriate heights based on content type.

## Detailed Description

Extend `frontend/src/lib/embeds/urlParsers.ts` with:

- `parseSpotifyUrl(url: string): { embedUrl: string; height: number } | null`

### Spotify URL Pattern

All Spotify URLs follow the pattern:
```
https://open.spotify.com/{type}/{id}
```

Where `{type}` is one of:
- `track` - Single song
- `album` - Album
- `playlist` - Playlist
- `artist` - Artist page
- `show` - Podcast show
- `episode` - Podcast episode

### Embed Heights by Type

| Type | Height (px) | Reason |
|------|-------------|--------|
| track | 152 | Compact single-track player |
| album | 380 | Shows track list |
| playlist | 380 | Shows track list |
| artist | 380 | Shows top tracks |
| show | 232 | Podcast player |
| episode | 232 | Podcast player |

### Output Format

```typescript
{
  embedUrl: "https://open.spotify.com/embed/{type}/{id}",
  height: number
}
```

Return `null` if the URL is not a valid Spotify URL or cannot be parsed.

## Files to Modify

| File | Changes |
|------|---------|
| `frontend/src/lib/embeds/urlParsers.ts` | Add `parseSpotifyUrl` function |
| `frontend/src/lib/embeds/urlParsers.test.ts` | Add Spotify parser tests |

## Expected Outcomes

1. Spotify parser correctly extracts type and ID from URLs
2. Returns appropriate embed URL format
3. Returns correct height for each content type
4. Handles URLs with query parameters (e.g., `?si=...`)
5. Returns `null` for invalid Spotify URLs
6. All tests pass

## Test Cases

```typescript
describe('parseSpotifyUrl', () => {
  describe('track URLs', () => {
    it('parses track URLs', () => {
      const result = parseSpotifyUrl('https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh');
      expect(result).toEqual({
        embedUrl: 'https://open.spotify.com/embed/track/4iV5W9uYEdYUVa79Axb7Rh',
        height: 152
      });
    });

    it('handles track URLs with query params', () => {
      const result = parseSpotifyUrl('https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh?si=abc123');
      expect(result).toEqual({
        embedUrl: 'https://open.spotify.com/embed/track/4iV5W9uYEdYUVa79Axb7Rh',
        height: 152
      });
    });
  });

  describe('album URLs', () => {
    it('parses album URLs', () => {
      const result = parseSpotifyUrl('https://open.spotify.com/album/1DFixLWuPkv3KT3TnV35m3');
      expect(result).toEqual({
        embedUrl: 'https://open.spotify.com/embed/album/1DFixLWuPkv3KT3TnV35m3',
        height: 380
      });
    });
  });

  describe('playlist URLs', () => {
    it('parses playlist URLs', () => {
      const result = parseSpotifyUrl('https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M');
      expect(result).toEqual({
        embedUrl: 'https://open.spotify.com/embed/playlist/37i9dQZF1DXcBWIGoYBM5M',
        height: 380
      });
    });
  });

  describe('artist URLs', () => {
    it('parses artist URLs', () => {
      const result = parseSpotifyUrl('https://open.spotify.com/artist/0OdUWJ0sBjDrqHygGUXeCF');
      expect(result).toEqual({
        embedUrl: 'https://open.spotify.com/embed/artist/0OdUWJ0sBjDrqHygGUXeCF',
        height: 380
      });
    });
  });

  describe('podcast URLs', () => {
    it('parses show URLs', () => {
      const result = parseSpotifyUrl('https://open.spotify.com/show/4rOoJ6Egrf8K2IrywzwOMk');
      expect(result).toEqual({
        embedUrl: 'https://open.spotify.com/embed/show/4rOoJ6Egrf8K2IrywzwOMk',
        height: 232
      });
    });

    it('parses episode URLs', () => {
      const result = parseSpotifyUrl('https://open.spotify.com/episode/512ojhOuo1ktJprKbVcKyQ');
      expect(result).toEqual({
        embedUrl: 'https://open.spotify.com/embed/episode/512ojhOuo1ktJprKbVcKyQ',
        height: 232
      });
    });
  });

  describe('invalid URLs', () => {
    it('returns null for non-Spotify URLs', () => {
      expect(parseSpotifyUrl('https://youtube.com/watch?v=abc')).toBeNull();
    });

    it('returns null for Spotify URLs without type/id', () => {
      expect(parseSpotifyUrl('https://open.spotify.com/')).toBeNull();
      expect(parseSpotifyUrl('https://open.spotify.com/search')).toBeNull();
    });

    it('returns null for unsupported Spotify URL types', () => {
      expect(parseSpotifyUrl('https://open.spotify.com/user/abc123')).toBeNull();
      expect(parseSpotifyUrl('https://open.spotify.com/genre/rock')).toBeNull();
    });

    it('returns null for malformed URLs', () => {
      expect(parseSpotifyUrl('not a url')).toBeNull();
    });
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

- Spotify IDs are base62 encoded strings (22 characters for most types)
- The `?si=` parameter is a share tracking token and should be stripped
- Only support the documented embed types; other URL patterns (user, genre, search) should return null
- Consider using a type union for the supported types:
  ```typescript
  type SpotifyContentType = 'track' | 'album' | 'playlist' | 'artist' | 'show' | 'episode';
  ```
