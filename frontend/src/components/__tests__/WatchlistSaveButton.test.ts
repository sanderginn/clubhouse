import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/svelte';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { tick } from 'svelte';

const apiAddToWatchlist = vi.hoisted(() => vi.fn());
const apiRemoveFromWatchlist = vi.hoisted(() => vi.fn());
const apiCreateWatchlistCategory = vi.hoisted(() => vi.fn());
const apiGetMyWatchlist = vi.hoisted(() => vi.fn());
const apiGetWatchlistCategories = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    addToWatchlist: apiAddToWatchlist,
    removeFromWatchlist: apiRemoveFromWatchlist,
    createWatchlistCategory: apiCreateWatchlistCategory,
    getMyWatchlist: apiGetMyWatchlist,
    getWatchlistCategories: apiGetWatchlistCategories,
  },
}));

const { movieStore } = await import('../../stores/movieStore');
const { authStore } = await import('../../stores/authStore');
const { default: WatchlistSaveButton } = await import('../movies/WatchlistSaveButton.svelte');

beforeEach(() => {
  movieStore.reset();
  authStore.setUser({
    id: 'user-1',
    username: 'movie-fan',
    email: 'movie-fan@example.com',
    isAdmin: false,
    totpEnabled: false,
  });

  apiAddToWatchlist.mockReset();
  apiRemoveFromWatchlist.mockReset();
  apiCreateWatchlistCategory.mockReset();
  apiGetMyWatchlist.mockReset();
  apiGetWatchlistCategories.mockReset();

  apiGetMyWatchlist.mockResolvedValue({ categories: [] });
  apiGetWatchlistCategories.mockResolvedValue({ categories: [] });
  apiRemoveFromWatchlist.mockResolvedValue(undefined);
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('WatchlistSaveButton', () => {
  it('renders initial not-saved state', () => {
    render(WatchlistSaveButton, { postId: 'post-1' });

    expect(screen.getByText('Add to List')).toBeInTheDocument();
    expect(screen.queryByText('In List')).not.toBeInTheDocument();
  });

  it('renders initial saved state with categories and save count', () => {
    render(WatchlistSaveButton, {
      postId: 'post-1',
      initialSaved: true,
      initialCategories: ['Favorites', 'Classics'],
      saveCount: 7,
    });

    expect(screen.getByText('In List')).toBeInTheDocument();
    expect(screen.getByTestId('watchlist-save-count')).toHaveTextContent('7');
  });

  it('opens dropdown when clicked', async () => {
    render(WatchlistSaveButton, { postId: 'post-1' });

    await fireEvent.click(screen.getByRole('button', { name: 'Add movie to watchlist' }));

    expect(screen.getByText('Save to watchlist')).toBeInTheDocument();
    expect(screen.getByText('+ Create category')).toBeInTheDocument();
  });

  it('selects categories and applies changes', async () => {
    movieStore.setCategories([
      {
        id: 'cat-1',
        name: 'Favorites',
        position: 1,
      },
    ]);

    apiAddToWatchlist.mockResolvedValue({
      watchlistItems: [
        {
          id: 'watch-1',
          userId: 'user-1',
          postId: 'post-2',
          category: 'Favorites',
          createdAt: '2026-02-01T00:00:00Z',
        },
      ],
    });

    render(WatchlistSaveButton, { postId: 'post-2' });

    await fireEvent.click(screen.getByRole('button', { name: 'Add movie to watchlist' }));
    await fireEvent.click(screen.getByLabelText('Favorites'));
    await fireEvent.click(screen.getByRole('button', { name: 'Apply' }));
    await tick();

    expect(apiAddToWatchlist).toHaveBeenCalledWith('post-2', ['Favorites']);
    expect(screen.getByText('In List')).toBeInTheDocument();
  });

  it('creates a new category inline', async () => {
    apiCreateWatchlistCategory.mockResolvedValue({
      category: {
        id: 'cat-2',
        name: 'Classics',
        position: 2,
      },
    });

    render(WatchlistSaveButton, { postId: 'post-3' });

    await fireEvent.click(screen.getByRole('button', { name: 'Add movie to watchlist' }));
    await fireEvent.click(screen.getByText('+ Create category'));

    expect(screen.getByTestId('watchlist-new-category-inline')).toBeInTheDocument();

    await fireEvent.input(screen.getByLabelText('New category name'), {
      target: { value: 'Classics' },
    });
    await fireEvent.click(screen.getByRole('button', { name: 'Create' }));

    await waitFor(() => {
      expect(apiCreateWatchlistCategory).toHaveBeenCalledWith('Classics');
    });

    expect(screen.getByLabelText('Classics')).toBeChecked();
  });

  it('keeps initial saved state when initial watchlist load fails', async () => {
    apiGetMyWatchlist.mockRejectedValue(new Error('Load failed'));

    render(WatchlistSaveButton, {
      postId: 'post-5',
      initialSaved: true,
      initialCategories: ['Favorites'],
      saveCount: 2,
    });

    await tick();
    await tick();

    expect(screen.getByText('In List')).toBeInTheDocument();
    expect(screen.queryByText('Add to List')).not.toBeInTheDocument();
    expect(screen.getByTestId('watchlist-save-count')).toHaveTextContent('2');
  });

  it('shows toast and reverts optimistic state on apply error', async () => {
    movieStore.setCategories([
      {
        id: 'cat-1',
        name: 'Favorites',
        position: 1,
      },
    ]);

    apiAddToWatchlist.mockRejectedValue(new Error('Network broke'));

    render(WatchlistSaveButton, { postId: 'post-4' });

    await fireEvent.click(screen.getByRole('button', { name: 'Add movie to watchlist' }));
    await fireEvent.click(screen.getByLabelText('Favorites'));
    await fireEvent.click(screen.getByRole('button', { name: 'Apply' }));

    await waitFor(() => {
      expect(screen.getByTestId('watchlist-error-toast')).toHaveTextContent('Network broke');
    });

    expect(screen.getByText('Add to List')).toBeInTheDocument();
    expect(screen.queryByText('In List')).not.toBeInTheDocument();
  });
});
