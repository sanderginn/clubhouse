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
});
