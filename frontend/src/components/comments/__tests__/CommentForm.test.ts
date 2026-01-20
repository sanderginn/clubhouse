import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { commentStore, postStore } from '../../../stores';
import { afterEach } from 'vitest';

const createComment = vi.hoisted(() => vi.fn());
const mapApiComment = vi.hoisted(() => vi.fn());

vi.mock('../../../services/api', () => ({
  api: {
    createComment,
  },
}));

vi.mock('../../../stores/commentMapper', () => ({
  mapApiComment: (comment: unknown) => mapApiComment(comment),
}));

const { default: CommentForm } = await import('../CommentForm.svelte');

beforeEach(() => {
  createComment.mockReset();
  mapApiComment.mockReset();
  const state = (commentStore as unknown as { resetThread: (postId: string) => void });
  if (state.resetThread) {
    state.resetThread('post-1');
  }
  postStore.reset();
});

afterEach(() => {
  cleanup();
});

describe('CommentForm', () => {
  it('submits comment and clears input', async () => {
    const addCommentSpy = vi.spyOn(commentStore, 'addComment');
    const incrementSpy = vi.spyOn(postStore, 'incrementCommentCount');

    mapApiComment.mockReturnValue({
      id: 'comment-1',
      postId: 'post-1',
      userId: 'user-1',
      content: 'Nice',
      createdAt: 'now',
    });
    createComment.mockResolvedValue({ comment: { id: 'comment-1' } });

    const { container } = render(CommentForm, { postId: 'post-1' });
    const textarea = screen.getByPlaceholderText('Write a comment...');

    await fireEvent.input(textarea, { target: { value: 'Nice' } });
    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);

    expect(createComment).toHaveBeenCalled();
    expect(addCommentSpy).toHaveBeenCalled();
    expect(incrementSpy).toHaveBeenCalledWith('post-1', 1);
    expect((textarea as HTMLTextAreaElement).value).toBe('');
  });

  it('shows error on failure', async () => {
    createComment.mockRejectedValue(new Error('boom'));

    const { container } = render(CommentForm, { postId: 'post-1' });
    const textarea = screen.getByPlaceholderText('Write a comment...');
    await fireEvent.input(textarea, { target: { value: 'Nice' } });

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);

    expect(screen.getByText('boom')).toBeInTheDocument();
  });
});
