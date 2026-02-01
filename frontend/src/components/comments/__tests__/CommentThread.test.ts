import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';
import { commentStore, authStore } from '../../../stores';
import { afterEach } from 'vitest';

const loadThreadComments = vi.hoisted(() => vi.fn());
const loadMoreThreadComments = vi.hoisted(() => vi.fn());
const apiDeleteComment = vi.hoisted(() => vi.fn());
const apiUpdateComment = vi.hoisted(() => vi.fn());

vi.mock('../../../stores/commentFeedStore', () => ({
  loadThreadComments,
  loadMoreThreadComments,
}));

vi.mock('../../../services/api', () => ({
  api: {
    deleteComment: apiDeleteComment,
    updateComment: apiUpdateComment,
  },
}));

const { default: CommentThread } = await import('../CommentThread.svelte');

beforeEach(() => {
  loadThreadComments.mockReset();
  loadMoreThreadComments.mockReset();
  apiUpdateComment.mockReset();
  authStore.setUser(null);
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

  it('hides load more button when all comments are loaded', async () => {
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

    expect(screen.queryByText('Load more comments')).not.toBeInTheDocument();
  });

  it('renders replies and load more button when more comments exist', async () => {
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

    render(CommentThread, { postId: 'post-1', commentCount: 2 });

    expect(screen.getByText('Load more comments')).toBeInTheDocument();

    const replyButton = screen.getByText('Reply');
    await fireEvent.click(replyButton);
    expect(screen.getByPlaceholderText('Write a reply...')).toBeInTheDocument();
  });

  it('shows edit action for own comment', () => {
    authStore.setUser({
      id: 'user-1',
      username: 'Sander',
      email: 'sander@example.com',
      isAdmin: false,
      totpEnabled: false,
    });

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
    ], null, false);

    render(CommentThread, { postId: 'post-1', commentCount: 1 });
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
  });

  it('shows delete action for own comment', async () => {
    authStore.setUser({
      id: 'user-1',
      username: 'Sander',
      email: 'sander@example.com',
      isAdmin: false,
      totpEnabled: false,
    });

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
    ], null, false);

    render(CommentThread, { postId: 'post-1', commentCount: 1 });
    expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
  });

  it('shows delete action for admins on others comments', async () => {
    authStore.setUser({
      id: 'admin-1',
      username: 'Admin',
      email: 'admin@example.com',
      isAdmin: true,
      totpEnabled: false,
    });

    commentStore.setThread('post-1', [
      {
        id: 'comment-1',
        postId: 'post-1',
        userId: 'user-2',
        content: 'Hello',
        createdAt: 'now',
        user: { id: 'user-2', username: 'Other' },
        replies: [],
      },
    ], null, false);

    render(CommentThread, { postId: 'post-1', commentCount: 1 });
    expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument();
  });

  it('shows reply action for comments with images', () => {
    commentStore.setThread('post-1', [
      {
        id: 'comment-1',
        postId: 'post-1',
        userId: 'user-1',
        content: 'Image comment',
        imageId: 'image-1',
        createdAt: 'now',
        user: { id: 'user-1', username: 'Sander' },
        replies: [],
      },
    ], null, false);

    render(CommentThread, {
      postId: 'post-1',
      commentCount: 1,
      imageItems: [
        {
          id: 'image-1',
          url: 'https://cdn.example.com/uploads/photo.png',
          title: 'Image 1',
        },
      ],
    });

    expect(screen.getByRole('button', { name: 'Reply' })).toBeInTheDocument();
    expect(screen.getByText('Image 1')).toBeInTheDocument();
  });

  it('saves edits with ctrl+enter', async () => {
    authStore.setUser({
      id: 'user-1',
      username: 'Sander',
      email: 'sander@example.com',
      isAdmin: false,
      totpEnabled: false,
    });

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
    ], null, false);

    apiUpdateComment.mockResolvedValue({
      comment: {
        id: 'comment-1',
        postId: 'post-1',
        userId: 'user-1',
        content: 'Hello',
        createdAt: 'now',
        user: { id: 'user-1', username: 'Sander' },
        replies: [],
      },
    });

    render(CommentThread, { postId: 'post-1', commentCount: 1 });

    await fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

    const textareas = screen.getAllByRole('textbox');
    const textarea = textareas.find((area) => area.getAttribute('rows') === '3');
    if (!textarea) {
      throw new Error('Edit textarea not found');
    }
    await fireEvent.keyDown(textarea, { key: 'Enter', ctrlKey: true });

    expect(apiUpdateComment).toHaveBeenCalledWith('comment-1', {
      content: 'Hello',
      mentionUsernames: [],
    });
  });

  it('ignores ctrl+enter when edit content is empty', async () => {
    authStore.setUser({
      id: 'user-1',
      username: 'Sander',
      email: 'sander@example.com',
      isAdmin: false,
      totpEnabled: false,
    });

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
    ], null, false);

    apiUpdateComment.mockResolvedValue({
      comment: {
        id: 'comment-1',
        postId: 'post-1',
        userId: 'user-1',
        content: 'Hello',
        createdAt: 'now',
        user: { id: 'user-1', username: 'Sander' },
        replies: [],
      },
    });

    render(CommentThread, { postId: 'post-1', commentCount: 1 });

    await fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

    const textareas = screen.getAllByRole('textbox');
    const textarea = textareas.find((area) => area.getAttribute('rows') === '3');
    if (!textarea) {
      throw new Error('Edit textarea not found');
    }
    await fireEvent.input(textarea, { target: { value: '   ' } });
    await fireEvent.keyDown(textarea, { key: 'Enter', ctrlKey: true });

    expect(apiUpdateComment).not.toHaveBeenCalled();
  });

  describe('profile highlighting', () => {
    it('highlights top-level comment by profile user with amber-50', () => {
      commentStore.setThread('post-1', [
        {
          id: 'comment-1',
          postId: 'post-1',
          userId: 'profile-user',
          content: 'Profile user comment',
          createdAt: 'now',
          user: { id: 'profile-user', username: 'ProfileUser' },
          replies: [],
        },
      ], null, false);

      render(CommentThread, {
        postId: 'post-1',
        commentCount: 1,
        highlightCommentIds: ['comment-1'],
        profileUserId: 'profile-user',
      });

      const article = document.getElementById('comment-comment-1');
      expect(article?.className).toContain('bg-amber-50');
      expect(article?.className).toContain('ring-amber-300');
    });

    it('does not highlight comment by other user even if in highlightCommentIds', () => {
      commentStore.setThread('post-1', [
        {
          id: 'comment-1',
          postId: 'post-1',
          userId: 'other-user',
          content: 'Other user comment',
          createdAt: 'now',
          user: { id: 'other-user', username: 'OtherUser' },
          replies: [],
        },
      ], null, false);

      render(CommentThread, {
        postId: 'post-1',
        commentCount: 1,
        highlightCommentIds: ['comment-1'],
        profileUserId: 'profile-user',
      });

      const article = document.getElementById('comment-comment-1');
      expect(article?.className).toContain('bg-white');
      expect(article?.className).not.toContain('ring-amber');
    });

    it('highlights nested reply by profile user with darker amber-100', () => {
      commentStore.setThread('post-1', [
        {
          id: 'comment-1',
          postId: 'post-1',
          userId: 'other-user',
          content: 'Parent comment',
          createdAt: 'now',
          user: { id: 'other-user', username: 'OtherUser' },
          replies: [
            {
              id: 'reply-1',
              postId: 'post-1',
              userId: 'profile-user',
              parentCommentId: 'comment-1',
              content: 'Profile user reply',
              createdAt: 'now',
              user: { id: 'profile-user', username: 'ProfileUser' },
            },
          ],
        },
      ], null, false);

      render(CommentThread, {
        postId: 'post-1',
        commentCount: 2,
        highlightCommentIds: ['reply-1'],
        profileUserId: 'profile-user',
      });

      const replyEl = document.getElementById('comment-reply-1');
      expect(replyEl?.className).toContain('bg-amber-100');
      expect(replyEl?.className).toContain('ring-amber-400');
    });

    it('does not highlight nested reply by other user', () => {
      commentStore.setThread('post-1', [
        {
          id: 'comment-1',
          postId: 'post-1',
          userId: 'profile-user',
          content: 'Profile user comment',
          createdAt: 'now',
          user: { id: 'profile-user', username: 'ProfileUser' },
          replies: [
            {
              id: 'reply-1',
              postId: 'post-1',
              userId: 'other-user',
              parentCommentId: 'comment-1',
              content: 'Other user reply',
              createdAt: 'now',
              user: { id: 'other-user', username: 'OtherUser' },
            },
          ],
        },
      ], null, false);

      render(CommentThread, {
        postId: 'post-1',
        commentCount: 2,
        highlightCommentIds: ['comment-1', 'reply-1'],
        profileUserId: 'profile-user',
      });

      const replyEl = document.getElementById('comment-reply-1');
      expect(replyEl?.className).not.toContain('bg-amber');
      expect(replyEl?.className).not.toContain('ring-amber');
      expect(replyEl?.className).toContain('bg-white');
      expect(replyEl?.className).toContain('rounded-lg');
      expect(replyEl?.className).toContain('p-2');
    });

    it('highlights comments without profileUserId filter (backward compatibility)', () => {
      commentStore.setThread('post-1', [
        {
          id: 'comment-1',
          postId: 'post-1',
          userId: 'any-user',
          content: 'Any user comment',
          createdAt: 'now',
          user: { id: 'any-user', username: 'AnyUser' },
          replies: [],
        },
      ], null, false);

      render(CommentThread, {
        postId: 'post-1',
        commentCount: 1,
        highlightCommentIds: ['comment-1'],
      });

      const article = document.getElementById('comment-comment-1');
      expect(article?.className).toContain('bg-amber-50');
      expect(article?.className).toContain('ring-amber-300');
    });
  });
});
