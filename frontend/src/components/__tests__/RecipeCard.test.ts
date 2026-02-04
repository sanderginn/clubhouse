import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import { describe, it, expect, afterEach, vi, beforeEach } from 'vitest';
import type { RecipeMetadata } from '../../stores/postStore';

const { default: RecipeCard } = await import('../recipes/RecipeCard.svelte');

const recipe: RecipeMetadata = {
  name: 'Lemon Pasta',
  description: 'Bright and simple.',
  prep_time: '10m',
  cook_time: '15m',
  yield: '4',
  ingredients: ['1 lemon', '200g pasta'],
  instructions: ['Boil pasta.', 'Toss with lemon.'],
  nutrition: {
    calories: '320',
    servings: '4 bowls',
  },
};

beforeEach(() => {
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true,
  });
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('RecipeCard', () => {
  it('renders collapsed summary by default', () => {
    render(RecipeCard, { recipe });

    expect(screen.getByTestId('recipe-title')).toHaveTextContent('Lemon Pasta');
    expect(screen.getByTestId('recipe-time')).toHaveTextContent('Prep: 10m');
    expect(screen.getByTestId('recipe-yield')).toHaveTextContent('Serves 4');
    expect(screen.getByTestId('recipe-toggle')).toHaveTextContent('View Recipe');
    expect(screen.queryByText('Ingredients')).not.toBeInTheDocument();
  });

  it('expands to show ingredients, instructions, and checkboxes', async () => {
    render(RecipeCard, { recipe });

    await fireEvent.click(screen.getByTestId('recipe-toggle'));

    expect(screen.getByText('Ingredients')).toBeInTheDocument();
    expect(screen.getByText('Instructions')).toBeInTheDocument();

    const checkbox = screen.getByLabelText('1 lemon');
    expect(checkbox).not.toBeChecked();
    await fireEvent.click(checkbox);
    expect(checkbox).toBeChecked();
  });

  it('copies ingredients to clipboard when requested', async () => {
    render(RecipeCard, { recipe });

    await fireEvent.click(screen.getByTestId('recipe-toggle'));
    await fireEvent.click(screen.getByTestId('recipe-copy'));

    const clipboard = navigator.clipboard as { writeText: (value: string) => Promise<void> };
    expect(clipboard.writeText).toHaveBeenCalledWith('1 lemon\n200g pasta');
  });

});
