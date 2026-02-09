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

type MovieMetadataForTest = {
  title: string;
  overview?: string;
  poster?: string;
  runtime?: number;
  genres?: string[];
  release_date?: string;
  director?: string;
  tmdb_rating?: number;
  trailer_key?: string;
  cast?: Array<{ name: string; character: string }>;
};

type LinkMetadataWithMovieForTest = NonNullable<
  NonNullable<Post['links']>[number]['metadata']
> & {
  movie?: MovieMetadataForTest;
};

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

  it('renders movie stats bar for movie sections', () => {
    sectionStore.setSections([
      {
        id: 'section-1',
        name: 'Movies',
        type: 'movie',
        icon: '🎬',
        slug: 'movies',
      },
    ]);

    const postWithMovieStats: Post = {
      ...basePost,
      movieStats: {
        watchlistCount: 5,
        watchCount: 3,
        averageRating: 4.5,
        viewerWatchlisted: true,
        viewerWatched: false,
        viewerRating: null,
        viewerCategories: ['Favorites'],
      },
    };

    render(PostCard, { post: postWithMovieStats });
    expect(screen.getByTestId('movie-stats-bar')).toBeInTheDocument();
  });

  it('renders movie card for movie metadata links in movie sections', () => {
    sectionStore.setSections([
      {
        id: 'section-1',
        name: 'Movies',
        type: 'movie',
        icon: '🎬',
        slug: 'movies',
      },
    ]);

    const metadataWithMovie: LinkMetadataWithMovieForTest = {
      url: 'https://www.imdb.com/title/tt0816692/',
      provider: 'imdb',
      title: 'Interstellar',
      movie: {
        title: 'Interstellar',
        overview: "A team travels through a wormhole to save humanity's future.",
        runtime: 169,
        genres: ['Sci-Fi', 'Drama'],
        release_date: '2014-11-07',
        director: 'Christopher Nolan',
        tmdb_rating: 8.6,
      },
    };

    const moviePost: Post = {
      ...basePost,
      links: [
        {
          url: 'https://www.imdb.com/title/tt0816692/',
          metadata: metadataWithMovie,
        },
      ],
    };

    render(PostCard, { post: moviePost });
    expect(screen.getByTestId('movie-card')).toBeInTheDocument();
    expect(screen.getByTestId('movie-title')).toHaveTextContent('Interstellar');
  });

  it('does not render movie components for non-movie sections', () => {
    sectionStore.setSections([
      {
        id: 'section-1',
        name: 'General',
        type: 'general',
        icon: '💬',
        slug: 'general',
      },
    ]);

    const metadataWithMovie: LinkMetadataWithMovieForTest = {
      url: 'https://www.imdb.com/title/tt0816692/',
      provider: 'imdb',
      title: 'Interstellar',
      movie: {
        title: 'Interstellar',
      },
    };

    const generalPostWithMovieData: Post = {
      ...basePost,
      links: [
        {
          url: 'https://www.imdb.com/title/tt0816692/',
          metadata: metadataWithMovie,
        },
      ],
      movieStats: {
        watchlistCount: 5,
        watchCount: 2,
        averageRating: 4,
      },
    };

    render(PostCard, { post: generalPostWithMovieData });

    expect(screen.queryByTestId('movie-card')).not.toBeInTheDocument();
    expect(screen.queryByTestId('movie-stats-bar')).not.toBeInTheDocument();
  });

  it('keeps recipe section rendering intact', () => {
    sectionStore.setSections([
      {
        id: 'section-1',
        name: 'Recipes',
        type: 'recipe',
        icon: '🍳',
        slug: 'recipes',
      },
    ]);

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
      recipeStats: {
        saveCount: 12,
        cookCount: 6,
        averageRating: 4.8,
      },
    };

    render(PostCard, { post: postWithRecipe });
    expect(screen.getByText('Tomato Soup')).toBeInTheDocument();
    expect(screen.getByTestId('recipe-stats-bar')).toBeInTheDocument();
    expect(screen.queryByTestId('movie-card')).not.toBeInTheDocument();
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
    expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
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
    expect(screen.queryByRole('button', { name: 'Delete' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Share' })).toBeInTheDocument();
  });

  it('shows delete action for admins on others posts', () => {
    authStore.setUser({
      id: 'admin-1',
      username: 'Admin',
      email: 'admin@example.com',
      isAdmin: true,
      totpEnabled: false,
    });

    render(PostCard, { post: basePost });
    expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument();
  });

  it('removes only the selected image link when editing', async () => {
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
    await fireEvent.click(screen.getByRole('button', { name: 'Remove image 2' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(apiUpdatePost).toHaveBeenCalledWith('post-1', {
      content: 'Hello world',
      links: [
        { url: 'https://cdn.example.com/uploads/first.png' },
        { url: 'https://example.com/article' },
        { url: 'https://example.com/extra' },
      ],
      removeLinkMetadata: false,
      mentionUsernames: [],
    });
  });

  it('removes link preview when editing', async () => {
    authStore.setUser({
      id: 'user-1',
      username: 'Sander',
      email: 'sander@example.com',
      isAdmin: false,
      totpEnabled: false,
    });

    const postWithLink: Post = {
      ...basePost,
      links: [
        {
          url: 'https://example.com',
          metadata: {
            url: 'https://example.com',
            title: 'Example',
            description: 'Desc',
          },
        },
      ],
    };

    apiUpdatePost.mockResolvedValue({
      post: { ...postWithLink, links: [] },
    });

    render(PostCard, { post: postWithLink });

    await fireEvent.click(screen.getByRole('button', { name: 'Edit' }));
    await fireEvent.click(screen.getByLabelText('Remove link'));
    await fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(apiUpdatePost).toHaveBeenCalledWith('post-1', {
      content: 'Hello world',
      links: undefined,
      removeLinkMetadata: true,
      mentionUsernames: [],
    });
  });

  it('replaces only the selected image link when editing', async () => {
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

    render(PostCard, { post: postWithImages });

    await fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

    const hiddenInput = screen.getByLabelText('Upload replacement for image 2') as HTMLInputElement;
    const file = new File(['image-bytes'], 'new.png', { type: 'image/png' });
    await fireEvent.change(hiddenInput, { target: { files: [file] } });
    await tick();
    await Promise.resolve();

    await fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(apiUpdatePost).toHaveBeenCalledWith('post-1', {
      content: 'Hello world',
      links: [
        { url: 'https://cdn.example.com/uploads/first.png' },
        { url: 'https://example.com/article' },
        { url: 'https://cdn.example.com/uploads/new.png' },
      ],
      removeLinkMetadata: false,
      mentionUsernames: [],
    });
  });

  it('saves edits with cmd+enter', async () => {
    authStore.setUser({
      id: 'user-1',
      username: 'Sander',
      email: 'sander@example.com',
      isAdmin: false,
      totpEnabled: false,
    });

    apiUpdatePost.mockResolvedValue({ post: { ...basePost } });

    render(PostCard, { post: basePost });

    await fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

    const textareas = screen.getAllByRole('textbox');
    const textarea = textareas.find((area) => area.getAttribute('rows') === '4');
    if (!textarea) {
      throw new Error('Edit textarea not found');
    }
    await fireEvent.keyDown(textarea, { key: 'Enter', metaKey: true });

    expect(apiUpdatePost).toHaveBeenCalledWith('post-1', {
      content: 'Hello world',
      links: undefined,
      removeLinkMetadata: false,
      mentionUsernames: [],
    });
  });

  it('ignores cmd+enter when edit content is empty', async () => {
    authStore.setUser({
      id: 'user-1',
      username: 'Sander',
      email: 'sander@example.com',
      isAdmin: false,
      totpEnabled: false,
    });

    apiUpdatePost.mockResolvedValue({ post: { ...basePost } });

    render(PostCard, { post: basePost });

    await fireEvent.click(screen.getByRole('button', { name: 'Edit' }));

    const textareas = screen.getAllByRole('textbox');
    const textarea = textareas.find((area) => area.getAttribute('rows') === '4');
    if (!textarea) {
      throw new Error('Edit textarea not found');
    }
    await fireEvent.input(textarea, { target: { value: '   ' } });
    await fireEvent.keyDown(textarea, { key: 'Enter', metaKey: true });

    expect(apiUpdatePost).not.toHaveBeenCalled();
  });
});
