import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { WatchLog, WatchlistCategory, WatchlistItem } from '../../stores/movieStore';
import type { ApiPost } from '../../stores/postMapper';
import { mapApiPost } from '../../stores/postMapper';
import { movieStore } from '../../stores/movieStore';
import { sectionStore } from '../../stores/sectionStore';
import { api } from '../../services/api';

const pushPath = vi.fn();

vi.mock('../../services/routeNavigation', () => ({
  buildStandaloneThreadHref: (postId: string) => `/posts/${postId}`,
  pushPath,
}));

const { default: Watchlist } = await import('../movies/Watchlist.svelte');

const apiPostOne: ApiPost = {
  id: 'post-1',
  user_id: 'user-1',
  section_id: 'section-movie',
  content: 'The Matrix',
  created_at: '2026-01-02T00:00:00Z',
  links: [
    {
      url: 'https://example.com/matrix',
      metadata: {
        title: 'The Matrix',
        image: 'https://example.com/matrix.jpg',
        movie: {
          title: 'The Matrix',
          poster: 'https://example.com/matrix.jpg',
        },
      },
    },
  ],
  movie_stats: {
    avg_rating: 4.9,
    watch_count: 14,
    watchlist_count: 6,
  },
};

const apiPostTwo: ApiPost = {
  id: 'post-2',
  user_id: 'user-2',
  section_id: 'section-movie',
  content: 'Alien',
  created_at: '2026-01-03T00:00:00Z',
  links: [
    {
      url: 'https://example.com/alien',
      metadata: {
        title: 'Alien',
        image: 'https://example.com/alien.jpg',
        movie: {
          title: 'Alien',
          poster: 'https://example.com/alien.jpg',
        },
      },
    },
  ],
  movie_stats: {
    avg_rating: 3.8,
    watch_count: 22,
    watchlist_count: 3,
  },
};

const apiPostThree: ApiPost = {
  id: 'post-3',
  user_id: 'user-3',
  section_id: 'section-series',
  content: 'Severance',
  created_at: '2026-01-04T00:00:00Z',
  links: [
    {
      url: 'https://example.com/severance',
      metadata: {
        title: 'Severance',
        image: 'https://example.com/severance.jpg',
        movie: {
          title: 'Severance',
          poster: 'https://example.com/severance.jpg',
        },
      },
    },
  ],
  movie_stats: {
    avg_rating: 4.5,
    watch_count: 8,
    watchlist_count: 11,
  },
};

const apiPostFive: ApiPost = {
  id: 'post-5',
  user_id: 'user-5',
  section_id: 'section-movie',
  content: 'Terminator 2',
  created_at: '2026-01-06T00:00:00Z',
  links: [
    {
      url: 'https://example.com/terminator2',
      metadata: {
        title: 'Terminator 2',
        image: 'https://example.com/terminator2.jpg',
        movie: {
          title: 'Terminator 2',
          poster: 'https://example.com/terminator2.jpg',
        },
      },
    },
  ],
  movie_stats: {
    avg_rating: 4.6,
    watch_count: 2,
    watchlist_count: 9,
  },
};

const apiNonMoviePost: ApiPost = {
  id: 'post-4',
  user_id: 'user-4',
  section_id: 'section-general',
  content: 'General update',
  created_at: '2026-01-05T00:00:00Z',
  links: [
    {
      url: 'https://example.com/general',
      metadata: {
        title: 'General update',
      },
    },
  ],
};

const postOne = mapApiPost(apiPostOne);
const postTwo = mapApiPost(apiPostTwo);
const postThree = mapApiPost(apiPostThree);
const postFive = mapApiPost(apiPostFive);
const nonMoviePost = mapApiPost(apiNonMoviePost);

const categories: WatchlistCategory[] = [
  { id: 'cat-1', name: 'Favorites', position: 1 },
  { id: 'cat-2', name: 'Horror', position: 2 },
];

