import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
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

const { commentStore, postStore } = await import('../../../stores');
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
    const textarea = screen.getByLabelText('Add a comment');

    await fireEvent.input(textarea, { target: { value: 'Nice' } });
    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);

    expect(createComment).toHaveBeenCalled();
    expect(addCommentSpy).toHaveBeenCalled();
    expect(incrementSpy).toHaveBeenCalledWith('post-1', 1);
    expect((textarea as HTMLTextAreaElement).value).toBe('');
  });

  it('skips comment count increment when websocket already counted', async () => {
    const incrementSpy = vi.spyOn(postStore, 'incrementCommentCount');

    mapApiComment.mockReturnValue({
      id: 'comment-1',
      postId: 'post-1',
      userId: 'user-1',
      content: 'Nice',
      createdAt: 'now',
    });
    createComment.mockResolvedValue({ comment: { id: 'comment-1' } });
    commentStore.markSeenComment('post-1', 'comment-1');

    const { container } = render(CommentForm, { postId: 'post-1' });
    const textarea = screen.getByLabelText('Add a comment');

    await fireEvent.input(textarea, { target: { value: 'Nice' } });
    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);

    expect(incrementSpy).not.toHaveBeenCalled();
  });

  it('shows error on failure', async () => {
    createComment.mockRejectedValue(new Error('boom'));

    const { container } = render(CommentForm, { postId: 'post-1' });
    const textarea = screen.getByLabelText('Add a comment');
    await fireEvent.input(textarea, { target: { value: 'Nice' } });

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);

    expect(screen.getByText('boom')).toBeInTheDocument();
  });

  it('submits timestamp when enabled', async () => {
    mapApiComment.mockReturnValue({
      id: 'comment-2',
      postId: 'post-1',
      userId: 'user-1',
      content: 'Nice',
      createdAt: 'now',
    });
    createComment.mockResolvedValue({ comment: { id: 'comment-2' } });

    const { container } = render(CommentForm, { postId: 'post-1', allowTimestamp: true });
    const textarea = screen.getByLabelText('Add a comment');
    const timestampInput = screen.getByLabelText('Timestamp (mm:ss or hh:mm:ss)');

    await fireEvent.input(textarea, { target: { value: 'Nice' } });
    await fireEvent.input(timestampInput, { target: { value: '02:30' } });
    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);

    expect(createComment).toHaveBeenCalled();
    const payload = createComment.mock.calls[0][0];
    expect(payload.timestampSeconds).toBe(150);
  });

  it('shows timestamp validation error', async () => {
    const { container } = render(CommentForm, { postId: 'post-1', allowTimestamp: true });
    const textarea = screen.getByLabelText('Add a comment');
    const timestampInput = screen.getByLabelText('Timestamp (mm:ss or hh:mm:ss)');

    await fireEvent.input(textarea, { target: { value: 'Nice' } });
    await fireEvent.input(timestampInput, { target: { value: '2:3' } });
    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);

    expect(createComment).not.toHaveBeenCalled();
    expect(screen.getByText('Enter a timestamp in mm:ss or hh:mm:ss format.')).toBeInTheDocument();
  });

  it('shows spoiler toggle for book posts and sends spoiler flag', async () => {
    mapApiComment.mockReturnValue({
      id: 'comment-3',
      postId: 'post-1',
      userId: 'user-1',
      content: 'Spoiler comment',
      containsSpoiler: true,
      createdAt: 'now',
    });
    createComment.mockResolvedValue({ comment: { id: 'comment-3' } });

    const { container } = render(CommentForm, {
      postId: 'post-1',
      allowSpoiler: true,
    });

    const textarea = screen.getByLabelText('Add a comment');
    await fireEvent.input(textarea, { target: { value: 'Spoiler comment' } });

    const spoilerToggle = screen.getByLabelText('Contains spoiler');
    await fireEvent.click(spoilerToggle);

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);

    expect(createComment).toHaveBeenCalled();
    expect(createComment.mock.calls[0][0]).toMatchObject({
      postId: 'post-1',
      content: 'Spoiler comment',
      containsSpoiler: true,
    });
  });

  it('hides spoiler toggle for non-book posts', () => {
    render(CommentForm, { postId: 'post-1' });
    expect(screen.queryByLabelText('Contains spoiler')).not.toBeInTheDocument();
  });
});
