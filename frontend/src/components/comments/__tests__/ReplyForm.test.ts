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

const { default: ReplyForm } = await import('../ReplyForm.svelte');

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

describe('ReplyForm', () => {
  it('submits reply and fires submit event', async () => {
    const addReplySpy = vi.spyOn(commentStore, 'addReply');
    const incrementSpy = vi.spyOn(postStore, 'incrementCommentCount');
    mapApiComment.mockReturnValue({
      id: 'reply-1',
      postId: 'post-1',
      parentCommentId: 'comment-1',
      userId: 'user-1',
      content: 'Reply',
      createdAt: 'now',
    });
    createComment.mockResolvedValue({ comment: { id: 'reply-1' } });

    const { component, container } = render(ReplyForm, {
      postId: 'post-1',
      parentCommentId: 'comment-1',
    });

    const submitHandler = vi.fn();
    component.$on('submit', submitHandler);

    const textarea = screen.getByPlaceholderText('Write a reply...');
    await fireEvent.input(textarea, { target: { value: 'Reply' } });

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);

    expect(createComment).toHaveBeenCalled();
    expect(addReplySpy).toHaveBeenCalled();
    expect(incrementSpy).toHaveBeenCalledWith('post-1', 1);
    expect(submitHandler).toHaveBeenCalled();
  });

  it('cancel button dispatches cancel', async () => {
    const { component } = render(ReplyForm, {
      postId: 'post-1',
      parentCommentId: 'comment-1',
    });
    const cancelHandler = vi.fn();
    component.$on('cancel', cancelHandler);

    const cancelButton = screen.getByText('Cancel');
    await fireEvent.click(cancelButton);

    expect(cancelHandler).toHaveBeenCalled();
  });

  it('skips comment count increment when websocket already counted', async () => {
    const incrementSpy = vi.spyOn(postStore, 'incrementCommentCount');
    mapApiComment.mockReturnValue({
      id: 'reply-1',
      postId: 'post-1',
      parentCommentId: 'comment-1',
      userId: 'user-1',
      content: 'Reply',
      createdAt: 'now',
    });
    createComment.mockResolvedValue({ comment: { id: 'reply-1' } });
    commentStore.markSeenComment('post-1', 'reply-1');

    const { container } = render(ReplyForm, {
      postId: 'post-1',
      parentCommentId: 'comment-1',
    });

    const textarea = screen.getByPlaceholderText('Write a reply...');
    await fireEvent.input(textarea, { target: { value: 'Reply' } });

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);

    expect(incrementSpy).not.toHaveBeenCalled();
  });

  it('shows spoiler toggle for book replies and sends spoiler flag', async () => {
    mapApiComment.mockReturnValue({
      id: 'reply-2',
      postId: 'post-1',
      parentCommentId: 'comment-1',
      userId: 'user-1',
      content: 'Spoiler reply',
      containsSpoiler: true,
      createdAt: 'now',
    });
    createComment.mockResolvedValue({ comment: { id: 'reply-2' } });

    const { container } = render(ReplyForm, {
      postId: 'post-1',
      parentCommentId: 'comment-1',
      allowSpoiler: true,
    });

    const textarea = screen.getByPlaceholderText('Write a reply...');
    await fireEvent.input(textarea, { target: { value: 'Spoiler reply' } });
    await fireEvent.click(screen.getByLabelText('Contains spoiler'));

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);

    expect(createComment).toHaveBeenCalled();
    expect(createComment.mock.calls[0][0]).toMatchObject({
      parentCommentId: 'comment-1',
      containsSpoiler: true,
    });
  });
});
