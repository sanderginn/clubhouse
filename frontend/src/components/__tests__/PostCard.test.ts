import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/svelte';
import type { Post } from '../../stores/postStore';
import { authStore } from '../../stores';

const loadThreadComments = vi.hoisted(() => vi.fn());
const loadMoreThreadComments = vi.hoisted(() => vi.fn());

vi.mock('../../stores/commentFeedStore', () => ({
  loadThreadComments,
  loadMoreThreadComments,
}));

const { default: PostCard } = await import('../PostCard.svelte');

const basePost: Post = {
  id: 'post-1',
  userId: 'user-1',
  sectionId: 'section-1',
  content: 'Hello world',
  createdAt: '2025-01-01T00:00:00Z',
  commentCount: 0,
  user: {
    id: 'user-1',
    username: 'Sander',
  },
};

beforeEach(() => {
  vi.clearAllMocks();
  authStore.setUser(null);
});

afterEach(() => {
  cleanup();
});

describe('PostCard', () => {
  it('shows comment thread by default', () => {
    render(PostCard, { post: basePost });

    expect(screen.getByText('No comments yet. Start the conversation.')).toBeInTheDocument();
  });

  it('renders rich link card when metadata present', () => {
    const postWithLink: Post = {
      ...basePost,
      links: [
        {
          url: 'https://example.com',
          metadata: {
            url: 'https://example.com',
            title: 'Example',
            description: 'Desc',
            provider: 'example',
          },
        },
      ],
    };

    render(PostCard, { post: postWithLink });
    expect(screen.getByText('Example')).toBeInTheDocument();
  });

  it('renders plain link when metadata missing', () => {
    const postWithLink: Post = {
      ...basePost,
      links: [
        {
          url: 'https://example.com',
        },
      ],
    };

    render(PostCard, { post: postWithLink });
    expect(screen.getByText('https://example.com')).toBeInTheDocument();
  });

  it('renders inline image when link points to an image', () => {
    const postWithImage: Post = {
      ...basePost,
      links: [
        {
          url: 'https://cdn.example.com/uploads/photo.png',
        },
      ],
    };

    render(PostCard, { post: postWithImage });
    expect(screen.getByRole('img', { name: 'Uploaded image' })).toBeInTheDocument();
  });

  it('shows avatar fallback when no profile image', () => {
    render(PostCard, { post: basePost });
    expect(screen.getByText('S')).toBeInTheDocument();
  });

  it('links to the author profile', () => {
    render(PostCard, { post: basePost });
    const link = screen.getByRole('link', { name: 'Sander' });
    expect(link).toHaveAttribute('href', '/users/user-1');
  });

  it('shows edit action for own post', async () => {
    authStore.setUser({
      id: 'user-1',
      username: 'Sander',
      email: 'sander@example.com',
      isAdmin: false,
    });

    render(PostCard, { post: basePost });
    expect(screen.getByRole('button', { name: 'Open post actions' })).toBeInTheDocument();
  });

  it('hides edit action for other users', () => {
    authStore.setUser({
      id: 'user-2',
      username: 'Other',
      email: 'other@example.com',
      isAdmin: false,
    });

    render(PostCard, { post: basePost });
    expect(screen.queryByRole('button', { name: 'Open post actions' })).not.toBeInTheDocument();
  });
});
