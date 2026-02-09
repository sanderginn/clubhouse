import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

const apiGetPostWatchlistInfo = vi.hoisted(() =>
  vi.fn().mockResolvedValue({
    saveCount: 2,
    users: [
      {
        id: 'user-1',
        username: 'sander',
        displayName: 'Sander',
        avatar: undefined,
      },
    ],
    viewerSaved: false,
    viewerCategories: [],
  })
);

const apiGetPostWatchLogs = vi.hoisted(() =>
  vi.fn().mockResolvedValue({
    watchCount: 1,
    avgRating: 4.2,
    logs: [
      {
        id: 'log-1',
        userId: 'user-2',
        postId: 'post-1',
        rating: 4.5,
        watchedAt: '2026-02-01T00:00:00Z',
        user: {
          id: 'user-2',
          username: 'alex',
          displayName: 'Alex',
          avatar: undefined,
        },
      },
    ],
    viewerWatched: false,
    viewerRating: undefined,
  })
);

vi.mock('../../services/api', () => ({
  api: {
    getPostWatchlistInfo: apiGetPostWatchlistInfo,
    getPostWatchLogs: apiGetPostWatchLogs,
  },
}));

vi.mock('../movies/WatchlistSaveButton.svelte', async () => ({
  default: (await import('./WatchlistSaveButtonPropsStub.svelte')).default,
}));

vi.mock('../movies/WatchButton.svelte', async () => ({
  default: (await import('./WatchButtonPropsStub.svelte')).default,
}));

const { default: MovieStatsBar } = await import('../movies/MovieStatsBar.svelte');

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('MovieStatsBar', () => {
  it('renders stats and action buttons', () => {
    render(MovieStatsBar, {
      postId: 'post-1',
      stats: {
        watchlistCount: 12,
        watchCount: 8,
        avgRating: 4.2,
        viewerWatchlisted: true,
        viewerWatched: true,
        viewerRating: 4,
        viewerCategories: ['Favorites'],
      },
    });

    expect(screen.getByTestId('movie-stats-bar')).toBeInTheDocument();
    expect(screen.getByText('12')).toBeInTheDocument();
    expect(screen.getByText('saved')).toBeInTheDocument();
    expect(screen.getByText('8')).toBeInTheDocument();
    expect(screen.getByText('watched')).toBeInTheDocument();
    expect(screen.getByTestId('movie-average-rating')).toHaveTextContent('4.2');
    expect(screen.getByTestId('watchlist-save-button-stub')).toBeInTheDocument();
    expect(screen.getByTestId('watch-button-stub')).toBeInTheDocument();
  });

  it('hides average rating when it is not provided', () => {
    render(MovieStatsBar, {
      postId: 'post-2',
      stats: {
        watchlistCount: 0,
        watchCount: 0,
        viewerWatchlisted: false,
        viewerWatched: false,
      },
    });

    expect(screen.queryByTestId('movie-average-rating')).not.toBeInTheDocument();
  });

  it('loads watchlist tooltip users on hover', async () => {
    vi.useFakeTimers();

    render(MovieStatsBar, {
      postId: 'post-3',
      stats: {
        watchlistCount: 2,
        watchCount: 0,
        viewerWatchlisted: false,
        viewerWatched: false,
      },
    });

    expect(apiGetPostWatchlistInfo).not.toHaveBeenCalled();

    await fireEvent.mouseEnter(screen.getByTestId('movie-watchlist-stat'));
    await vi.runAllTimersAsync();

    expect(apiGetPostWatchlistInfo).toHaveBeenCalledWith('post-3');
    expect(await screen.findByText('Sander')).toBeInTheDocument();

    vi.useRealTimers();
  });

  it('loads watch tooltip logs on hover', async () => {
    vi.useFakeTimers();

    render(MovieStatsBar, {
      postId: 'post-4',
      stats: {
        watchlistCount: 0,
        watchCount: 1,
        avgRating: 4.2,
        viewerWatchlisted: false,
        viewerWatched: false,
      },
    });

    expect(apiGetPostWatchLogs).not.toHaveBeenCalled();

    await fireEvent.mouseEnter(screen.getByTestId('movie-watch-stat'));
    await vi.runAllTimersAsync();

    expect(apiGetPostWatchLogs).toHaveBeenCalledWith('post-4');
    expect(await screen.findByText('Alex')).toBeInTheDocument();
    expect(screen.getByText('4.5')).toBeInTheDocument();

    vi.useRealTimers();
  });

  it('uses mobile-first responsive layout classes', () => {
    render(MovieStatsBar, {
      postId: 'post-5',
      stats: {
        watchlistCount: 1,
        watchCount: 1,
        avgRating: 4,
        viewerWatchlisted: false,
        viewerWatched: false,
      },
    });

    const container = screen.getByTestId('movie-stats-bar');
    expect(container.className).toContain('flex-col');
    expect(container.className).toContain('sm:flex-row');
  });

  it('passes viewer state props to action buttons', () => {
    render(MovieStatsBar, {
      postId: 'post-6',
      stats: {
        watchlistCount: 5,
        watchCount: 3,
        avgRating: 3.5,
        viewerWatchlisted: true,
        viewerWatched: true,
        viewerRating: 5,
        viewerCategories: ['Favorites', 'Sci-Fi'],
      },
    });

    const watchlistButton = screen.getByTestId('watchlist-save-button-stub');
    expect(watchlistButton).toHaveAttribute('data-post-id', 'post-6');
    expect(watchlistButton).toHaveAttribute('data-initial-saved', 'true');
    expect(watchlistButton).toHaveAttribute('data-categories', 'Favorites,Sci-Fi');
    expect(watchlistButton).toHaveAttribute('data-save-count', '5');

    const watchButton = screen.getByTestId('watch-button-stub');
    expect(watchButton).toHaveAttribute('data-post-id', 'post-6');
    expect(watchButton).toHaveAttribute('data-initial-watched', 'true');
    expect(watchButton).toHaveAttribute('data-initial-rating', '5');
    expect(watchButton).toHaveAttribute('data-watch-count', '3');
  });
});
