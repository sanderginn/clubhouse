import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { tick } from 'svelte';
import { get, writable } from 'svelte/store';
import type { BookStats } from '../../stores/postStore';

type ReadLogEntry = {
  id: string;
  userId: string;
  postId: string;
  rating?: number;
  createdAt: string;
};

type BookStoreMetaState = {
  loading: {
    categories: boolean;
    myBookshelf: boolean;
    allBookshelf: boolean;
    readHistory: boolean;
  };
  cursors: {
    myBookshelf: string | null;
    allBookshelf: string | null;
    readHistory: string | null;
  };
  error: string | null;
};

const readHistoryState = writable<ReadLogEntry[]>([]);
const bookStoreMetaState = writable<BookStoreMetaState>({
  loading: {
    categories: false,
    myBookshelf: false,
    allBookshelf: false,
    readHistory: false,
  },
  cursors: {
    myBookshelf: null,
    allBookshelf: null,
    readHistory: null,
  },
  error: null,
});

const logRead = vi.fn(async (postId: string, rating?: number) => {
  readHistoryState.update((logs) => [
    {
      id: 'read-1',
      userId: 'user-1',
      postId,
      rating,
      createdAt: '2026-01-01T00:00:00Z',
    },
    ...logs.filter((entry) => entry.postId !== postId),
  ]);
  bookStoreMetaState.update((state) => ({ ...state, error: null }));
});

const removeRead = vi.fn(async (postId: string) => {
  readHistoryState.update((logs) => logs.filter((entry) => entry.postId !== postId));
  bookStoreMetaState.update((state) => ({ ...state, error: null }));
});

const updateRating = vi.fn(async (postId: string, rating: number) => {
  readHistoryState.update((logs) => {
    const existing = logs.find((entry) => entry.postId === postId);
    if (existing) {
      return logs.map((entry) => (entry.postId === postId ? { ...entry, rating } : entry));
    }
    return [
      {
        id: 'read-new',
        userId: 'user-1',
        postId,
        rating,
        createdAt: '2026-01-01T00:00:00Z',
      },
      ...logs,
    ];
  });
  bookStoreMetaState.update((state) => ({ ...state, error: null }));
});

const setState = (next: { readHistory?: ReadLogEntry[]; error?: string | null }) => {
  if (next.readHistory) {
    readHistoryState.set(next.readHistory);
  }
  if (typeof next.error !== 'undefined') {
    bookStoreMetaState.update((state) => ({ ...state, error: next.error ?? null }));
  }
};

const resetState = () => {
  readHistoryState.set([]);
  bookStoreMetaState.set({
    loading: {
      categories: false,
      myBookshelf: false,
      allBookshelf: false,
      readHistory: false,
    },
    cursors: {
      myBookshelf: null,
      allBookshelf: null,
      readHistory: null,
    },
    error: null,
  });
  logRead.mockClear();
  removeRead.mockClear();
  updateRating.mockClear();
};

vi.mock('../../stores/bookStore', () => ({
  bookStore: {
    logRead,
    removeRead,
    updateRating,
  },
  readHistory: {
    subscribe: readHistoryState.subscribe,
  },
  bookStoreMeta: {
    subscribe: bookStoreMetaState.subscribe,
  },
  __setState: setState,
  __resetState: resetState,
}));

const { default: ReadButton } = await import('./ReadButton.svelte');

const buildStats = (overrides: Partial<BookStats> = {}): BookStats => ({
  bookshelfCount: 0,
  readCount: 0,
  averageRating: null,
  viewerOnBookshelf: false,
  viewerCategories: [],
  viewerRead: false,
  viewerRating: null,
  ...overrides,
});

afterEach(() => {
  cleanup();
  resetState();
});

