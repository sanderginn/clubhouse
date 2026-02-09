import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Post } from '../../stores/postStore';
import type { WatchLog, WatchlistCategory, WatchlistItem } from '../../stores/movieStore';
import { postStore } from '../../stores/postStore';
import { movieStore } from '../../stores/movieStore';

const pushPath = vi.fn();

vi.mock('../../services/routeNavigation', () => ({
  buildStandaloneThreadHref: (postId: string) => `/posts/${postId}`,
  pushPath,
}));

const { default: Watchlist } = await import('../movies/Watchlist.svelte');

type MovieStatsLike = {
  avgRating: number;
  watchCount: number;
  watchlistCount: number;
};

const postOne: Post & { movieStats: MovieStatsLike } = {
  id: 'post-1',
  userId: 'user-1',
  sectionId: 'section-movie',
  content: 'The Matrix',
  createdAt: '2026-01-02T00:00:00Z',
  links: [
    {
      url: 'https://example.com/matrix',
      metadata: {
        title: 'The Matrix',
        image: 'https://example.com/matrix.jpg',
      },
    },
  ],
  movieStats: {
    avgRating: 4.9,
    watchCount: 14,
    watchlistCount: 6,
  },
};

const postTwo: Post & { movieStats: MovieStatsLike } = {
  id: 'post-2',
  userId: 'user-2',
  sectionId: 'section-movie',
  content: 'Alien',
  createdAt: '2026-01-03T00:00:00Z',
  links: [
    {
      url: 'https://example.com/alien',
      metadata: {
        title: 'Alien',
        image: 'https://example.com/alien.jpg',
      },
    },
  ],
  movieStats: {
    avgRating: 3.8,
    watchCount: 22,
    watchlistCount: 3,
  },
};

const postThree: Post & { movieStats: MovieStatsLike } = {
  id: 'post-3',
  userId: 'user-3',
  sectionId: 'section-series',
  content: 'Severance',
  createdAt: '2026-01-04T00:00:00Z',
  links: [
    {
      url: 'https://example.com/severance',
      metadata: {
        title: 'Severance',
        image: 'https://example.com/severance.jpg',
      },
    },
  ],
  movieStats: {
    avgRating: 4.5,
    watchCount: 8,
    watchlistCount: 11,
  },
};

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
  postStore.reset();
  movieStore.reset();

  postStore.setPosts([postOne, postTwo, postThree], null, false);
  movieStore.setCategories([...categories]);
  movieStore.setWatchlist(new Map(watchlistMap));
  movieStore.setWatchLogs([...watchLogs]);

  vi.spyOn(movieStore, 'loadWatchlistCategories').mockResolvedValue();
  vi.spyOn(movieStore, 'loadWatchlist').mockResolvedValue();
  vi.spyOn(movieStore, 'loadWatchLogs').mockResolvedValue();
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  pushPath.mockReset();
  postStore.reset();
  movieStore.reset();
});

describe('Watchlist', () => {
  it('renders My List tab by default with saved movies', () => {
    render(Watchlist);

    expect(screen.getByTestId('watchlist-tab-my')).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByTestId('watchlist-my-item-post-1')).toBeInTheDocument();
    expect(screen.getByTestId('watchlist-my-item-post-2')).toBeInTheDocument();
    expect(screen.getByTestId('watchlist-watched-post-1')).toBeInTheDocument();
  });

  it('renders All Movies tab and supports sorting', async () => {
    render(Watchlist);

    await fireEvent.click(screen.getByTestId('watchlist-tab-all'));

    let items = screen.getAllByTestId(/watchlist-all-item-/);
    expect(items[0]).toHaveTextContent('The Matrix');

    await fireEvent.change(screen.getByTestId('watchlist-sort'), {
      target: { value: 'watch_count' },
    });

    items = screen.getAllByTestId(/watchlist-all-item-/);
    expect(items[0]).toHaveTextContent('Alien');

    await fireEvent.change(screen.getByTestId('watchlist-sort'), {
      target: { value: 'watchlist_count' },
    });

    items = screen.getAllByTestId(/watchlist-all-item-/);
    expect(items[0]).toHaveTextContent('Severance');
  });

  it('filters My List by selected category', async () => {
    render(Watchlist);

    await fireEvent.click(screen.getByTestId('watchlist-category-Horror'));

    expect(screen.getByTestId('watchlist-my-item-post-2')).toBeInTheDocument();
    expect(screen.queryByTestId('watchlist-my-item-post-1')).not.toBeInTheDocument();
  });

  it('shows empty states for no saved movies and empty selected category', async () => {
    movieStore.reset();
    postStore.setPosts([], null, false);

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

  it('navigates to the post when a movie card is clicked', async () => {
    render(Watchlist);

    await fireEvent.click(screen.getByTestId('watchlist-my-item-post-1'));

    expect(pushPath).toHaveBeenCalledWith('/posts/post-1');
  });
});
