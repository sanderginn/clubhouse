import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { commentStore } from '../commentStore';

const apiGetThreadComments = vi.hoisted(() => vi.fn());
const mapApiComment = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    getThreadComments: apiGetThreadComments,
  },
}));

vi.mock('../commentMapper', () => ({
  mapApiComment: (comment: unknown) => mapApiComment(comment),
}));

const { loadThreadComments, loadMoreThreadComments } = await import('../commentFeedStore');

beforeEach(() => {
  apiGetThreadComments.mockReset();
  mapApiComment.mockReset();
  const state = get(commentStore);
  Object.keys(state).forEach((postId) => commentStore.resetThread(postId));
});

describe('commentFeedStore', () => {
  it('loadThreadComments success sets thread', async () => {
    mapApiComment.mockImplementation((comment) => ({ id: comment.id, postId: 'post-1', userId: 'u', content: 'x', createdAt: 'now' }));
    apiGetThreadComments.mockResolvedValue({
      comments: [{ id: 'comment-1' }],
      meta: { cursor: 'cursor-1', has_more: true },
    });

    await loadThreadComments('post-1');
    const thread = get(commentStore)['post-1'];

    expect(thread.comments).toHaveLength(1);
    expect(thread.cursor).toBe('cursor-1');
    expect(thread.hasMore).toBe(true);
    expect(thread.isLoading).toBe(false);
  });

  it('loadThreadComments failure sets error', async () => {
    apiGetThreadComments.mockRejectedValue(new Error('fail'));

    await loadThreadComments('post-1');
    const thread = get(commentStore)['post-1'];

    expect(thread.error).toBe('fail');
    expect(thread.isLoading).toBe(false);
  });

  it('loadMoreThreadComments guard skips request', async () => {
    await loadMoreThreadComments('post-1');
    expect(apiGetThreadComments).not.toHaveBeenCalled();

    commentStore.setThread('post-1', [], null, false);
    await loadMoreThreadComments('post-1');
    expect(apiGetThreadComments).not.toHaveBeenCalled();
  });

  it('loadMoreThreadComments success appends', async () => {
    mapApiComment.mockImplementation((comment) => ({ id: comment.id, postId: 'post-1', userId: 'u', content: 'x', createdAt: 'now' }));
    commentStore.setThread('post-1', [], 'cursor-1', true);

    apiGetThreadComments.mockResolvedValue({
      comments: [{ id: 'comment-2' }],
      meta: { cursor: null, has_more: false },
    });

    await loadMoreThreadComments('post-1');
    const thread = get(commentStore)['post-1'];
    expect(thread.comments).toHaveLength(1);
    expect(thread.hasMore).toBe(false);
  });

  it('loadMoreThreadComments failure sets error', async () => {
    commentStore.setThread('post-1', [], 'cursor-1', true);
    apiGetThreadComments.mockRejectedValue(new Error('boom'));

    await loadMoreThreadComments('post-1');
    const thread = get(commentStore)['post-1'];
    expect(thread.error).toBe('boom');
    expect(thread.isLoading).toBe(false);
  });
});
