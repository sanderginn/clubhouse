import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';

const { default: RecipeStatsBar } = await import('../recipes/RecipeStatsBar.svelte');

describe('RecipeStatsBar', () => {
  it('renders nothing when stats are empty and showEmpty is false', () => {
    const { container, queryByTestId } = render(RecipeStatsBar, {
      saveCount: 0,
      cookCount: 0,
      averageRating: null,
      showEmpty: false,
    });

    expect(queryByTestId('recipe-stats-bar')).toBeNull();
    expect(container.textContent?.trim()).toBe('');
  });

  it('renders stats when counts are provided', () => {
    render(RecipeStatsBar, {
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
    expect(screen.getByText('avg')).toBeInTheDocument();
  });

  it('shows empty state when showEmpty is true', () => {
    render(RecipeStatsBar, {
      saveCount: 0,
      cookCount: 0,
      averageRating: null,
      showEmpty: true,
    });

    expect(screen.getByText('No recipe stats yet')).toBeInTheDocument();
  });
});
