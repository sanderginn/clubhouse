import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { postStore } from '../postStore';

const basePost = {
  id: 'post-1',
  userId: 'user-1',
  sectionId: 'section-1',
  content: 'hello',
  createdAt: '2025-01-01T00:00:00Z',
};

beforeEach(() => {
  postStore.reset();
});

describe('postStore', () => {
  it('setPosts sets posts and clears loading/error', () => {
    postStore.setLoading(true);
    postStore.setError('oops');

    postStore.setPosts([basePost], 'cursor-1', false);
    const state = get(postStore);

    expect(state.posts).toHaveLength(1);
    expect(state.cursor).toBe('cursor-1');
    expect(state.hasMore).toBe(false);
    expect(state.isLoading).toBe(false);
    expect(state.error).toBeNull();
    expect(state.paginationError).toBeNull();
  });

  it('setLoading clears error when starting a request', () => {
    postStore.setError('boom');
    postStore.setLoading(true);

    const state = get(postStore);
    expect(state.isLoading).toBe(true);
    expect(state.error).toBeNull();
    expect(state.paginationError).toBeNull();
  });

  it('addPost prepends', () => {
    postStore.setPosts([basePost], null, true);
    postStore.addPost({ ...basePost, id: 'post-2' });

    const state = get(postStore);
    expect(state.posts[0].id).toBe('post-2');
    expect(state.posts[1].id).toBe('post-1');
  });

  it('upsertPost inserts or merges', () => {
    postStore.setPosts([basePost], null, true);
    postStore.upsertPost({ ...basePost, id: 'post-2', content: 'new' });

    let state = get(postStore);
    expect(state.posts[0].id).toBe('post-2');

    postStore.upsertPost({ id: 'post-1', content: 'updated', userId: 'user-1', sectionId: 'section-1', createdAt: '2025-01-01T00:00:00Z' });

    state = get(postStore);
    const updated = state.posts.find((post) => post.id === 'post-1');
    expect(updated?.content).toBe('updated');
    expect(updated?.userId).toBe('user-1');
  });

  it('appendPosts appends and updates cursor/hasMore', () => {
    postStore.setPosts([basePost], 'cursor-1', true);
    postStore.appendPosts([
      { ...basePost, id: 'post-2' },
      { ...basePost, id: 'post-3' },
    ], 'cursor-2', false);

    const state = get(postStore);
    expect(state.posts).toHaveLength(3);
    expect(state.posts[2].id).toBe('post-3');
    expect(state.cursor).toBe('cursor-2');
    expect(state.hasMore).toBe(false);
    expect(state.isLoading).toBe(false);
    expect(state.paginationError).toBeNull();
  });

  it('removePost removes by id', () => {
    postStore.setPosts([basePost, { ...basePost, id: 'post-2' }], null, true);
    postStore.removePost('post-1');
    const state = get(postStore);
    expect(state.posts).toHaveLength(1);
    expect(state.posts[0].id).toBe('post-2');
  });

  it('incrementCommentCount updates count and never below zero', () => {
    postStore.setPosts([{ ...basePost, commentCount: 1 }], null, true);
    postStore.incrementCommentCount('post-1', 1);
    let state = get(postStore);
    expect(state.posts[0].commentCount).toBe(2);

    postStore.incrementCommentCount('post-1', -5);
    state = get(postStore);
    expect(state.posts[0].commentCount).toBe(0);
  });

  it('updateReactionCount adds and removes emoji counts', () => {
    postStore.setPosts([{ ...basePost, reactionCounts: {} }], null, true);

    postStore.updateReactionCount('post-1', '🔥', 1);
    let state = get(postStore);
    expect(state.posts[0].reactionCounts?.['🔥']).toBe(1);

    postStore.updateReactionCount('post-1', '🔥', -1);
    state = get(postStore);
    expect(state.posts[0].reactionCounts?.['🔥']).toBeUndefined();
  });

  it('updateHighlightReaction updates counts and viewer state', () => {
    postStore.setPosts(
      [
        {
          ...basePost,
          links: [
            {
              id: 'link-1',
              url: 'https://example.com',
              highlights: [{ id: 'highlight-1', timestamp: 5, heartCount: 0, viewerReacted: false }],
            },
          ],
        },
      ],
      null,
      true
    );

    postStore.updateHighlightReaction('post-1', 'link-1', 'highlight-1', 1, true);
    let state = get(postStore);
    expect(state.posts[0].links?.[0].highlights?.[0].heartCount).toBe(1);
    expect(state.posts[0].links?.[0].highlights?.[0].viewerReacted).toBe(true);

    postStore.updateHighlightReaction('post-1', 'link-1', 'highlight-1', -1, false);
    state = get(postStore);
    expect(state.posts[0].links?.[0].highlights?.[0].heartCount).toBe(0);
    expect(state.posts[0].links?.[0].highlights?.[0].viewerReacted).toBe(false);
  });

  it('updateLinkMetadata updates metadata for a link', () => {
    postStore.setPosts(
      [
        {
          ...basePost,
          links: [{ id: 'link-1', url: 'https://example.com', metadata: undefined }],
        },
      ],
      null,
      true
    );

    postStore.updateLinkMetadata('post-1', 'link-1', {
      url: 'https://example.com',
      title: 'Test Title',
      embedUrl: 'https://embed.example.com',
    });

    const state = get(postStore);
    expect(state.posts[0].links?.[0].metadata?.title).toBe('Test Title');
    expect(state.posts[0].links?.[0].metadata?.embedUrl).toBe('https://embed.example.com');
  });

  it('updateLinkMetadata is a no-op for unknown post or link', () => {
    postStore.setPosts(
      [
        {
          ...basePost,
          links: [{ id: 'link-1', url: 'https://example.com', metadata: undefined }],
        },
      ],
      null,
      true
    );

    postStore.updateLinkMetadata('post-999', 'link-1', { url: 'https://example.com', title: 'X' });
    postStore.updateLinkMetadata('post-1', 'link-999', { url: 'https://example.com', title: 'X' });

    const state = get(postStore);
    expect(state.posts[0].links?.[0].metadata).toBeUndefined();
  });

  it('setMovieWatchlistState updates viewer flags and watchlist count', () => {
    postStore.setPosts(
      [
        {
          ...basePost,
          movieStats: {
            watchlistCount: 3,
            watchCount: 1,
            averageRating: 4,
            viewerWatchlisted: false,
            viewerWatched: false,
            viewerRating: null,
            viewerCategories: [],
          },
        },
      ],
      null,
      true
    );

    postStore.setMovieWatchlistState('post-1', true, ['Favorites']);
    let state = get(postStore);
    expect(state.posts[0].movieStats?.watchlistCount).toBe(4);
    expect(state.posts[0].movieStats?.viewerWatchlisted).toBe(true);
    expect(state.posts[0].movieStats?.viewerCategories).toEqual(['Favorites']);

    postStore.setMovieWatchlistState('post-1', false, []);
    state = get(postStore);
    expect(state.posts[0].movieStats?.watchlistCount).toBe(3);
    expect(state.posts[0].movieStats?.viewerWatchlisted).toBe(false);
    expect(state.posts[0].movieStats?.viewerCategories).toEqual([]);
  });

  it('setMovieWatchState updates watch count and average rating', () => {
    postStore.setPosts(
      [
        {
          ...basePost,
          movieStats: {
            watchlistCount: 2,
            watchCount: 2,
            averageRating: 4,
            viewerWatchlisted: false,
            viewerWatched: false,
            viewerRating: null,
            viewerCategories: [],
          },
        },
      ],
      null,
      true
    );

    postStore.setMovieWatchState('post-1', true, 5);
    let state = get(postStore);
    expect(state.posts[0].movieStats?.watchCount).toBe(3);
    expect(state.posts[0].movieStats?.averageRating).toBeCloseTo(4.333, 3);
    expect(state.posts[0].movieStats?.viewerWatched).toBe(true);
    expect(state.posts[0].movieStats?.viewerRating).toBe(5);

    postStore.setMovieWatchState('post-1', true, 3);
    state = get(postStore);
    expect(state.posts[0].movieStats?.watchCount).toBe(3);
    expect(state.posts[0].movieStats?.averageRating).toBeCloseTo(3.667, 3);
    expect(state.posts[0].movieStats?.viewerRating).toBe(3);

    postStore.setMovieWatchState('post-1', false, null);
    state = get(postStore);
    expect(state.posts[0].movieStats?.watchCount).toBe(2);
    expect(state.posts[0].movieStats?.averageRating).toBe(4);
    expect(state.posts[0].movieStats?.viewerWatched).toBe(false);
    expect(state.posts[0].movieStats?.viewerRating).toBeNull();
  });

  it('setMovieStats overwrites reconciled movie aggregate fields', () => {
    postStore.setPosts([basePost], null, true);

    postStore.setMovieStats('post-1', {
      watchlistCount: 6,
      watchCount: 4,
      averageRating: 4.5,
      viewerWatchlisted: true,
      viewerWatched: true,
      viewerRating: 5,
      viewerCategories: ['Top Picks'],
    });

    const state = get(postStore);
    expect(state.posts[0].movieStats).toMatchObject({
      watchlistCount: 6,
      watchCount: 4,
      averageRating: 4.5,
      viewerWatchlisted: true,
      viewerWatched: true,
      viewerRating: 5,
      viewerCategories: ['Top Picks'],
    });
  });

  it('reset restores defaults', () => {
    postStore.setPosts([basePost], 'cursor-1', false);
    postStore.reset();
    const state = get(postStore);
    expect(state.posts).toHaveLength(0);
    expect(state.cursor).toBeNull();
    expect(state.hasMore).toBe(true);
    expect(state.error).toBeNull();
    expect(state.paginationError).toBeNull();
  });

  it('updateUserProfilePicture updates matching post users', () => {
    postStore.setPosts(
      [
        {
          ...basePost,
          user: { id: 'user-1', username: 'sander', profilePictureUrl: 'old-url' },
        },
        {
          ...basePost,
          id: 'post-2',
          userId: 'user-2',
          user: { id: 'user-2', username: 'alex', profilePictureUrl: 'keep-url' },
        },
      ],
      null,
      true
    );

    postStore.updateUserProfilePicture('user-1', 'new-url');

    const state = get(postStore);
    expect(state.posts[0].user?.profilePictureUrl).toBe('new-url');
    expect(state.posts[1].user?.profilePictureUrl).toBe('keep-url');
  });
});
