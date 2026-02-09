import { beforeEach, describe, expect, it, vi } from 'vitest';
import { get } from 'svelte/store';

const apiGetMyWatchlist = vi.hoisted(() => vi.fn());
const apiGetWatchlistCategories = vi.hoisted(() => vi.fn());
const apiAddToWatchlist = vi.hoisted(() => vi.fn());
const apiRemoveFromWatchlist = vi.hoisted(() => vi.fn());
const apiCreateWatchlistCategory = vi.hoisted(() => vi.fn());
const apiUpdateWatchlistCategory = vi.hoisted(() => vi.fn());
const apiDeleteWatchlistCategory = vi.hoisted(() => vi.fn());
const apiGetMyWatchLogs = vi.hoisted(() => vi.fn());
const apiLogWatch = vi.hoisted(() => vi.fn());
const apiUpdateWatchLog = vi.hoisted(() => vi.fn());
const apiRemoveWatchLog = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    getMyWatchlist: apiGetMyWatchlist,
    getWatchlistCategories: apiGetWatchlistCategories,
    addToWatchlist: apiAddToWatchlist,
    removeFromWatchlist: apiRemoveFromWatchlist,
    createWatchlistCategory: apiCreateWatchlistCategory,
    updateWatchlistCategory: apiUpdateWatchlistCategory,
    deleteWatchlistCategory: apiDeleteWatchlistCategory,
    getMyWatchLogs: apiGetMyWatchLogs,
    logWatch: apiLogWatch,
    updateWatchLog: apiUpdateWatchLog,
    removeWatchLog: apiRemoveWatchLog,
  },
}));

const {
  movieStore,
  watchlistByCategory,
  sortedCategories,
  handleMovieWatchlistedEvent,
  handleMovieUnwatchlistedEvent,
  handleMovieWatchedEvent,
  handleMovieWatchRemovedEvent,
} = await import('../movieStore');
const { authStore } = await import('../authStore');

beforeEach(() => {
  movieStore.reset();
  authStore.setUser({
    id: 'user-1',
    username: 'movie-user',
    email: 'movie@example.com',
    isAdmin: false,
    totpEnabled: false,
  });
  apiGetMyWatchlist.mockReset();
  apiGetWatchlistCategories.mockReset();
  apiAddToWatchlist.mockReset();
  apiRemoveFromWatchlist.mockReset();
  apiCreateWatchlistCategory.mockReset();
  apiUpdateWatchlistCategory.mockReset();
  apiDeleteWatchlistCategory.mockReset();
  apiGetMyWatchLogs.mockReset();
  apiLogWatch.mockReset();
  apiUpdateWatchLog.mockReset();
  apiRemoveWatchLog.mockReset();
});

