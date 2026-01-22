import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';
import { commentStore } from '../../../stores';
import { afterEach } from 'vitest';

const loadThreadComments = vi.hoisted(() => vi.fn());
const loadMoreThreadComments = vi.hoisted(() => vi.fn());

vi.mock('../../../stores/commentFeedStore', () => ({
  loadThreadComments,
  loadMoreThreadComments,
}));

const { default: CommentThread } = await import('../CommentThread.svelte');

beforeEach(() => {
  loadThreadComments.mockReset();
  loadMoreThreadComments.mockReset();
  const state = commentStore as unknown as { resetThread: (postId: string) => void };
  if (state.resetThread) {
    state.resetThread('post-1');
  }
});

afterEach(() => {
  cleanup();
});

describe('CommentThread', () => {
  it('loads initial thread once when visible and comments exist', async () => {
    render(CommentThread, { postId: 'post-1', commentCount: 2 });
    const observer = (globalThis as { __lastObserver?: { trigger: (value: boolean) => void } }).__lastObserver;
    observer?.trigger(true);
    await tick();
    expect(loadThreadComments).toHaveBeenCalledTimes(1);
    expect(loadThreadComments).toHaveBeenCalledWith('post-1');
  });

  it('attaches observer when comment count flips from zero', async () => {
    const observerRef = globalThis as { __lastObserver?: { trigger: (value: boolean) => void } };
    observerRef.__lastObserver = undefined;
    const { component } = render(CommentThread, { postId: 'post-1', commentCount: 0 });

    component.$set({ commentCount: 1 });
    await tick();

    const observer = observerRef.__lastObserver;
    observer?.trigger(true);
    await tick();

    expect(loadThreadComments).toHaveBeenCalledTimes(1);
    expect(loadThreadComments).toHaveBeenCalledWith('post-1');
  });

  it('shows loading state', () => {
    commentStore.setLoading('post-1', true);
    render(CommentThread, { postId: 'post-1', commentCount: 0 });
    expect(screen.getByText('Loading comments...')).toBeInTheDocument();
  });

  it('shows error state', () => {
    commentStore.setError('post-1', 'boom');
    render(CommentThread, { postId: 'post-1', commentCount: 0 });
    expect(screen.getByText('boom')).toBeInTheDocument();
  });

  it('shows empty state', () => {
    commentStore.setThread('post-1', [], null, false);
    render(CommentThread, { postId: 'post-1', commentCount: 0 });
    expect(screen.getByText('No comments yet. Start the conversation.')).toBeInTheDocument();
  });

  it('renders replies and load more button', async () => {
    commentStore.setThread('post-1', [
      {
        id: 'comment-1',
        postId: 'post-1',
        userId: 'user-1',
        content: 'Hello',
        createdAt: 'now',
        user: { id: 'user-1', username: 'Sander' },
        replies: [],
      },
    ], 'cursor-1', true);

    render(CommentThread, { postId: 'post-1', commentCount: 1 });

    expect(screen.getByText('Load more comments')).toBeInTheDocument();

    const replyButton = screen.getByText('Reply');
    await fireEvent.click(replyButton);
    expect(screen.getByPlaceholderText('Write a reply...')).toBeInTheDocument();
  });
});
