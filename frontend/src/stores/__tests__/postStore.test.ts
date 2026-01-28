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
  });

  it('setLoading clears error when starting a request', () => {
    postStore.setError('boom');
    postStore.setLoading(true);

    const state = get(postStore);
    expect(state.isLoading).toBe(true);
    expect(state.error).toBeNull();
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

  it('reset restores defaults', () => {
    postStore.setPosts([basePost], 'cursor-1', false);
    postStore.reset();
    const state = get(postStore);
    expect(state.posts).toHaveLength(0);
    expect(state.cursor).toBeNull();
    expect(state.hasMore).toBe(true);
    expect(state.error).toBeNull();
  });
});
