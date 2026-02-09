import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/svelte';
import * as stores from '../stores';
import { movieStore } from '../stores/movieStore';

const { default: App } = await import('../App.svelte');

const defaultUser = {
  id: 'user-1',
  username: 'moviefan',
  email: 'moviefan@example.com',
  isAdmin: false,
  totpEnabled: false,
};

beforeEach(() => {
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

  stores.authStore.setUser(null);
  stores.sectionStore.setSections([]);
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
  it('renders watchlist route for authenticated users', async () => {
    stores.authStore.setUser(defaultUser);
    window.history.replaceState(null, '', '/watchlist');
    expect(window.location.pathname).toBe('/watchlist');

    render(App);
    window.dispatchEvent(new PopStateEvent('popstate'));

    await waitFor(() => {
      expect(screen.getByTestId('watchlist')).toBeInTheDocument();
    });
    expect(screen.getByRole('heading', { name: 'My Watchlist' })).toBeInTheDocument();
    expect(document.title).toBe('My Watchlist - Clubhouse');
  });

  it('renders login for unauthenticated users on watchlist route', async () => {
    window.history.replaceState(null, '', '/watchlist');
    expect(window.location.pathname).toBe('/watchlist');

    render(App);
    window.dispatchEvent(new PopStateEvent('popstate'));

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Sign in to Clubhouse' })).toBeInTheDocument();
    });
    expect(screen.queryByTestId('watchlist')).not.toBeInTheDocument();
  });
});
