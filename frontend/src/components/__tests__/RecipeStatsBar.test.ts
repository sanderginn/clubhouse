import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import { describe, it, expect, vi, afterEach } from 'vitest';

const apiGetPostSaves = vi.hoisted(() =>
  vi.fn().mockResolvedValue({
    save_count: 1,
    users: [{ id: 'user-1', username: 'sander', profile_picture_url: null }],
    viewer_saved: false,
  })
);
const apiGetPostCookLogs = vi.hoisted(() =>
  vi.fn().mockResolvedValue({
    cook_count: 1,
    avg_rating: 4.2,
    users: [
      {
        id: 'user-2',
        username: 'alex',
        profile_picture_url: null,
        rating: 4.5,
        created_at: '2026-02-01T00:00:00Z',
      },
    ],
    viewer_cooked: false,
  })
);

vi.mock('../../services/api', () => ({
  api: {
    getPostSaves: apiGetPostSaves,
    getPostCookLogs: apiGetPostCookLogs,
  },
}));

vi.mock('../recipes/RecipeSaveButton.svelte', async () => ({
  default: (await import('./RecipeSaveButtonStub.svelte')).default,
}));

vi.mock('../recipes/CookButton.svelte', async () => ({
  default: (await import('./CookButtonStub.svelte')).default,
}));

const { default: RecipeStatsBar } = await import('../recipes/RecipeStatsBar.svelte');

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('RecipeStatsBar', () => {
  it('renders stats and action buttons', () => {
    render(RecipeStatsBar, {
      postId: 'post-1',
      saveCount: 12,
      cookCount: 4,
      averageRating: 4.3,
    });

    expect(screen.getByTestId('recipe-stats-bar')).toBeInTheDocument();
    expect(screen.getByText('12')).toBeInTheDocument();
    expect(screen.getByText('saved')).toBeInTheDocument();
    expect(screen.getByText('4')).toBeInTheDocument();
    expect(screen.getByText('cooked')).toBeInTheDocument();
    expect(screen.getByText('4.3')).toBeInTheDocument();
    expect(screen.getByTestId('recipe-save-button')).toBeInTheDocument();
    expect(screen.getByTestId('cook-button')).toBeInTheDocument();
  });

  it('loads save tooltip on hover', async () => {
    vi.useFakeTimers();
    render(RecipeStatsBar, {
      postId: 'post-1',
      saveCount: 1,
      cookCount: 0,
      averageRating: null,
    });

    const target = screen.getByTestId('recipe-save-stat');
    await fireEvent.mouseEnter(target);
    await vi.runAllTimersAsync();

    expect(apiGetPostSaves).toHaveBeenCalledWith('post-1');
    expect(await screen.findByText('sander')).toBeInTheDocument();

    vi.useRealTimers();
  });

  it('loads cook tooltip on hover', async () => {
    vi.useFakeTimers();
    render(RecipeStatsBar, {
      postId: 'post-1',
      saveCount: 0,
      cookCount: 1,
      averageRating: 4.2,
    });

    const target = screen.getByTestId('recipe-cook-stat');
    await fireEvent.mouseEnter(target);
    await vi.runAllTimersAsync();

    expect(apiGetPostCookLogs).toHaveBeenCalledWith('post-1');
    expect(await screen.findByText('alex')).toBeInTheDocument();
    expect(screen.getByText('4.5')).toBeInTheDocument();

    vi.useRealTimers();
  });
});
