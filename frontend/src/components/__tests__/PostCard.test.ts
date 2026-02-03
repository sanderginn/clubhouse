import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@testing-library/svelte';
import type { Post } from '../../stores/postStore';
import { authStore } from '../../stores';
import { sectionStore } from '../../stores/sectionStore';
import { tick } from 'svelte';

const loadThreadComments = vi.hoisted(() => vi.fn());
const loadMoreThreadComments = vi.hoisted(() => vi.fn());
const apiUpdatePost = vi.hoisted(() => vi.fn());
const apiUploadImage = vi.hoisted(() => vi.fn());
const apiDeletePost = vi.hoisted(() => vi.fn());

vi.mock('../../stores/commentFeedStore', () => ({
  loadThreadComments,
  loadMoreThreadComments,
}));

vi.mock('../../services/api', () => ({
  api: {
    updatePost: apiUpdatePost,
    uploadImage: apiUploadImage,
    deletePost: apiDeletePost,
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
  sectionStore.setSections([]);
});

afterEach(() => {
  cleanup();
  apiUpdatePost.mockReset();
  apiUploadImage.mockReset();
  apiDeletePost.mockReset();
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

  it('renders soundcloud embed when embed metadata present', () => {
    const postWithEmbed: Post = {
      ...basePost,
      links: [
        {
          url: 'https://soundcloud.com/artist/track',
          metadata: {
            url: 'https://soundcloud.com/artist/track',
            provider: 'soundcloud',
            title: 'Track Title',
            embed: {
              provider: 'soundcloud',
              embedUrl: 'https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/tracks/1',
              height: 166,
            },
          },
        },
      ],
    };

    render(PostCard, { post: postWithEmbed });
    const iframe = screen.getByTestId('soundcloud-embed');
    expect(iframe).toHaveAttribute(
      'src',
      'https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/tracks/1'
    );
    expect(iframe).toHaveAttribute('height', '166');
  });

  it('renders YouTube embed when metadata includes embed data', () => {
    const postWithEmbed: Post = {
      ...basePost,
      links: [
        {
          url: 'https://www.youtube.com/watch?v=dQw4w9WgXcQ',
          metadata: {
            url: 'https://www.youtube.com/watch?v=dQw4w9WgXcQ',
            title: 'Video',
            embed: {
              provider: 'youtube',
              embedUrl: 'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ',
            },
          },
        },
      ],
    };

    render(PostCard, { post: postWithEmbed });
    expect(screen.getByTestId('youtube-embed-frame')).toBeInTheDocument();
  });

  it('renders recipe card when recipe metadata present', () => {
    const postWithRecipe: Post = {
      ...basePost,
      links: [
        {
          url: 'https://example.com/recipe',
          metadata: {
            url: 'https://example.com/recipe',
            title: 'Example Recipe',
            image: 'https://example.com/recipe.jpg',
            recipe: {
              name: 'Tomato Soup',
              prep_time: '10m',
              cook_time: '20m',
              yield: '2',
              ingredients: ['Tomatoes', 'Salt'],
              instructions: ['Simmer', 'Serve'],
            },
          },
        },
      ],
    };

    render(PostCard, { post: postWithRecipe });
    expect(screen.getByText('Tomato Soup')).toBeInTheDocument();
    expect(screen.getByText('View Recipe')).toBeInTheDocument();
  });

  it('renders highlights when link includes them', () => {
    const postWithHighlights: Post = {
      ...basePost,
      links: [
        {
          url: 'https://example.com',
          metadata: {
            url: 'https://example.com',
            title: 'Example',
          },
          highlights: [{ timestamp: 75, label: 'Intro' }],
        },
      ],
    };

    render(PostCard, { post: postWithHighlights });
    expect(screen.getByText('01:15')).toBeInTheDocument();
    expect(screen.getByText('Intro')).toBeInTheDocument();
  });

  it('renders spotify embed when embed metadata is present', () => {
    const postWithSpotify: Post = {
      ...basePost,
      links: [
        {
          url: 'https://open.spotify.com/track/3n3Ppam7vgaVa1iaRUc9Lp',
          metadata: {
            url: 'https://open.spotify.com/track/3n3Ppam7vgaVa1iaRUc9Lp',
            embed: {
              embedUrl: 'https://open.spotify.com/embed/track/3n3Ppam7vgaVa1iaRUc9Lp',
              height: 152,
              provider: 'spotify',
            },
          },
        },
      ],
    };

    render(PostCard, { post: postWithSpotify });
    expect(screen.getByTitle('Spotify track')).toBeInTheDocument();
  });

  it('shows a posted in label when section pill label is enabled', () => {
    sectionStore.setSections([
      {
        id: 'section-1',
        name: 'General',
        type: 'general',
        icon: '💬',
        slug: 'general',
      },
    ]);

    render(PostCard, { post: basePost, showSectionPill: true, showSectionLabel: true });

    expect(screen.getByText('Posted in:')).toBeInTheDocument();
    expect(screen.getByText('General')).toBeInTheDocument();
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
    expect(screen.getByRole('button', { name: 'Reply to image' })).toBeInTheDocument();
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

    expect(screen.getByText('/api/v1/uploads/user-1/photo.png')).toBeInTheDocument();
  });

  it('renders edit form when edit button clicked', async () => {
    const postWithLink: Post = {
      ...basePost,
      userId: 'user-1',
    };

    authStore.setUser({ id: 'user-1', username: 'Sander', email: 'sander@test.com' });
    render(PostCard, { post: postWithLink });

    const editButton = screen.getByRole('button', { name: 'Edit' });
    await fireEvent.click(editButton);

    expect(screen.getByRole('textbox', { name: 'Edit post content' })).toBeInTheDocument();
  });

  it('submits edit form', async () => {
    const postWithLink: Post = {
      ...basePost,
      userId: 'user-1',
    };

    authStore.setUser({ id: 'user-1', username: 'Sander', email: 'sander@test.com' });
    render(PostCard, { post: postWithLink });

    const editButton = screen.getByRole('button', { name: 'Edit' });
    await fireEvent.click(editButton);

    const textarea = screen.getByRole('textbox', { name: 'Edit post content' });
    await fireEvent.input(textarea, { target: { value: 'Updated content' } });

    const saveButton = screen.getByRole('button', { name: 'Save' });
    await fireEvent.click(saveButton);

    await tick();

    expect(apiUpdatePost).toHaveBeenCalledWith(
      'post-1',
      expect.objectContaining({ content: 'Updated content' })
    );
  });
});
