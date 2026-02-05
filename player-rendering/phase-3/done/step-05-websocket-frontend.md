# Phase 3, Step 5: WebSocket Handler for Metadata Updates (Frontend)

## Overview

Add frontend handling for the `link_metadata_updated` WebSocket event to update post cards in real-time when metadata arrives.

## Detailed Description

Modify the frontend to:

1. Handle `link_metadata_updated` events in the WebSocket store
2. Add an `updateLinkMetadata` method to the post store
3. PostCard components will reactively update when their link metadata changes

### Event Flow

```
Backend Worker fetches metadata
    ↓
Publishes to Redis pub/sub
    ↓
WebSocket server broadcasts to clients
    ↓
Frontend websocketStore receives event
    ↓
Calls postStore.updateLinkMetadata()
    ↓
PostCard reactively re-renders with new metadata
```

## Files to Modify

| File | Changes |
|------|---------|
| `frontend/src/stores/websocketStore.ts` | Handle `link_metadata_updated` event |
| `frontend/src/stores/postStore.ts` | Add `updateLinkMetadata()` method |

## Expected Outcomes

1. `link_metadata_updated` events are received and processed
2. Post store is updated with new link metadata
3. PostCard components reactively show new metadata
4. Embeds appear for links that now have metadata
5. No errors on malformed events
6. All tests pass

## Implementation

### websocketStore.ts

Add case to the message handler:

```typescript
// In the WebSocket message handler switch statement
case 'link_metadata_updated': {
    const payload = parsed.data as {
        post_id: string;
        link_id: string;
        metadata: LinkMetadata;
    };

    if (payload?.post_id && payload?.link_id && payload?.metadata) {
        postStore.updateLinkMetadata(
            payload.post_id,
            payload.link_id,
            payload.metadata
        );
    } else {
        console.warn('Invalid link_metadata_updated payload', payload);
    }
    break;
}
```

### postStore.ts

Add the update method:

```typescript
import { writable, get } from 'svelte/store';
import type { Post, LinkMetadata } from '$lib/types';

// Assuming posts is a writable store of Record<string, Post>
export const posts = writable<Record<string, Post>>({});

export function updateLinkMetadata(
    postId: string,
    linkId: string,
    metadata: LinkMetadata
): void {
    posts.update(currentPosts => {
        const post = currentPosts[postId];
        if (!post) {
            // Post not in store, nothing to update
            return currentPosts;
        }

        if (!post.links) {
            return currentPosts;
        }

        // Find and update the specific link
        const linkIndex = post.links.findIndex(l => l.id === linkId);
        if (linkIndex === -1) {
            return currentPosts;
        }

        // Create updated post with new metadata
        const updatedLinks = [...post.links];
        updatedLinks[linkIndex] = {
            ...updatedLinks[linkIndex],
            metadata
        };

        return {
            ...currentPosts,
            [postId]: {
                ...post,
                links: updatedLinks
            }
        };
    });
}

// Export for use in websocketStore
export const postStore = {
    subscribe: posts.subscribe,
    updateLinkMetadata
};
```

### Type Definition (if not exists)

Ensure `LinkMetadata` type is defined in `$lib/types`:

```typescript
export interface LinkMetadata {
    title?: string;
    description?: string;
    image?: string;
    site_name?: string;
    embed?: {
        provider: string;
        embedUrl: string;
        height?: number;
    };
    // Add other fields as needed
}

export interface Link {
    id: string;
    url: string;
    display_order: number;
    metadata: LinkMetadata | null;
}

export interface Post {
    id: string;
    content: string;
    links: Link[];
    // ... other fields
}
```

## Test Cases

### postStore.test.ts