describe('ReadButton', () => {
  it('renders unread state', () => {
    render(ReadButton, { postId: 'post-1', bookStats: buildStats() });

    expect(screen.getByTestId('read-button')).toBeInTheDocument();
    expect(screen.getByText('Mark as Read')).toBeInTheDocument();
  });

  it('renders read state with no rating', () => {
    render(ReadButton, {
      postId: 'post-2',
      bookStats: buildStats({ viewerRead: true }),
    });

    expect(screen.getByTestId('read-button-read')).toBeInTheDocument();
    expect(screen.getByText('Read')).toBeInTheDocument();
    expect(screen.queryByTestId('read-rating-display')).not.toBeInTheDocument();
  });

  it('renders read state with rating', () => {
    render(ReadButton, {
      postId: 'post-3',
      bookStats: buildStats({ viewerRead: true, viewerRating: 4 }),
    });

    expect(screen.getByTestId('read-button-read')).toBeInTheDocument();
    expect(screen.getByTestId('read-rating-display')).toBeInTheDocument();
    expect(screen.getByTestId('read-rating-value')).toHaveTextContent('4');
  });

  it('marks a post as read on first click', async () => {
    render(ReadButton, { postId: 'post-4', bookStats: buildStats() });

    await fireEvent.click(screen.getByTestId('read-button'));

    expect(logRead).toHaveBeenCalledWith('post-4', undefined);
    await tick();
    expect(screen.getByText('Read')).toBeInTheDocument();
    expect(screen.getByTestId('read-rating-popover')).toBeInTheDocument();
  });

  it('does not open rating popover when marking as read fails', async () => {
    logRead.mockImplementationOnce(async () => {
      bookStoreMetaState.update((state) => ({ ...state, error: 'unable to mark as read' }));
    });

    render(ReadButton, { postId: 'post-4-error', bookStats: buildStats() });

    await fireEvent.click(screen.getByTestId('read-button'));

    expect(logRead).toHaveBeenCalledWith('post-4-error', undefined);
    await tick();
    expect(screen.getByText('Mark as Read')).toBeInTheDocument();
    expect(screen.queryByTestId('read-rating-popover')).not.toBeInTheDocument();
  });

  it('keeps popover toggle behavior for already-read books', async () => {
    setState({
      readHistory: [
        {
          id: 'read-4-toggle',
          userId: 'user-1',
          postId: 'post-4-toggle',
          createdAt: '2026-01-01T00:00:00Z',
        },
      ],
    });

    render(ReadButton, {
      postId: 'post-4-toggle',
      bookStats: buildStats({ viewerRead: true }),
    });

    await fireEvent.click(screen.getByTestId('read-button-read'));
    expect(screen.getByTestId('read-rating-popover')).toBeInTheDocument();

    await fireEvent.click(screen.getByTestId('read-button-read'));
    expect(screen.queryByTestId('read-rating-popover')).not.toBeInTheDocument();
  });

  it('updates rating when selecting a star', async () => {
    setState({
      readHistory: [
        {
          id: 'read-5',
          userId: 'user-1',
          postId: 'post-5',
          createdAt: '2026-01-01T00:00:00Z',
        },
      ],
    });

    render(ReadButton, {
      postId: 'post-5',
      bookStats: buildStats({ viewerRead: true }),
    });

    await fireEvent.click(screen.getByTestId('read-button-read'));
    expect(screen.getByTestId('read-rating-popover')).toBeInTheDocument();

    await fireEvent.click(screen.getByTestId('rating-star-5'));

    expect(updateRating).toHaveBeenCalledWith('post-5', 5);
    await tick();

    expect(get(readHistoryState).find((entry) => entry.postId === 'post-5')?.rating).toBe(5);
  });

  it('removes read log from remove menu', async () => {
    setState({
      readHistory: [
        {
          id: 'read-6',
          userId: 'user-1',
          postId: 'post-6',
          rating: 3,
          createdAt: '2026-01-01T00:00:00Z',
        },
      ],
    });

    render(ReadButton, {
      postId: 'post-6',
      bookStats: buildStats({ viewerRead: true, viewerRating: 3 }),
    });

    await fireEvent.contextMenu(screen.getByTestId('read-button-read'));
    expect(screen.getByTestId('read-remove-menu')).toBeInTheDocument();

    await fireEvent.click(screen.getByTestId('read-remove'));

    expect(removeRead).toHaveBeenCalledWith('post-6');
    await tick();
    expect(screen.getByText('Mark as Read')).toBeInTheDocument();
  });
});
