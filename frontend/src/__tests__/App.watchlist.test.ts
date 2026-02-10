import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import * as stores from '../stores';
import { movieStore } from '../stores/movieStore';

const loadFeedMock = vi.hoisted(() => vi.fn());
const loadMorePostsMock = vi.hoisted(() => vi.fn());
const loadThreadTargetPostMock = vi.hoisted(() => vi.fn());
const loadSectionLinksMock = vi.hoisted(() => vi.fn());
const loadMoreSectionLinksMock = vi.hoisted(() => vi.fn());

vi.mock('../stores/feedStore', () => ({
  loadFeed: loadFeedMock,
  loadMorePosts: loadMorePostsMock,
}));

vi.mock('../stores/sectionLinksFeedStore', () => ({
  loadSectionLinks: loadSectionLinksMock,
  loadMoreSectionLinks: loadMoreSectionLinksMock,
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
  loadSectionLinksMock.mockResolvedValue(undefined);
  loadMoreSectionLinksMock.mockResolvedValue(undefined);

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
  it('maps legacy /watchlist route into Movies section feed with inline watchlist for authenticated users', async () => {
    stores.authStore.setUser(defaultUser);
    window.history.replaceState(null, '', '/watchlist');
    expect(window.location.pathname).toBe('/watchlist');

    render(App);
    window.dispatchEvent(new Event('popstate'));

    await waitFor(() => {
      expect(screen.getByTestId('watchlist')).toBeInTheDocument();
    });
    expect(screen.getByRole('heading', { name: 'Movies' })).toBeInTheDocument();
    expect(screen.queryByTestId('section-tab-watchlist')).not.toBeInTheDocument();
    expect(screen.getByPlaceholderText(/Share something in Movies/i)).toBeInTheDocument();
    expect(window.location.pathname).toBe('/sections/movies');
    expect(document.title).toBe('Clubhouse');
  });

  it('renders watchlist and feed together in Movies section without a section-level watchlist toggle', async () => {
    stores.authStore.setUser(defaultUser);
    window.history.replaceState(null, '', '/sections/movies');

    render(App);

    await waitFor(() => {
      expect(screen.getByTestId('watchlist')).toBeInTheDocument();
    });
    expect(screen.getByPlaceholderText(/Share something in Movies/i)).toBeInTheDocument();
    expect(window.location.pathname).toBe('/sections/movies');
    expect(screen.queryByTestId('section-tab-watchlist')).not.toBeInTheDocument();
    expect(screen.getByTestId('watchlist-collapse')).toHaveAttribute('aria-expanded', 'true');
  });

  it('renders watchlist and feed together in Series section without a section-level watchlist toggle', async () => {
    stores.authStore.setUser(defaultUser);
    stores.sectionStore.setActiveSection({
      id: 'section-series',
      name: 'Series',
      type: 'series',
      icon: '📺',
      slug: 'series',
    });
    window.history.replaceState(null, '', '/sections/series');

    render(App);

    await waitFor(() => {
      expect(screen.getByTestId('watchlist')).toBeInTheDocument();
    });
    expect(screen.getByPlaceholderText(/Share something in Series/i)).toBeInTheDocument();
    expect(screen.queryByTestId('section-tab-watchlist')).not.toBeInTheDocument();
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

  it('renders podcast top container only for podcast sections', async () => {
    stores.authStore.setUser(defaultUser);
    stores.sectionStore.setSections([
      { id: 'section-general', name: 'General', type: 'general', icon: '💬', slug: 'general' },
      { id: 'section-podcasts', name: 'Podcasts', type: 'podcast', icon: '🎙️', slug: 'podcasts' },
    ]);
    stores.sectionStore.setActiveSection({
      id: 'section-podcasts',
      name: 'Podcasts',
      type: 'podcast',
      icon: '🎙️',
      slug: 'podcasts',
    });
    window.history.replaceState(null, '', '/');

    render(App);

    await waitFor(() => {
      expect(screen.getByTestId('podcasts-top-container')).toBeInTheDocument();
    });
  });

  it('does not render podcast top container for non-podcast sections', async () => {
    stores.authStore.setUser(defaultUser);
    stores.sectionStore.setSections([
      { id: 'section-general', name: 'General', type: 'general', icon: '💬', slug: 'general' },
      { id: 'section-music', name: 'Music', type: 'music', icon: '🎵', slug: 'music' },
      { id: 'section-podcasts', name: 'Podcasts', type: 'podcast', icon: '🎙️', slug: 'podcasts' },
    ]);
    stores.sectionStore.setActiveSection({
      id: 'section-music',
      name: 'Music',
      type: 'music',
      icon: '🎵',
      slug: 'music',
    });
    window.history.replaceState(null, '', '/');

    render(App);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Music' })).toBeInTheDocument();
    });
    expect(screen.queryByTestId('podcasts-top-container')).not.toBeInTheDocument();
  });
});
