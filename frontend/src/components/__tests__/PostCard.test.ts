import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@testing-library/svelte';
import type { Post } from '../../stores/postStore';
import { authStore } from '../../stores';
import { tick } from 'svelte';

const loadThreadComments = vi.hoisted(() => vi.fn());
const loadMoreThreadComments = vi.hoisted(() => vi.fn());
const apiUpdatePost = vi.hoisted(() => vi.fn());
const apiUploadImage = vi.hoisted(() => vi.fn());

vi.mock('../../stores/commentFeedStore', () => ({
  loadThreadComments,
  loadMoreThreadComments,
}));

vi.mock('../../services/api', () => ({
  api: {
    updatePost: apiUpdatePost,
    uploadImage: apiUploadImage,
  },
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
  apiUpdatePost.mockReset();
  apiUploadImage.mockReset();
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

  it('hides internal upload URLs from post content when an image is rendered', () => {
    const postWithInternalImage: Post = {
      ...basePost,
      content: 'Look /api/v1/uploads/user-1/photo.png',
      links: [
        {
          url: '/api/v1/uploads/user-1/photo.png',
        },
      ],
    };

    render(PostCard, { post: postWithInternalImage });

    expect(screen.getByRole('img', { name: 'Uploaded image' })).toBeInTheDocument();
    expect(screen.getByText('Look')).toBeInTheDocument();
    expect(screen.queryByText('/api/v1/uploads/user-1/photo.png')).not.toBeInTheDocument();
  });

  it('shows internal upload link when image fails to load', async () => {
    const postWithInternalImage: Post = {
      ...basePost,
      links: [
        {
          url: '/api/v1/uploads/user-1/photo.png',
        },
      ],
    };

    render(PostCard, { post: postWithInternalImage });

    const image = screen.getByRole('img', { name: 'Uploaded image' });
    await fireEvent.error(image);

    const link = screen.getByRole('link', {
      name: /\/api\/v1\/uploads\/user-1\/photo\.png/,
    });
    expect(link).toHaveAttribute('href', '/api/v1/uploads/user-1/photo.png');
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
      totpEnabled: false,
    });

    render(PostCard, { post: basePost });
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Share' })).toBeInTheDocument();
  });

  it('hides edit action for other users', () => {
    authStore.setUser({
      id: 'user-2',
      username: 'Other',
      email: 'other@example.com',
      isAdmin: false,
      totpEnabled: false,
    });

    render(PostCard, { post: basePost });
    expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Share' })).toBeInTheDocument();
  });

  it('removes only the first image link when editing', async () => {
    authStore.setUser({
      id: 'user-1',
      username: 'Sander',
      email: 'sander@example.com',
      isAdmin: false,
      totpEnabled: false,
    });

    const postWithImages: Post = {
      ...basePost,
      links: [
        { url: 'https://cdn.example.com/uploads/first.png' },
        { url: 'https://example.com/article' },
        { url: 'https://cdn.example.com/uploads/second.png' },
        { url: 'https://example.com/extra' },
      ],
    };

    apiUpdatePost.mockResolvedValue({
      post: { ...postWithImages, content: postWithImages.content },
    });

    render(PostCard, { post: postWithImages });

    await fireEvent.click(screen.getByRole('button', { name: 'Edit' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Remove image' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(apiUpdatePost).toHaveBeenCalledWith('post-1', {
      content: 'Hello world',
      links: [
        { url: 'https://example.com/article' },
        { url: 'https://cdn.example.com/uploads/second.png' },
        { url: 'https://example.com/extra' },
      ],
    });
  });

  it('replaces only the first image link when editing', async () => {
    authStore.setUser({
      id: 'user-1',
      username: 'Sander',
      email: 'sander@example.com',
      isAdmin: false,
      totpEnabled: false,
    });

    const postWithImages: Post = {
      ...basePost,
      links: [
        { url: 'https://cdn.example.com/uploads/first.png' },
        { url: 'https://example.com/article' },
        { url: 'https://cdn.example.com/uploads/second.png' },
      ],
    };

    apiUploadImage.mockResolvedValue({ url: 'https://cdn.example.com/uploads/new.png' });
    apiUpdatePost.mockResolvedValue({
      post: { ...postWithImages, content: postWithImages.content },
    });

    const { container } = render(PostCard, { post: postWithImages });

    await fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

    await fireEvent.click(screen.getByRole('button', { name: 'Replace image' }));

    const hiddenInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File(['image-bytes'], 'new.png', { type: 'image/png' });
    await fireEvent.change(hiddenInput, { target: { files: [file] } });
    await tick();
    await Promise.resolve();

    await fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(apiUpdatePost).toHaveBeenCalledWith('post-1', {
      content: 'Hello world',
      links: [
        { url: 'https://cdn.example.com/uploads/new.png' },
        { url: 'https://example.com/article' },
        { url: 'https://cdn.example.com/uploads/second.png' },
      ],
    });
  });
});
