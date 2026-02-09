import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { get, writable } from 'svelte/store';
import { tick } from 'svelte';

type WatchLogEntry = {
  id: string;
  userId: string;
  postId: string;
  rating: number;
  notes?: string;
  watchedAt: string;
};

type MovieState = {
  watchLogs: WatchLogEntry[];
  error: string | null;
};

const state = writable<MovieState>({
  watchLogs: [],
  error: null,
});

const setError = (error: string | null) =>
  state.update((current) => ({
    ...current,
    error,
  }));

const logWatch = vi.fn(async (postId: string, rating: number, notes?: string) => {
  state.update((current) => ({
    ...current,
    watchLogs: [
      {
        id: 'watch-1',
        userId: 'user-1',
        postId,
        rating,
        notes,
        watchedAt: '2026-01-01T00:00:00Z',
      },
    ],
    error: null,
  }));
});

const updateWatchLog = vi.fn(async (postId: string, rating?: number, notes?: string) => {
  state.update((current) => ({
    ...current,
    watchLogs: current.watchLogs.map((log) =>
      log.postId === postId
        ? {
            ...log,
            rating: rating ?? log.rating,
            notes: notes ?? log.notes,
            watchedAt: '2026-01-02T00:00:00Z',
          }
        : log
    ),
    error: null,
  }));
});

const removeWatchLog = vi.fn(async (postId: string) => {
  state.update((current) => ({
    ...current,
    watchLogs: current.watchLogs.filter((log) => log.postId !== postId),
    error: null,
  }));
});

const setState = (nextState: Partial<MovieState>) => {
  state.set({
    ...get(state),
    ...nextState,
  });
};

const resetState = () => {
  state.set({ watchLogs: [], error: null });
  logWatch.mockClear();
  updateWatchLog.mockClear();
  removeWatchLog.mockClear();
};

vi.mock('../../stores/movieStore', () => ({
  movieStore: {
    subscribe: state.subscribe,
    setError,
    logWatch,
    updateWatchLog,
    removeWatchLog,
  },
  __setState: setState,
  __resetState: resetState,
}));

const { default: WatchButton } = await import('../movies/WatchButton.svelte');
const { __setState, __resetState } = await import('../../stores/movieStore');

describe('WatchButton', () => {
  afterEach(() => {
    cleanup();
    __resetState();
  });

  it('renders initial not-watched state', () => {
    render(WatchButton, { postId: 'post-1', watchCount: 2 });

    expect(screen.getByTestId('watch-button')).toBeInTheDocument();
    expect(screen.getByText('Mark Watched')).toBeInTheDocument();
    expect(screen.getByTestId('watch-count')).toHaveTextContent('2 watches');
  });

  it('renders initial watched state with rating', () => {
    render(WatchButton, {
      postId: 'post-2',
      initialWatched: true,
      initialRating: 4,
    });

    expect(screen.getByTestId('watched-button')).toBeInTheDocument();
    expect(screen.getByText('Watched ★4')).toBeInTheDocument();
  });

  it('opens modal when button is clicked', async () => {
    render(WatchButton, { postId: 'post-3' });

    await fireEvent.click(screen.getByTestId('watch-button'));

    expect(screen.getByTestId('watch-modal')).toBeInTheDocument();
    expect(screen.getByText('Rate this movie')).toBeInTheDocument();
  });

  it('selects rating and saves a new watch log', async () => {
    render(WatchButton, { postId: 'post-4' });

    await fireEvent.click(screen.getByTestId('watch-button'));
    await fireEvent.click(screen.getByTestId('rating-star-5'));
    await fireEvent.input(screen.getByTestId('watch-notes'), {
      target: { value: 'Great ending' },
    });
    await fireEvent.click(screen.getByTestId('watch-save'));

    expect(logWatch).toHaveBeenCalledWith('post-4', 5, 'Great ending');

    await tick();

    expect(screen.queryByTestId('watch-modal')).not.toBeInTheDocument();
    expect(screen.getByText('Watched ★5')).toBeInTheDocument();
  });

  it('updates an existing watch log rating', async () => {
    __setState({
      watchLogs: [
        {
          id: 'watch-2',
          userId: 'user-1',
          postId: 'post-5',
          rating: 2,
          notes: 'Too slow',
          watchedAt: '2026-01-01T00:00:00Z',
        },
      ],
    });

    render(WatchButton, { postId: 'post-5' });

    expect(screen.getByText('Watched ★2')).toBeInTheDocument();

    await fireEvent.click(screen.getByTestId('watched-button'));

    const notes = screen.getByTestId('watch-notes') as HTMLTextAreaElement;
    expect(notes.value).toBe('Too slow');

    await fireEvent.click(screen.getByTestId('rating-star-4'));
    await fireEvent.click(screen.getByTestId('watch-save'));

    expect(updateWatchLog).toHaveBeenCalledWith('post-5', 4, 'Too slow');

    await tick();

    expect(screen.getByText('Watched ★4')).toBeInTheDocument();
  });

  it('removes an existing watch log', async () => {
    __setState({
      watchLogs: [
        {
          id: 'watch-3',
          userId: 'user-1',
          postId: 'post-6',
          rating: 3,
          watchedAt: '2026-01-01T00:00:00Z',
        },
      ],
    });

    render(WatchButton, { postId: 'post-6' });

    await fireEvent.click(screen.getByTestId('watched-button'));
    await fireEvent.click(screen.getByTestId('watch-remove'));

    expect(removeWatchLog).toHaveBeenCalledWith('post-6');

    await tick();

    expect(screen.getByText('Mark Watched')).toBeInTheDocument();
  });

  it('stays not watched after removing a prop-initialized watch log', async () => {
    render(WatchButton, {
      postId: 'post-6b',
      initialWatched: true,
      initialRating: 4,
    });

    expect(screen.getByText('Watched ★4')).toBeInTheDocument();

    await fireEvent.click(screen.getByTestId('watched-button'));
    await fireEvent.click(screen.getByTestId('watch-remove'));

    expect(removeWatchLog).toHaveBeenCalledWith('post-6b');

    await tick();

    expect(screen.getByText('Mark Watched')).toBeInTheDocument();
    expect(screen.queryByText('Watched ★4')).not.toBeInTheDocument();
  });

  it('shows a toast on save error and keeps modal open', async () => {
    logWatch.mockImplementationOnce(async () => {
      setError('Failed to log watch');
    });

    render(WatchButton, { postId: 'post-7' });

    await fireEvent.click(screen.getByTestId('watch-button'));
    await fireEvent.click(screen.getByTestId('rating-star-3'));
    await fireEvent.click(screen.getByTestId('watch-save'));

    await tick();

    expect(screen.getByTestId('watch-toast')).toHaveTextContent('Failed to log watch');
    expect(screen.getByTestId('watch-modal')).toBeInTheDocument();
    expect(screen.getByText('Mark Watched')).toBeInTheDocument();
  });
});