```typescript
import { get } from 'svelte/store';
import { posts, updateLinkMetadata } from './postStore';

describe('postStore', () => {
    beforeEach(() => {
        // Reset store
        posts.set({});
    });

    describe('updateLinkMetadata', () => {
        it('updates metadata for existing link', () => {
            // Setup: add a post with a link
            posts.set({
                'post-1': {
                    id: 'post-1',
                    content: 'Test post',
                    links: [{
                        id: 'link-1',
                        url: 'https://bandcamp.com/album/test',
                        display_order: 0,
                        metadata: null
                    }]
                }
            });

            // Update metadata
            updateLinkMetadata('post-1', 'link-1', {
                title: 'Test Album',
                description: 'Great music',
                embed: {
                    provider: 'bandcamp',
                    embedUrl: 'https://bandcamp.com/embed/...',
                    height: 120
                }
            });

            // Verify
            const state = get(posts);
            expect(state['post-1'].links[0].metadata).toEqual({
                title: 'Test Album',
                description: 'Great music',
                embed: {
                    provider: 'bandcamp',
                    embedUrl: 'https://bandcamp.com/embed/...',
                    height: 120
                }
            });
        });

        it('does nothing for non-existent post', () => {
            posts.set({
                'post-1': {
                    id: 'post-1',
                    content: 'Test',
                    links: []
                }
            });

            // Try to update non-existent post
            updateLinkMetadata('post-999', 'link-1', { title: 'Test' });

            // State should be unchanged
            const state = get(posts);
            expect(Object.keys(state)).toEqual(['post-1']);
        });

        it('does nothing for non-existent link', () => {
            posts.set({
                'post-1': {
                    id: 'post-1',
                    content: 'Test',
                    links: [{
                        id: 'link-1',
                        url: 'https://example.com',
                        display_order: 0,
                        metadata: null
                    }]
                }
            });

            // Try to update non-existent link
            updateLinkMetadata('post-1', 'link-999', { title: 'Test' });

            // Original link should be unchanged
            const state = get(posts);
            expect(state['post-1'].links[0].metadata).toBeNull();
        });

        it('preserves other posts when updating one', () => {
            posts.set({
                'post-1': {
                    id: 'post-1',
                    content: 'Post 1',
                    links: [{
                        id: 'link-1',
                        url: 'https://example.com',
                        display_order: 0,
                        metadata: null
                    }]
                },
                'post-2': {
                    id: 'post-2',
                    content: 'Post 2',
                    links: []
                }
            });

            updateLinkMetadata('post-1', 'link-1', { title: 'Updated' });

            const state = get(posts);
            expect(state['post-2'].content).toBe('Post 2');
        });

        it('preserves other links when updating one', () => {
            posts.set({
                'post-1': {
                    id: 'post-1',
                    content: 'Test',
                    links: [
                        { id: 'link-1', url: 'https://a.com', display_order: 0, metadata: null },
                        { id: 'link-2', url: 'https://b.com', display_order: 1, metadata: { title: 'B' } }
                    ]
                }
            });

            updateLinkMetadata('post-1', 'link-1', { title: 'A' });

            const state = get(posts);
            expect(state['post-1'].links[0].metadata?.title).toBe('A');
            expect(state['post-1'].links[1].metadata?.title).toBe('B');
        });
    });
});
```

### websocketStore.test.ts

```typescript
// Add to existing websocketStore tests

describe('link_metadata_updated handler', () => {
    it('updates post store when receiving event', () => {
        // Setup: mock postStore.updateLinkMetadata
        const updateSpy = vi.spyOn(postStore, 'updateLinkMetadata');

        // Simulate WebSocket message
        const event = {
            type: 'link_metadata_updated',
            data: {
                post_id: 'post-123',
                link_id: 'link-456',
                metadata: {
                    title: 'Test Title',
                    embed: { provider: 'bandcamp', embedUrl: 'https://...' }
                }
            }
        };

        // Trigger message handler (implementation depends on store structure)
        handleWebSocketMessage(JSON.stringify(event));

        expect(updateSpy).toHaveBeenCalledWith(
            'post-123',
            'link-456',
            expect.objectContaining({ title: 'Test Title' })
        );
    });

    it('handles malformed event gracefully', () => {
        const consoleSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});

        const event = {
            type: 'link_metadata_updated',
            data: {
                // Missing required fields
                post_id: 'post-123'
            }
        };

        // Should not throw
        expect(() => handleWebSocketMessage(JSON.stringify(event))).not.toThrow();
        expect(consoleSpy).toHaveBeenCalled();

        consoleSpy.mockRestore();
    });
});
```

## Verification

```bash
# Run store tests
cd frontend && npm run test -- postStore
cd frontend && npm run test -- websocketStore

# Verify TypeScript
cd frontend && npm run check

# Manual testing:
# 1. Start dev server
# 2. Open browser with dev tools console
# 3. Create a post with a Bandcamp link
# 4. Watch console for link_metadata_updated event
# 5. Verify post card updates to show embed/metadata
```

## Notes

- Check existing WebSocket message handling in `websocketStore.ts` for the pattern to follow
- The post store structure might differ - verify how posts are stored (by ID, in array, etc.)
- PostCard should already reactively update when its props change (via store subscription)
- Consider adding a visual indicator (subtle fade-in) when metadata arrives
- If posts are stored in an array instead of a map, adjust the update logic accordingly
