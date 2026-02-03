import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { tick } from 'svelte';

const apiSaveRecipe = vi.hoisted(() => vi.fn());
const apiUnsaveRecipe = vi.hoisted(() => vi.fn());
const apiCreateRecipeCategory = vi.hoisted(() => vi.fn());
const apiGetMySavedRecipes = vi.hoisted(() => vi.fn());
const apiGetMyRecipeCategories = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    saveRecipe: apiSaveRecipe,
    unsaveRecipe: apiUnsaveRecipe,
    createRecipeCategory: apiCreateRecipeCategory,
    getMySavedRecipes: apiGetMySavedRecipes,
    getMyRecipeCategories: apiGetMyRecipeCategories,
  },
}));

const { recipeStore } = await import('../../stores/recipeStore');
const { authStore } = await import('../../stores/authStore');
const { default: RecipeSaveButton } = await import('../recipes/RecipeSaveButton.svelte');

beforeEach(() => {
  recipeStore.reset();
  authStore.setUser({
    id: 'user-1',
    username: 'cook',
    email: 'cook@example.com',
    isAdmin: false,
    totpEnabled: false,
  });
  apiSaveRecipe.mockReset();
  apiUnsaveRecipe.mockReset();
  apiCreateRecipeCategory.mockReset();
  apiGetMySavedRecipes.mockReset();
  apiGetMyRecipeCategories.mockReset();
  apiGetMySavedRecipes.mockResolvedValue({ categories: [] });
  apiGetMyRecipeCategories.mockResolvedValue({ categories: [] });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('RecipeSaveButton', () => {
  it('renders the unsaved state and opens the dropdown', async () => {
    render(RecipeSaveButton, { postId: 'post-1' });

    expect(screen.getByText('Add to my recipes')).toBeInTheDocument();

    await fireEvent.click(screen.getByText('Add to my recipes'));

    expect(screen.getByText('Save to categories')).toBeInTheDocument();
    expect(screen.getByText(/Create new category/)).toBeInTheDocument();
  });

  it('shows saved state with category count', () => {
    const map = new Map();
    map.set('Favorites', [
      {
        id: 'save-1',
        userId: 'user-1',
        postId: 'post-1',
        category: 'Favorites',
        createdAt: '2026-01-01T00:00:00Z',
      },
    ]);
    recipeStore.setSavedRecipes(map);

    render(RecipeSaveButton, { postId: 'post-1' });

    expect(screen.getByText('Saved')).toBeInTheDocument();
    expect(screen.getByTestId('recipe-save-count')).toHaveTextContent('1');
  });

  it('saves selected categories when applying', async () => {
    recipeStore.setCategories([
      {
        id: 'cat-1',
        userId: 'user-1',
        name: 'Favorites',
        position: 1,
        createdAt: '2026-01-01T00:00:00Z',
      },
    ]);
    apiSaveRecipe.mockResolvedValue({
      saved_recipes: [
        {
          id: 'save-2',
          user_id: 'user-1',
          post_id: 'post-2',
          category: 'Favorites',
          created_at: '2026-01-01T00:00:00Z',
        },
      ],
    });

    render(RecipeSaveButton, { postId: 'post-2' });

    await fireEvent.click(screen.getByText('Add to my recipes'));
    await fireEvent.click(screen.getByLabelText('Favorites'));
    await fireEvent.click(screen.getByText('Apply'));
    await tick();

    expect(apiSaveRecipe).toHaveBeenCalledWith('post-2', ['Favorites']);
    expect(screen.getByText('Saved')).toBeInTheDocument();
  });
});
