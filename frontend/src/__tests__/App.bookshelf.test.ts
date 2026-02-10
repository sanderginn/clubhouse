import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { get } from 'svelte/store';
import * as stores from '../stores';
import { movieStore } from '../stores/movieStore';
import { bookStore } from '../stores/bookStore';

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
  username: 'bookfan',
  email: 'bookfan@example.com',
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
  vi.spyOn(bookStore, 'loadBookshelfCategories').mockResolvedValue();
  vi.spyOn(bookStore, 'loadMyBookshelf').mockResolvedValue(undefined);
  vi.spyOn(bookStore, 'loadAllBookshelf').mockResolvedValue(undefined);
  loadFeedMock.mockResolvedValue(undefined);
  loadMorePostsMock.mockResolvedValue(undefined);
  loadThreadTargetPostMock.mockResolvedValue(undefined);
  loadSectionLinksMock.mockResolvedValue(undefined);
  loadMoreSectionLinksMock.mockResolvedValue(undefined);

  stores.authStore.setUser(null);
  stores.sectionStore.setSections([
    { id: 'section-books', name: 'Books', type: 'book', icon: '📚', slug: 'books' },
    { id: 'section-music', name: 'Music', type: 'music', icon: '🎵', slug: 'music' },
  ]);
  stores.uiStore.setActiveView('feed');
  stores.threadRouteStore.clearTarget();
  movieStore.reset();
  bookStore.reset();
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  window.history.replaceState(null, '', '/');
});

describe('App bookshelf routing', () => {
  it('does not treat /bookshelf as a standalone bookshelf view for authenticated users', async () => {
    stores.authStore.setUser(defaultUser);
    window.history.replaceState(null, '', '/bookshelf');

    render(App);
    window.dispatchEvent(new Event('popstate'));

    await waitFor(() => {
      expect(get(stores.activeView)).toBe('feed');
    });
    expect(screen.queryByTestId('section-tab-bookshelf')).not.toBeInTheDocument();
    expect(document.title).toBe('Clubhouse');
  });

  it('renders login for unauthenticated users on legacy bookshelf route', async () => {
    window.history.replaceState(null, '', '/bookshelf');

    render(App);
    window.dispatchEvent(new Event('popstate'));

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Sign in to Clubhouse' })).toBeInTheDocument();
    });
    expect(screen.queryByRole('heading', { name: 'Bookshelf' })).not.toBeInTheDocument();
  });

  it('renders Bookshelf inline in books section without a standalone tab', async () => {
    stores.authStore.setUser(defaultUser);
    window.history.replaceState(null, '', '/sections/books');

    render(App);

    await fireEvent.click(await screen.findByRole('button', { name: 'Books' }));

    await waitFor(() => {
      expect(screen.getByTestId('bookshelf')).toBeInTheDocument();
    });
    expect(screen.queryByTestId('section-tab-bookshelf')).not.toBeInTheDocument();
    expect(window.location.pathname).toBe('/sections/books');
  });
});