const watchlistMap = new Map<string, WatchlistItem[]>([
  [
    'Favorites',
    [
      {
        id: 'watch-1',
        userId: 'user-1',
        postId: 'post-1',
        category: 'Favorites',
        createdAt: '2026-01-05T00:00:00Z',
        post: postOne,
      },
      {
        id: 'watch-3',
        userId: 'user-1',
        postId: 'post-3',
        category: 'Favorites',
        createdAt: '2026-01-07T00:00:00Z',
        post: postThree,
      },
    ],
  ],
  [
    'Horror',
    [
      {
        id: 'watch-2',
        userId: 'user-1',
        postId: 'post-2',
        category: 'Horror',
        createdAt: '2026-01-06T00:00:00Z',
        post: postTwo,
      },
    ],
  ],
]);

const watchLogs: WatchLog[] = [
  {
    id: 'log-1',
    userId: 'user-1',
    postId: 'post-1',
    rating: 5,
    watchedAt: '2026-01-07T00:00:00Z',
    post: postOne,
  },
];

beforeEach(() => {
  movieStore.reset();
  sectionStore.setSections([
    { id: 'section-general', name: 'General', type: 'general', icon: '💬', slug: 'general' },
    { id: 'section-movie', name: 'Movies', type: 'movie', icon: '🎬', slug: 'movies' },
    { id: 'section-series', name: 'Series', type: 'series', icon: '📺', slug: 'series' },
  ]);

  movieStore.setCategories([...categories]);
  movieStore.setWatchlist(new Map(watchlistMap));
  movieStore.setWatchLogs([...watchLogs]);

  vi.spyOn(api, 'getMoviePosts').mockImplementation(
    async (_limit?: number, cursor?: string, sectionType?: 'movie' | 'series') => {
      if (sectionType === 'series') {
        return {
          posts: [postThree],
          hasMore: false,
        };
      }

      if (cursor === 'cursor-page-2') {
        return {
          posts: [postFive],
          hasMore: false,
        };
      }
      return {
        posts: [postOne, postTwo, nonMoviePost],
        hasMore: true,
        nextCursor: 'cursor-page-2',
      };
    }
  );

  vi.spyOn(movieStore, 'loadWatchlistCategories').mockResolvedValue();
  vi.spyOn(movieStore, 'loadWatchlist').mockResolvedValue();
  vi.spyOn(movieStore, 'loadWatchLogs').mockResolvedValue();
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  pushPath.mockReset();
  movieStore.reset();
});

