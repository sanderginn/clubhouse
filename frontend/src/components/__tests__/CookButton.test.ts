import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { get, writable } from 'svelte/store';
import { tick } from 'svelte';

type CookLogEntry = {
  id: string;
  userId: string;
  postId: string;
  rating: number;
  notes?: string | null;
  createdAt: string;
  updatedAt?: string | null;
  deletedAt?: string | null;
};

type RecipeState = {
  cookLogs: CookLogEntry[];
  error: string | null;
};

const state = writable<RecipeState>({
  cookLogs: [],
  error: null,
});

const setError = (error: string | null) =>
  state.update((current) => ({
    ...current,
    error,
  }));

const logCook = vi.fn(async (postId: string, rating: number, notes?: string) => {
  state.update((current) => ({
    ...current,
    cookLogs: [
      {
        id: 'log-1',
        userId: 'user-1',
        postId,
        rating,
        notes: notes ?? null,
        createdAt: '2025-01-01T00:00:00Z',
        updatedAt: null,
        deletedAt: null,
      },
    ],
    error: null,
  }));
});

const updateCookLog = vi.fn(async (postId: string, rating: number, notes?: string) => {
  state.update((current) => ({
    ...current,
    cookLogs: current.cookLogs.map((log) =>
      log.postId === postId
        ? {
            ...log,
            rating,
            notes: notes ?? null,
            updatedAt: '2025-01-02T00:00:00Z',
          }
        : log
    ),
    error: null,
  }));
});

const removeCookLog = vi.fn(async (postId: string) => {
  state.update((current) => ({
    ...current,
    cookLogs: current.cookLogs.filter((log) => log.postId !== postId),
    error: null,
  }));
});

const setState = (nextState: Partial<RecipeState>) => {
  state.set({
    ...get(state),
    ...nextState,
  });
};

const resetState = () => {
  state.set({ cookLogs: [], error: null });
  logCook.mockClear();
  updateCookLog.mockClear();
  removeCookLog.mockClear();
};

vi.mock('../../stores/recipeStore', () => ({
  recipeStore: {
    subscribe: state.subscribe,
    setError,
    logCook,
    updateCookLog,
    removeCookLog,
  },
  __setState: setState,
  __resetState: resetState,
}));

const { default: CookButton } = await import('../recipes/CookButton.svelte');
const { __setState, __resetState } = await import('../../stores/recipeStore');

describe('CookButton', () => {
  afterEach(() => {
    cleanup();
    __resetState();
  });

  it('logs a cook and shows the cooked state', async () => {
    render(CookButton, { postId: 'post-1' });

    await fireEvent.click(screen.getByTestId('cook-button'));
    await fireEvent.click(screen.getByTestId('rating-star-4'));
    await fireEvent.input(screen.getByTestId('cook-notes'), {
      target: { value: 'Great flavor' },
    });
    await fireEvent.click(screen.getByTestId('cook-save'));

    expect(logCook).toHaveBeenCalledWith('post-1', 4, 'Great flavor');

    await tick();

    expect(screen.getByText('Cooked')).toBeInTheDocument();
  });

  it('prefills the modal for an existing cook log', async () => {
    __setState({
      cookLogs: [
        {
          id: 'log-2',
          userId: 'user-1',
          postId: 'post-2',
          rating: 5,
          notes: 'Needs more salt',
          createdAt: '2025-01-01T00:00:00Z',
          updatedAt: null,
          deletedAt: null,
        },
      ],
    });

    render(CookButton, { postId: 'post-2' });

    expect(screen.getByText('Cooked')).toBeInTheDocument();

    await fireEvent.click(screen.getByTestId('cook-edit'));

    const notes = screen.getByTestId('cook-notes') as HTMLTextAreaElement;
    expect(notes.value).toBe('Needs more salt');
    expect(screen.getByTestId('cook-remove')).toBeInTheDocument();
  });

  it('reverts optimistic state on error', async () => {
    logCook.mockImplementationOnce(async () => {
      setError('Failed to log cook');
    });

    render(CookButton, { postId: 'post-3' });

    await fireEvent.click(screen.getByTestId('cook-button'));
    await fireEvent.click(screen.getByTestId('rating-star-3'));
    await fireEvent.click(screen.getByTestId('cook-save'));

    await tick();

    expect(screen.getByText('Failed to log cook')).toBeInTheDocument();
    expect(screen.getByText('I cooked this')).toBeInTheDocument();
  });
});
