import { describe, expect, it } from 'vitest';
import { get } from 'svelte/store';
import { commentStore } from '../commentStore';

function resetStore() {
  const state = get(commentStore);
  Object.keys(state).forEach((postId) => commentStore.resetThread(postId));
}

describe('commentStore', () => {
  it('sets and appends thread comments', () => {
    resetStore();

    commentStore.setThread(
      'post-1',
      [
        {
          id: 'comment-1',
          userId: 'user-1',
          postId: 'post-1',
          content: 'Hello',
          createdAt: '2025-01-01T00:00:00Z',
        },
      ],
      'cursor-1',
      true
    );

    let state = get(commentStore)['post-1'];
    expect(state.comments).toHaveLength(1);
    expect(state.cursor).toBe('cursor-1');
    expect(state.hasMore).toBe(true);
    expect(state.isLoading).toBe(false);

    commentStore.appendThread(
      'post-1',
      [
        {
          id: 'comment-2',
          userId: 'user-2',
          postId: 'post-1',
          content: 'More',
          createdAt: '2025-01-02T00:00:00Z',
        },
      ],
      null,
      false
    );

    state = get(commentStore)['post-1'];
    expect(state.comments).toHaveLength(2);
    expect(state.cursor).toBe(null);
    expect(state.hasMore).toBe(false);
  });

  it('adds a reply to a comment', () => {
    resetStore();

    commentStore.setThread(
      'post-2',
      [
        {
          id: 'comment-10',
          userId: 'user-10',
          postId: 'post-2',
          content: 'Parent',
          createdAt: '2025-01-01T00:00:00Z',
          replies: [],
        },
      ],
      null,
      true
    );

    commentStore.addReply('post-2', 'comment-10', {
      id: 'reply-1',
      userId: 'user-11',
      postId: 'post-2',
      parentCommentId: 'comment-10',
      content: 'Reply',
      createdAt: '2025-01-02T00:00:00Z',
    });

    const state = get(commentStore)['post-2'];
    expect(state.comments[0].replies).toHaveLength(1);
    expect(state.comments[0].replies?.[0].id).toBe('reply-1');
  });

  it('adds comment to top of thread', () => {
    resetStore();

    commentStore.setThread(
      'post-3',
      [
        {
          id: 'comment-20',
          userId: 'user-20',
          postId: 'post-3',
          content: 'Existing',
          createdAt: '2025-01-01T00:00:00Z',
        },
      ],
      null,
      true
    );

    commentStore.addComment('post-3', {
      id: 'comment-21',
      userId: 'user-21',
      postId: 'post-3',
      content: 'New',
      createdAt: '2025-01-02T00:00:00Z',
    });

    const state = get(commentStore)['post-3'];
    expect(state.comments[0].id).toBe('comment-21');
    expect(state.comments[1].id).toBe('comment-20');
  });
});