describe('Watchlist', () => {
  it('toggles collapsed and expanded states from the header button', async () => {
    render(Watchlist);

    const toggle = screen.getByTestId('watchlist-collapse');
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(toggle).toHaveTextContent('Collapse');
    expect(screen.getByTestId('watchlist-category-panel')).toBeInTheDocument();

    await fireEvent.click(toggle);

    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(toggle).toHaveTextContent('Expand');
    expect(screen.queryByTestId('watchlist-category-panel')).not.toBeInTheDocument();
    expect(screen.getByTestId('watchlist-tab-my')).toBeInTheDocument();

    await fireEvent.click(toggle);

    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(toggle).toHaveTextContent('Collapse');
    expect(screen.getByTestId('watchlist-category-panel')).toBeInTheDocument();
  });

  it('supports collapse behavior in series mode', async () => {
    render(Watchlist, { sectionType: 'series' });

    const toggle = screen.getByTestId('watchlist-collapse');
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByRole('tab', { name: 'All Series' })).toBeInTheDocument();
    expect(screen.getByTestId('watchlist-category-panel')).toBeInTheDocument();

    await fireEvent.click(toggle);

    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByTestId('watchlist-category-panel')).not.toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'All Series' })).toBeInTheDocument();
  });

  it('renders My List tab by default with saved movies', () => {
    render(Watchlist);

    expect(screen.getByTestId('watchlist-tab-my')).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByTestId('watchlist-my-item-post-1')).toBeInTheDocument();
    expect(screen.getByTestId('watchlist-my-item-post-2')).toBeInTheDocument();
    expect(screen.queryByTestId('watchlist-my-item-post-3')).not.toBeInTheDocument();
    expect(screen.getByTestId('watchlist-watched-post-1')).toBeInTheDocument();
  });

  it('loads All Movies from API, paginates, and supports sorting', async () => {
    render(Watchlist);

    await fireEvent.click(screen.getByTestId('watchlist-tab-all'));

    let items = await screen.findAllByTestId(/watchlist-all-item-/);
    expect(items).toHaveLength(2);
    expect(screen.queryByTestId('watchlist-all-item-post-4')).not.toBeInTheDocument();
    expect(api.getMoviePosts).toHaveBeenNthCalledWith(1, 20, undefined, 'movie');
    expect(items[0]).toHaveTextContent('The Matrix');

    await fireEvent.change(screen.getByTestId('watchlist-sort'), {
      target: { value: 'watch_count' },
    });

    items = screen.getAllByTestId(/watchlist-all-item-/);
    expect(items[0]).toHaveTextContent('Alien');

    await fireEvent.click(screen.getByTestId('watchlist-all-load-more'));
    await screen.findByTestId('watchlist-all-item-post-5');
    expect(api.getMoviePosts).toHaveBeenNthCalledWith(2, 20, 'cursor-page-2', 'movie');

    await fireEvent.change(screen.getByTestId('watchlist-sort'), {
      target: { value: 'watchlist_count' },
    });

    items = screen.getAllByTestId(/watchlist-all-item-/);
    expect(items[0]).toHaveTextContent('Terminator 2');

    await fireEvent.input(screen.getByTestId('watchlist-search'), {
      target: { value: 'terminator' },
    });
    await waitFor(() => {
      expect(screen.getAllByTestId(/watchlist-all-item-/)).toHaveLength(1);
    });
    items = screen.getAllByTestId(/watchlist-all-item-/);
    expect(items[0]).toHaveTextContent('Terminator 2');
  });

  it('renders series-specific labels and filters in series context', async () => {
    render(Watchlist, { sectionType: 'series' });

    expect(screen.getByRole('tab', { name: 'All Series' })).toBeInTheDocument();
    expect(screen.getByText('1 series')).toBeInTheDocument();
    expect(screen.getByTestId('watchlist-my-item-post-3')).toBeInTheDocument();
    expect(screen.queryByTestId('watchlist-my-item-post-1')).not.toBeInTheDocument();

    await fireEvent.click(screen.getByTestId('watchlist-tab-all'));
    expect(api.getMoviePosts).toHaveBeenNthCalledWith(1, 20, undefined, 'series');
    expect(await screen.findByTestId('watchlist-all-item-post-3')).toBeInTheDocument();
  });

  it('does not auto-retry movie feed after failure without user action', async () => {
    vi.mocked(api.getMoviePosts).mockRejectedValueOnce(new Error('Movie feed failed'));

    render(Watchlist);
    await fireEvent.click(screen.getByTestId('watchlist-tab-all'));

    await screen.findByTestId('watchlist-all-error');
    await waitFor(() => {
      expect(api.getMoviePosts).toHaveBeenCalledTimes(1);
    });

    await Promise.resolve();
    await Promise.resolve();
    expect(api.getMoviePosts).toHaveBeenCalledTimes(1);

    await fireEvent.click(screen.getByTestId('watchlist-all-retry'));
    await waitFor(() => {
      expect(api.getMoviePosts).toHaveBeenCalledTimes(2);
    });
  });

  it('filters My List by selected category', async () => {
    render(Watchlist);

    await fireEvent.click(screen.getByTestId('watchlist-category-Horror'));

    expect(screen.getByTestId('watchlist-my-item-post-2')).toBeInTheDocument();
    expect(screen.queryByTestId('watchlist-my-item-post-1')).not.toBeInTheDocument();
  });

  it('supports inline category rename', async () => {
    const updateSpy = vi.spyOn(movieStore, 'updateCategory').mockResolvedValue();

    render(Watchlist);

    await fireEvent.click(screen.getByTestId('watchlist-category-edit-cat-1'));
    const editInput = await screen.findByTestId('watchlist-category-edit-input');
    expect(editInput).toHaveValue('Favorites');

    await fireEvent.input(editInput, {
      target: { value: 'Top Picks' },
    });
    await fireEvent.click(await screen.findByTestId('watchlist-category-edit-save'));

    await waitFor(() => {
      expect(updateSpy).toHaveBeenCalledWith('cat-1', { name: 'Top Picks' });
    });
  });

  it('shows a confirmation prompt before deleting category', async () => {
    const deleteSpy = vi.spyOn(movieStore, 'deleteCategory').mockResolvedValue();

    render(Watchlist);

    await fireEvent.click(screen.getByTestId('watchlist-category-delete-cat-1'));
    expect(await screen.findByTestId('watchlist-category-delete-confirm')).toBeInTheDocument();

    await fireEvent.click(await screen.findByTestId('watchlist-category-delete-confirm-button'));

    await waitFor(() => {
      expect(deleteSpy).toHaveBeenCalledWith('cat-1');
    });
  });

  it('shows loading states while category actions are in progress', async () => {
    let resolveRename: (() => void) | null = null;
    let resolveDelete: (() => void) | null = null;

    vi.spyOn(movieStore, 'updateCategory').mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolveRename = resolve;
        })
    );
    vi.spyOn(movieStore, 'deleteCategory').mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolveDelete = resolve;
        })
    );

    render(Watchlist);

    await fireEvent.click(screen.getByTestId('watchlist-category-edit-cat-1'));
    await fireEvent.input(await screen.findByTestId('watchlist-category-edit-input'), {
      target: { value: 'Top Picks' },
    });
    await fireEvent.click(await screen.findByTestId('watchlist-category-edit-save'));
    const renameSaveButton = await screen.findByTestId('watchlist-category-edit-save');
    expect(renameSaveButton).toBeDisabled();
    expect(renameSaveButton).toHaveTextContent('Saving...');

    resolveRename?.();
    await waitFor(() => {
      expect(screen.queryByTestId('watchlist-category-edit-input')).not.toBeInTheDocument();
    });

    await fireEvent.click(screen.getByTestId('watchlist-category-delete-cat-1'));
    await fireEvent.click(await screen.findByTestId('watchlist-category-delete-confirm-button'));
    const deleteConfirmButton = await screen.findByTestId(
      'watchlist-category-delete-confirm-button'
    );
    expect(deleteConfirmButton).toBeDisabled();
    expect(deleteConfirmButton).toHaveTextContent('Deleting...');

    resolveDelete?.();
    await waitFor(() => {
      expect(screen.queryByTestId('watchlist-category-delete-confirm')).not.toBeInTheDocument();
    });
  });

  it('shows category action error when rename fails', async () => {
    vi.spyOn(movieStore, 'updateCategory').mockImplementation(async () => {
      movieStore.setError('Rename failed');
    });

    render(Watchlist);

    await fireEvent.click(screen.getByTestId('watchlist-category-edit-cat-1'));
    await fireEvent.input(await screen.findByTestId('watchlist-category-edit-input'), {
      target: { value: 'Top Picks' },
    });
    await fireEvent.click(await screen.findByTestId('watchlist-category-edit-save'));

    await waitFor(() => {
      expect(screen.getByTestId('watchlist-category-error')).toHaveTextContent('Rename failed');
    });
  });

  it('shows empty states for no saved movies and empty selected category', async () => {
    movieStore.reset();

    const { unmount } = render(Watchlist);
    expect(screen.getByText('No movies saved yet')).toBeInTheDocument();

    unmount();

    movieStore.setCategories([
      { id: 'cat-1', name: 'Favorites', position: 1 },
      { id: 'cat-2', name: 'Classics', position: 2 },
    ]);
    movieStore.setWatchlist(
      new Map([
        [
          'Favorites',
          [
            {
              id: 'watch-1',
              userId: 'user-1',
              postId: 'post-1',
              category: 'Favorites',
              createdAt: '2026-01-05T00:00:00Z',
              post: postOne,
            },
          ],
        ],
      ])
    );

    render(Watchlist);

    await fireEvent.click(screen.getByTestId('watchlist-category-Classics'));
    expect(screen.getByText('No movies in this category')).toBeInTheDocument();
  });

  it('shows series-specific empty copy in series context', () => {
    movieStore.reset();

    render(Watchlist, { sectionType: 'series' });

    expect(screen.getByText('No series saved yet')).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'All Series' })).toBeInTheDocument();
  });

  it('navigates to the post when a movie card is clicked', async () => {
    render(Watchlist);

    await fireEvent.click(screen.getByTestId('watchlist-my-item-post-1'));

    expect(pushPath).toHaveBeenCalledWith('/posts/post-1');
  });
});
