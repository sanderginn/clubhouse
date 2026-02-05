# Phase 2, Step 2: Integrate SoundCloud oEmbed into PostCard

## Overview

Update PostCard.svelte to fetch SoundCloud embeds via frontend oEmbed call, with fallback to backend metadata.

## Detailed Description

Modify `frontend/src/components/PostCard.svelte` to:

1. Import the SoundCloud functions
2. Add component state for frontend-fetched SoundCloud embed
3. Add `onMount` logic to fetch SoundCloud embed when needed
4. Update reactive statement to prefer frontend-fetched embed

### Key Difference from YouTube/Spotify

Unlike YouTube and Spotify (which are synchronous URL parsing), SoundCloud requires an async oEmbed API call. This means:

1. **Can't use reactive statement alone** - Need `onMount` for the async fetch
2. **Loading state** - Embed won't be immediately available
3. **Conditional fetching** - Only fetch if no backend metadata exists yet

### Implementation Approach

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { isSoundCloudUrl, fetchSoundCloudEmbed } from '$lib/embeds/urlParsers';

  // State for frontend-fetched SoundCloud embed
  let soundCloudEmbedFromFrontend: { embedUrl: string; height: number } | null = null;

  onMount(async () => {
    const url = primaryLink?.url;
    // Only fetch if it's a SoundCloud URL and we don't have backend metadata yet
    if (url && isSoundCloudUrl(url) && !metadata?.embed?.embedUrl) {
      soundCloudEmbedFromFrontend = await fetchSoundCloudEmbed(url);
    }
  });

  // Prefer frontend-fetched, fall back to backend metadata
  $: soundCloudEmbed = soundCloudEmbedFromFrontend ??
    (metadata?.embed?.provider === 'soundcloud' && metadata.embed.embedUrl
      ? metadata.embed
      : undefined);
</script>
```

### Handling Dynamic Updates

If the post's links can change (e.g., editing), the `onMount` approach won't re-fetch. Consider using a reactive statement with an async block or a store if dynamic updates are needed. For MVP, `onMount` is sufficient since posts are typically not edited.

## Files to Modify

| File | Changes |
|------|---------|
| `frontend/src/components/PostCard.svelte` | Import functions, add state, add onMount, update reactive statement |

## Expected Outcomes

1. SoundCloud embeds render after frontend oEmbed fetch completes
2. If backend metadata exists, it's used immediately (no frontend fetch)
3. If both exist, frontend-fetched takes precedence (fresher)
4. Failed fetches gracefully fall back to backend metadata or link card
5. No duplicate fetches (only fetch when metadata is missing)
6. TypeScript compiles without errors

## Test Cases

### Manual Testing Checklist

1. **SoundCloud Track - New Post**
   - Create a new post with a SoundCloud track URL
   - Verify embed appears after a brief moment (oEmbed fetch)
   - Check Network tab - should see request to `soundcloud.com/oembed`

2. **SoundCloud Playlist - New Post**
   - Create post with SoundCloud playlist URL (`/sets/...`)
   - Verify embed appears with correct (taller) height

3. **Existing Posts (Backward Compatibility)**
   - Load page with existing posts that have SoundCloud links
   - Verify embeds render from backend metadata
   - Check Network tab - should NOT see oEmbed requests (metadata exists)

4. **Failed oEmbed Fetch**
   - Post a private or deleted SoundCloud URL
   - Verify graceful fallback to link card
   - No errors in console

5. **Race Condition: Backend Metadata Arrives First**
   - Post a SoundCloud link
   - If backend metadata arrives before oEmbed completes, verify no visual glitch
   - The embed should remain stable once rendered

### Component Test (Optional)

```typescript
import { render, waitFor } from '@testing-library/svelte';
import PostCard from './PostCard.svelte';

describe('PostCard SoundCloud embed', () => {
  it('fetches SoundCloud embed when no metadata exists', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        height: 166,
        html: '<iframe src="https://w.soundcloud.com/player/?url=test"></iframe>'
      })
    });
    global.fetch = mockFetch;

    const post = {
      id: '123',
      links: [{
        id: 'link-1',
        url: 'https://soundcloud.com/artist/track',
        metadata: null
      }]
    };

    const { container } = render(PostCard, { props: { post } });

    await waitFor(() => {
      const iframe = container.querySelector('iframe[src*="soundcloud.com/player"]');
      expect(iframe).toBeTruthy();
    });

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining('soundcloud.com/oembed'),
      expect.any(Object)
    );
  });

  it('does not fetch when backend metadata exists', async () => {
    const mockFetch = vi.fn();
    global.fetch = mockFetch;

    const post = {
      id: '123',
      links: [{
        id: 'link-1',
        url: 'https://soundcloud.com/artist/track',
        metadata: {
          embed: {
            provider: 'soundcloud',
            embedUrl: 'https://w.soundcloud.com/player/?url=existing',
            height: 166
          }
        }
      }]
    };

    const { container } = render(PostCard, { props: { post } });

    // Give time for potential fetch
    await new Promise(r => setTimeout(r, 100));

    // Should render from metadata, not fetch
    const iframe = container.querySelector('iframe[src*="existing"]');
    expect(iframe).toBeTruthy();
    expect(mockFetch).not.toHaveBeenCalledWith(
      expect.stringContaining('soundcloud.com/oembed'),
      expect.any(Object)
    );
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
# Navigate to the app and test posting SoundCloud links
```

## Notes

- The oEmbed fetch is non-blocking - the post renders immediately, embed appears when ready
- Consider showing a loading skeleton for SoundCloud embeds while fetching
- If implementing loading state, ensure it doesn't flash (minimum display time or only show after delay)
- The `metadata?.embed?.embedUrl` check assumes backend SoundCloud metadata uses the same structure
- Verify the existing SoundCloud embed detection in PostCard and align with it
