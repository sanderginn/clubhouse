# Phase 1, Step 3: Integrate YouTube/Spotify Parsers into PostCard

## Overview

Update PostCard.svelte to use the frontend URL parsers for instant YouTube and Spotify embed rendering, with fallback to backend metadata.

## Detailed Description

Modify `frontend/src/components/PostCard.svelte` to:

1. Import the URL parsers
2. Update reactive statements to check frontend parsers first
3. Fall back to backend metadata if frontend parsing fails

### Current Behavior (lines ~559-587)

The component currently derives embed URLs from `metadata.embed`:
```svelte
$: youtubeEmbedUrl = metadata?.embed?.provider === 'youtube' ? metadata.embed.embedUrl : undefined;
$: spotifyEmbed = metadata?.embed && isSpotifyEmbedUrl(metadata.embed.embedUrl) ? metadata.embed : undefined;
```

### New Behavior

Check the raw URL first using frontend parsers, then fall back to backend metadata:

```svelte
<script lang="ts">
  import { isYouTubeUrl, isSpotifyUrl, parseYouTubeUrl, parseSpotifyUrl } from '$lib/embeds/urlParsers';

  // YouTube: try frontend parsing first, fall back to backend metadata
  $: youtubeEmbedUrl = (() => {
    const url = primaryLink?.url;
    if (url && isYouTubeUrl(url)) {
      return parseYouTubeUrl(url);
    }
    return metadata?.embed?.provider === 'youtube' ? metadata.embed.embedUrl : undefined;
  })();

  // Spotify: try frontend parsing first, fall back to backend metadata
  $: spotifyEmbed = (() => {
    const url = primaryLink?.url;
    if (url && isSpotifyUrl(url)) {
      return parseSpotifyUrl(url);
    }
    // Fall back to backend metadata
    if (metadata?.embed && isSpotifyEmbedUrl(metadata.embed.embedUrl)) {
      return metadata.embed;
    }
    return undefined;
  })();
</script>
```

### Important Considerations

1. **primaryLink** - Need to verify how the primary link is determined in PostCard. Look for existing `primaryLink` reactive statement.

2. **Backward Compatibility** - Backend metadata should still work as fallback for:
   - Existing posts created before this change
   - Cases where frontend parsing unexpectedly fails

3. **Type Safety** - Ensure the return types match what the template expects:
   - `youtubeEmbedUrl`: `string | undefined`
   - `spotifyEmbed`: `{ embedUrl: string; height: number } | undefined`

## Files to Modify

| File | Changes |
|------|---------|
| `frontend/src/components/PostCard.svelte` | Import parsers, update reactive statements |

## Expected Outcomes

1. YouTube embeds render immediately without waiting for backend metadata
2. Spotify embeds render immediately without waiting for backend metadata
3. Existing posts with backend metadata continue to work
4. Posts where frontend parsing fails fall back to backend metadata
5. No visual changes to the rendered embeds
6. TypeScript compiles without errors

## Test Cases

### Manual Testing Checklist

Since PostCard is a Svelte component that requires the full app context, manual testing is recommended:

1. **YouTube Embed - New Post**
   - Create a new post with a YouTube link
   - Verify embed appears immediately (before backend metadata arrives)
   - Check browser Network tab - embed should render before metadata API response

2. **Spotify Embed - New Post**
   - Create posts with different Spotify link types:
     - Track: `https://open.spotify.com/track/...`
     - Album: `https://open.spotify.com/album/...`
     - Playlist: `https://open.spotify.com/playlist/...`
   - Verify embeds appear immediately
   - Verify correct heights for each type

3. **Existing Posts (Backward Compatibility)**
   - Load a page with existing posts that have YouTube/Spotify links
   - Verify embeds still render correctly from backend metadata

4. **Mixed Scenarios**
   - Post with YouTube link + other non-embed link
   - Post with Spotify link + text content
   - Post with link that has both frontend-parseable URL and backend metadata

5. **Error Cases**
   - Post a malformed YouTube URL (e.g., `youtube.com/channel/...`)
   - Verify it falls back to backend metadata or shows link card

### Component Test (Optional)

If component testing is set up, add to `PostCard.test.ts`:

```typescript
import { render } from '@testing-library/svelte';
import PostCard from './PostCard.svelte';

describe('PostCard embed rendering', () => {
  it('renders YouTube embed from URL without metadata', () => {
    const post = {
      id: '123',
      links: [{
        id: 'link-1',
        url: 'https://www.youtube.com/watch?v=dQw4w9WgXcQ',
        metadata: null // No backend metadata yet
      }]
    };

    const { container } = render(PostCard, { props: { post } });

    const iframe = container.querySelector('iframe[src*="youtube-nocookie.com"]');
    expect(iframe).toBeTruthy();
    expect(iframe.src).toContain('dQw4w9WgXcQ');
  });

  it('renders Spotify embed from URL without metadata', () => {
    const post = {
      id: '123',
      links: [{
        id: 'link-1',
        url: 'https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh',
        metadata: null
      }]
    };

    const { container } = render(PostCard, { props: { post } });

    const iframe = container.querySelector('iframe[src*="open.spotify.com/embed"]');
    expect(iframe).toBeTruthy();
  });
});
```

## Verification

```bash
# Verify TypeScript compiles
cd frontend && npm run check

# Run existing tests to ensure no regressions
cd frontend && npm run test

# Manual testing with dev server
task dev:up
# Navigate to the app and test posting YouTube/Spotify links
```

## Notes

- Check the existing `isSpotifyEmbedUrl` helper function in PostCard - it may need to be kept for the fallback logic
- The `primaryLink` variable should already exist in PostCard; verify its type and how it's derived
- Consider adding a small visual indicator (like a loading state) while waiting for backend metadata for non-embeddable links
- Don't remove the existing backend metadata handling - it's needed for fallback and for link preview cards (title, description, image)
