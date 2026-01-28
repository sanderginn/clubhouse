import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';

vi.mock('../../../services/api', () => ({
  api: {
    getPostReactions: vi.fn().mockResolvedValue({
      reactions: [
        {
          emoji: '👍',
          users: [
            { id: 'user-1', username: 'sander', profile_picture_url: null },
          ],
        },
      ],
    }),
    getCommentReactions: vi.fn().mockResolvedValue({ reactions: [] }),
  },
}));

const { default: ReactionBar } = await import('../ReactionBar.svelte');

afterEach(() => {
  cleanup();
});

describe('ReactionBar', () => {
  it('renders reactions and toggles when clicked', async () => {
    const onToggle = vi.fn();
    render(ReactionBar, {
      reactionCounts: { '👍': 2 },
      userReactions: new Set<string>(),
      onToggle,
    });

    const reactionButton = screen.getByText('👍');
    await fireEvent.click(reactionButton);

    expect(onToggle).toHaveBeenCalledWith('👍');
  });

  it('opens emoji picker and selects emoji', async () => {
    const onToggle = vi.fn();
    render(ReactionBar, {
      reactionCounts: {},
      userReactions: new Set<string>(),
      onToggle,
    });

    const addButton = screen.getByText('React');
    await fireEvent.click(addButton);

    const emojiButton = screen.getByLabelText('React with 👍');
    await fireEvent.click(emojiButton);

    expect(onToggle).toHaveBeenCalledWith('👍');
  });

  it('loads tooltip reactions on hover', async () => {
    vi.useFakeTimers();
    const onToggle = vi.fn();
    render(ReactionBar, {
      reactionCounts: { '👍': 1 },
      userReactions: new Set<string>(),
      onToggle,
      postId: 'post-1',
    });

    const tooltipTarget = screen.getByRole('group');
    await fireEvent.mouseEnter(tooltipTarget);
    await vi.runAllTimersAsync();

    expect(await screen.findByText('sander')).toBeInTheDocument();
    vi.useRealTimers();
  });
});
