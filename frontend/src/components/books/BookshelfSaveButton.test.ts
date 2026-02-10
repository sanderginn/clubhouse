import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const apiGetBookshelfCategories = vi.hoisted(() => vi.fn());
const apiGetMyBookshelf = vi.hoisted(() => vi.fn());
const apiAddToBookshelf = vi.hoisted(() => vi.fn());
const apiRemoveFromBookshelf = vi.hoisted(() => vi.fn());
const apiCreateBookshelfCategory = vi.hoisted(() => vi.fn());
const apiGetPostReadLogs = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    getBookshelfCategories: apiGetBookshelfCategories,
    getMyBookshelf: apiGetMyBookshelf,
    addToBookshelf: apiAddToBookshelf,
    removeFromBookshelf: apiRemoveFromBookshelf,
    createBookshelfCategory: apiCreateBookshelfCategory,
    getPostReadLogs: apiGetPostReadLogs,
  },
}));

const { default: BookshelfSaveButton } = await import('./BookshelfSaveButton.svelte');
const { bookStore, bookshelfCategories } = await import('../../stores/bookStore');
const { authStore } = await import('../../stores/authStore');

const createDeferred = <T>() => {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
};

beforeEach(() => {
  bookStore.reset();
  authStore.setUser({
    id: 'user-1',
    username: 'reader',
    email: 'reader@example.com',
    isAdmin: false,
    totpEnabled: false,
  });

  apiGetBookshelfCategories.mockReset();
  apiGetMyBookshelf.mockReset();
  apiAddToBookshelf.mockReset();
  apiRemoveFromBookshelf.mockReset();
  apiCreateBookshelfCategory.mockReset();
  apiGetPostReadLogs.mockReset();

  apiGetBookshelfCategories.mockResolvedValue({
    categories: [{ id: 'cat-1', name: 'Favorites', position: 0 }],
  });
  apiGetMyBookshelf.mockResolvedValue({ bookshelfItems: [], nextCursor: undefined });
  apiAddToBookshelf.mockResolvedValue(undefined);
  apiRemoveFromBookshelf.mockResolvedValue(undefined);
  apiCreateBookshelfCategory.mockResolvedValue({
    category: { id: 'cat-2', name: 'Classics', position: 1 },
  });
  apiGetPostReadLogs.mockResolvedValue({
    readCount: 0,
    averageRating: null,
    viewerRead: false,
    viewerRating: null,
    readers: [],
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('BookshelfSaveButton', () => {
  it('renders unsaved state', () => {
    render(BookshelfSaveButton, {
      postId: 'post-1',
      bookStats: {
        bookshelfCount: 0,
        readCount: 0,
        averageRating: null,
        viewerOnBookshelf: false,
        viewerCategories: [],
      },
    });

    expect(screen.getByText('Save')).toBeInTheDocument();
    expect(screen.queryByText('Saved')).not.toBeInTheDocument();
  });

  it('renders saved state from book stats', async () => {
    render(BookshelfSaveButton, {
      postId: 'post-1',
      bookStats: {
        bookshelfCount: 3,
        readCount: 0,
        averageRating: null,
        viewerOnBookshelf: true,
        viewerCategories: ['Favorites'],
      },
    });

    expect(screen.getByText('Saved')).toBeInTheDocument();
    expect(screen.queryByText('Save')).not.toBeInTheDocument();
  });

  it('toggles save and unsave from the main button', async () => {
    render(BookshelfSaveButton, {
      postId: 'post-2',
      bookStats: {
        bookshelfCount: 0,
        readCount: 0,
        averageRating: null,
        viewerOnBookshelf: false,
        viewerCategories: [],
      },
    });

    await fireEvent.click(screen.getByRole('button', { name: 'Save to bookshelf' }));

    await waitFor(() => {
      expect(apiAddToBookshelf).toHaveBeenCalledWith('post-2', []);
    });
    expect(screen.getByText('Saved')).toBeInTheDocument();

    await fireEvent.click(screen.getByRole('button', { name: 'Remove from bookshelf' }));

    await waitFor(() => {
      expect(apiRemoveFromBookshelf).toHaveBeenCalledWith('post-2');
    });
    expect(screen.getByText('Save')).toBeInTheDocument();
  });

  it('opens category dropdown from chevron button', async () => {
    render(BookshelfSaveButton, {
      postId: 'post-3',
      bookStats: {
        bookshelfCount: 0,
        readCount: 0,
        averageRating: null,
        viewerOnBookshelf: false,
        viewerCategories: [],
      },
    });

    await fireEvent.click(screen.getByTestId('bookshelf-dropdown-toggle'));

    expect(screen.getByRole('dialog', { name: 'Select bookshelf categories' })).toBeInTheDocument();
    expect(screen.getByText('+ Create category')).toBeInTheDocument();
  });

  it('persists a single category when multiple are selected before close', async () => {
    bookshelfCategories.set([
      { id: 'cat-1', name: 'Favorites', position: 0 },
      { id: 'cat-2', name: 'Sci-Fi', position: 1 },
    ]);

    render(BookshelfSaveButton, {
      postId: 'post-multi',
      bookStats: {
        bookshelfCount: 0,
        readCount: 0,
        averageRating: null,
        viewerOnBookshelf: false,
        viewerCategories: [],
      },
    });

    await fireEvent.click(screen.getByTestId('bookshelf-dropdown-toggle'));
    await fireEvent.click(screen.getByLabelText('Favorites'));
    await fireEvent.click(screen.getByLabelText('Sci-Fi'));
    await fireEvent.click(screen.getByTestId('bookshelf-dropdown-toggle'));

    await waitFor(() => {
      expect(apiAddToBookshelf).toHaveBeenCalledWith('post-multi', ['Sci-Fi']);
    });

    await fireEvent.click(screen.getByTestId('bookshelf-dropdown-toggle'));
    expect(screen.getByLabelText('Sci-Fi')).toBeChecked();
    expect(screen.getByLabelText('Favorites')).not.toBeChecked();
  });

  it('creates a new category inline and selects it', async () => {
    render(BookshelfSaveButton, {
      postId: 'post-4',
      bookStats: {
        bookshelfCount: 0,
        readCount: 0,
        averageRating: null,
        viewerOnBookshelf: false,
        viewerCategories: [],
      },
    });

    await fireEvent.click(screen.getByTestId('bookshelf-dropdown-toggle'));
    await fireEvent.click(screen.getByText('+ Create category'));

    expect(screen.getByTestId('bookshelf-new-category-inline')).toBeInTheDocument();

    await fireEvent.input(screen.getByLabelText('New category name'), {
      target: { value: 'Classics' },
    });
    await fireEvent.click(screen.getByRole('button', { name: 'Create' }));

    await waitFor(() => {
      expect(apiCreateBookshelfCategory).toHaveBeenCalledWith('Classics');
    });
    expect(screen.getByLabelText('Classics')).toBeChecked();
  });

  it('shows optimistic saved state while add call is in flight', async () => {
    const deferred = createDeferred<void>();
    apiAddToBookshelf.mockReturnValueOnce(deferred.promise);

    render(BookshelfSaveButton, {
      postId: 'post-5',
      bookStats: {
        bookshelfCount: 0,
        readCount: 0,
        averageRating: null,
        viewerOnBookshelf: false,
        viewerCategories: [],
      },
    });

    await fireEvent.click(screen.getByRole('button', { name: 'Save to bookshelf' }));

    expect(screen.getByText('Saved')).toBeInTheDocument();
    expect(screen.getByTestId('bookshelf-save-spinner')).toBeInTheDocument();

    deferred.resolve(undefined);
    await waitFor(() => {
      expect(apiAddToBookshelf).toHaveBeenCalledWith('post-5', []);
    });
  });
});