describe('movieStore', () => {
  it('loadWatchlist populates grouped watchlist map', async () => {
    apiGetMyWatchlist.mockResolvedValue({
      categories: [
        {
          name: 'Favorites',
          items: [
            {
              id: 'watch-1',
              userId: 'user-1',
              postId: 'post-1',
              category: 'Favorites',
              createdAt: '2026-01-01T00:00:00Z',
              post: {
                id: 'post-1',
                user_id: 'user-1',
                section_id: 'section-1',
                content: 'Movie 1',
                created_at: '2026-01-01T00:00:00Z',
              },
            },
          ],
        },
      ],
    });

    await movieStore.loadWatchlist();

    const state = get(movieStore);
    const favorites = state.watchlist.get('Favorites') ?? [];
    expect(state.isLoadingWatchlist).toBe(false);
    expect(favorites).toHaveLength(1);
    expect(favorites[0].post?.id).toBe('post-1');
  });

  it('addToWatchlist and removeFromWatchlist update state', async () => {
    apiAddToWatchlist.mockResolvedValue({
      watchlistItems: [
        {
          id: 'watch-2',
          userId: 'user-1',
          postId: 'post-2',
          category: 'Favorites',
          createdAt: '2026-01-02T00:00:00Z',
        },
      ],
    });
    apiRemoveFromWatchlist.mockResolvedValue(undefined);

    await movieStore.addToWatchlist('post-2', ['Favorites']);
    let state = get(movieStore);
    expect(state.watchlist.get('Favorites')).toHaveLength(1);

    await movieStore.removeFromWatchlist('post-2', 'Favorites');
    state = get(movieStore);
    expect(state.watchlist.get('Favorites')).toBeUndefined();
  });

  it('category CRUD updates categories and watchlist mappings', async () => {
    apiCreateWatchlistCategory.mockResolvedValue({
      category: { id: 'cat-1', name: 'Favorites', position: 2 },
    });
    apiUpdateWatchlistCategory.mockResolvedValue({
      category: { id: 'cat-1', name: 'Top Picks', position: 1 },
    });
    apiDeleteWatchlistCategory.mockResolvedValue(undefined);

    await movieStore.createCategory('Favorites');
    movieStore.applyWatchlistItems([
      {
        id: 'watch-3',
        userId: 'user-1',
        postId: 'post-3',
        category: 'Favorites',
        createdAt: '2026-01-03T00:00:00Z',
      },
    ]);

    await movieStore.updateCategory('cat-1', { name: 'Top Picks' });

    let state = get(movieStore);
    expect(state.watchlist.get('Favorites')).toBeUndefined();
    expect(state.watchlist.get('Top Picks')).toHaveLength(1);

    await movieStore.deleteCategory('cat-1');

    state = get(movieStore);
    const uncategorized = state.watchlist.get('Uncategorized') ?? [];
    expect(state.categories).toHaveLength(0);
    expect(uncategorized).toHaveLength(1);
    expect(uncategorized[0].category).toBe('Uncategorized');
  });

  it('loadWatchLogs and watch log actions update state', async () => {
    apiGetMyWatchLogs.mockResolvedValue({
      watchLogs: [
        {
          id: 'log-1',
          userId: 'user-1',
          postId: 'post-1',
          rating: 4,
          notes: 'Nice',
          watchedAt: '2026-01-01T00:00:00Z',
          post: {
            id: 'post-1',
            user_id: 'user-1',
            section_id: 'section-1',
            content: 'Movie 1',
            created_at: '2026-01-01T00:00:00Z',
          },
        },
      ],
    });
    apiLogWatch.mockResolvedValue({
      watchLog: {
        id: 'log-2',
        userId: 'user-1',
        postId: 'post-2',
        rating: 5,
        notes: 'Great',
        watchedAt: '2026-01-02T00:00:00Z',
      },
    });
    apiUpdateWatchLog.mockResolvedValue({
      watchLog: {
        id: 'log-2',
        userId: 'user-1',
        postId: 'post-2',
        rating: 3,
        notes: 'Actually okay',
        watchedAt: '2026-01-02T00:00:00Z',
      },
    });
    apiRemoveWatchLog.mockResolvedValue(undefined);

    await movieStore.loadWatchLogs(10);
    await movieStore.logWatch('post-2', 5, 'Great');
    await movieStore.updateWatchLog('post-2', 3, 'Actually okay');
    await movieStore.removeWatchLog('post-2');

    const state = get(movieStore);
    expect(state.watchLogs).toHaveLength(1);
    expect(state.watchLogs[0].postId).toBe('post-1');
  });

  it('derived stores expose grouped watchlist and sorted categories', () => {
    movieStore.setCategories([
      { id: 'cat-1', name: 'Zed', position: 2 },
      { id: 'cat-2', name: 'Alpha', position: 1 },
    ]);

    const grouped = get(watchlistByCategory);
    const sorted = get(sortedCategories);

    expect(grouped).toBeInstanceOf(Map);
    expect(sorted[0].name).toBe('Alpha');
  });

  it('websocket handlers apply movie events to local state', () => {
    handleMovieWatchlistedEvent({
      user_id: 'user-1',
      watchlist_item: {
        id: 'watch-ws',
        userId: 'user-1',
        postId: 'post-ws',
        category: 'Favorites',
        createdAt: '2026-01-02T00:00:00Z',
      },
    });
    handleMovieWatchedEvent({
      user_id: 'user-1',
      watch_log: {
        id: 'log-ws',
        userId: 'user-1',
        postId: 'post-ws',
        rating: 5,
        watchedAt: '2026-01-02T00:00:00Z',
      },
    });

    let state = get(movieStore);
    expect(state.watchlist.get('Favorites')).toHaveLength(1);
    expect(state.watchLogs).toHaveLength(1);

    handleMovieUnwatchlistedEvent({ post_id: 'post-ws', user_id: 'user-1', category: 'Favorites' });
    handleMovieWatchRemovedEvent({ post_id: 'post-ws', user_id: 'user-1' });

    state = get(movieStore);
    expect(state.watchlist.get('Favorites')).toBeUndefined();
    expect(state.watchLogs).toHaveLength(0);
  });

  it('websocket handlers ignore events from other users and sparse self payloads', () => {
    movieStore.applyWatchlistItems([
      {
        id: 'watch-own',
        userId: 'user-1',
        postId: 'post-own',
        category: 'Favorites',
        createdAt: '2026-01-01T00:00:00Z',
      },
    ]);
    movieStore.applyWatchLog({
      id: 'log-own',
      userId: 'user-1',
      postId: 'post-own',
      rating: 4,
      watchedAt: '2026-01-01T00:00:00Z',
    });

    handleMovieWatchlistedEvent({
      user_id: 'user-2',
      watchlist_item: {
        id: 'watch-other',
        userId: 'user-2',
        postId: 'post-other',
        category: 'Favorites',
        createdAt: '2026-01-03T00:00:00Z',
      },
    });
    handleMovieWatchedEvent({
      user_id: 'user-2',
      watch_log: {
        id: 'log-other',
        userId: 'user-2',
        postId: 'post-other',
        rating: 2,
        watchedAt: '2026-01-03T00:00:00Z',
      },
    });
    handleMovieUnwatchlistedEvent({
      post_id: 'post-own',
      user_id: 'user-2',
      category: 'Favorites',
    });
    handleMovieWatchRemovedEvent({
      post_id: 'post-own',
      user_id: 'user-2',
    });

    handleMovieWatchlistedEvent({
      post_id: 'post-sparse',
      user_id: 'user-1',
      categories: ['Favorites'],
    });
    handleMovieWatchedEvent({
      post_id: 'post-sparse',
      user_id: 'user-1',
      rating: 5,
    });

    const state = get(movieStore);
    const favorites = state.watchlist.get('Favorites') ?? [];
    expect(favorites).toHaveLength(1);
    expect(favorites[0].postId).toBe('post-own');
    expect(state.watchLogs).toHaveLength(1);
    expect(state.watchLogs[0].postId).toBe('post-own');
  });
});
