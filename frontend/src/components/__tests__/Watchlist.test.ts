import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { WatchLog, WatchlistCategory, WatchlistItem } from '../../stores/movieStore';
import type { ApiPost } from '../../stores/postMapper';
import { mapApiPost } from '../../stores/postMapper';
import { postStore } from '../../stores/postStore';
import { movieStore } from '../../stores/movieStore';

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

  postStore.setPosts([postOne, postTwo, postThree, nonMoviePost], null, false);
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

  it('renders All Movies tab, excludes non-movie posts, and supports sorting', async () => {
    render(Watchlist);

    await fireEvent.click(screen.getByTestId('watchlist-tab-all'));

    let items = screen.getAllByTestId(/watchlist-all-item-/);
    expect(items).toHaveLength(3);
    expect(screen.queryByTestId('watchlist-all-item-post-4')).not.toBeInTheDocument();
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
