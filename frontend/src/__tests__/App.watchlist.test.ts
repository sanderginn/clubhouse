import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import * as stores from '../stores';
import { movieStore } from '../stores/movieStore';

const loadFeedMock = vi.hoisted(() => vi.fn());
const loadMorePostsMock = vi.hoisted(() => vi.fn());
const loadThreadTargetPostMock = vi.hoisted(() => vi.fn());

vi.mock('../stores/feedStore', () => ({
  loadFeed: loadFeedMock,
  loadMorePosts: loadMorePostsMock,
}));

vi.mock('../stores/threadRouteStore', async () => {
  const actual = await vi.importActual('../stores/threadRouteStore');
  return {
    ...actual,
    loadThreadTargetPost: loadThreadTargetPostMock,
  };
});

const { default: App } = await import('../App.svelte');

const defaultUser = {
  id: 'user-1',
  username: 'moviefan',
  email: 'moviefan@example.com',
  isAdmin: false,
  totpEnabled: false,
};

beforeEach(() => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation(() => ({
      matches: false,
      media: '(max-width: 1023px)',
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
  vi.spyOn(stores.authStore, 'checkSession').mockResolvedValue(false);
  vi.spyOn(stores.websocketStore, 'init').mockImplementation(() => {});
  vi.spyOn(stores.websocketStore, 'cleanup').mockImplementation(() => {});
  vi.spyOn(stores.sectionStore, 'loadSections').mockResolvedValue();
  vi.spyOn(stores.pwaStore, 'init').mockResolvedValue();
  vi.spyOn(stores.configStore, 'load').mockResolvedValue();
  vi.spyOn(stores, 'initNotifications').mockImplementation(() => {});
  vi.spyOn(stores, 'cleanupNotifications').mockImplementation(() => {});
  vi.spyOn(movieStore, 'loadWatchlist').mockResolvedValue();
  vi.spyOn(movieStore, 'loadWatchlistCategories').mockResolvedValue();
  loadFeedMock.mockResolvedValue(undefined);
  loadMorePostsMock.mockResolvedValue(undefined);
  loadThreadTargetPostMock.mockResolvedValue(undefined);

  stores.authStore.setUser(null);
  stores.sectionStore.setSections([
    { id: 'section-movies', name: 'Movies', type: 'movie', icon: '🎬', slug: 'movies' },
    { id: 'section-series', name: 'Series', type: 'series', icon: '📺', slug: 'series' },
  ]);
  stores.uiStore.setActiveView('feed');
  stores.threadRouteStore.clearTarget();
  movieStore.reset();
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  window.history.replaceState(null, '', '/');
});

describe('App watchlist routing', () => {
  it('maps legacy /watchlist route into Movies section watchlist view for authenticated users', async () => {
    stores.authStore.setUser(defaultUser);
    window.history.replaceState(null, '', '/watchlist');
    expect(window.location.pathname).toBe('/watchlist');

    render(App);
    window.dispatchEvent(new Event('popstate'));

    await waitFor(() => {
      expect(screen.getByTestId('watchlist')).toBeInTheDocument();
    });
    expect(screen.getByRole('heading', { name: 'Movies' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Watchlist' })).toHaveAttribute('aria-selected', 'true');
    expect(window.location.pathname).toMatch(/^\/watchlist$|^\/sections\/movies\/watchlist$/);
    expect(document.title).toBe('Movies Watchlist - Clubhouse');
  });

  it('toggles between feed and watchlist tabs in Movies section', async () => {
    stores.authStore.setUser(defaultUser);
    window.history.replaceState(null, '', '/sections/movies');

    render(App);

    const watchlistTab = await screen.findByTestId('section-tab-watchlist');
    const feedTab = screen.getByTestId('section-tab-feed');
    expect(feedTab).toHaveAttribute('aria-selected', 'true');

    await fireEvent.click(watchlistTab);
    await waitFor(() => {
      expect(screen.getByTestId('watchlist')).toBeInTheDocument();
    });
    expect(window.location.pathname).toBe('/sections/movies/watchlist');

    await fireEvent.click(feedTab);
    await waitFor(() => {
      expect(screen.queryByTestId('watchlist')).not.toBeInTheDocument();
    });
    expect(window.location.pathname).toBe('/sections/movies');
    expect(feedTab).toHaveAttribute('aria-selected', 'true');
  });

  it('renders login for unauthenticated users on watchlist route', async () => {
    window.history.replaceState(null, '', '/watchlist');
    expect(window.location.pathname).toBe('/watchlist');

    render(App);
    window.dispatchEvent(new Event('popstate'));

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Sign in to Clubhouse' })).toBeInTheDocument();
    });
    expect(screen.queryByTestId('watchlist')).not.toBeInTheDocument();
  });

  it('switches unauth register route to login when navigating to watchlist', async () => {
    const { unmount } = render(App);

    await fireEvent.click(screen.getByRole('button', { name: 'create a new account' }));
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Create your account' })).toBeInTheDocument();
    });

    unmount();
    window.history.replaceState(null, '', '/watchlist');

    render(App);
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Sign in to Clubhouse' })).toBeInTheDocument();
    });
    expect(screen.queryByRole('heading', { name: 'Create your account' })).not.toBeInTheDocument();
  });
});
