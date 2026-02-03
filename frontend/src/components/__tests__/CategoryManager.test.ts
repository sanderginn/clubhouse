import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/svelte';
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest';
import type { RecipeCategory, SavedRecipe } from '../../stores/recipeStore';
import { recipeStore } from '../../stores/recipeStore';

const { default: CategoryManager } = await import('../recipes/CategoryManager.svelte');

const categories: RecipeCategory[] = [
  {
    id: 'cat-1',
    userId: 'user-1',
    name: 'Favorites',
    position: 1,
    createdAt: '2025-01-01T00:00:00Z',
  },
  {
    id: 'cat-2',
    userId: 'user-1',
    name: 'Quick',
    position: 2,
    createdAt: '2025-01-01T00:00:00Z',
  },
];

const savedRecipes = new Map<string, SavedRecipe[]>([
  [
    'Favorites',
    [
      {
        id: 'save-1',
        userId: 'user-1',
        postId: 'post-1',
        category: 'Favorites',
        createdAt: '2025-01-01T00:00:00Z',
      },
      {
        id: 'save-2',
        userId: 'user-1',
        postId: 'post-2',
        category: 'Favorites',
        createdAt: '2025-01-02T00:00:00Z',
      },
    ],
  ],
  [
    'Quick',
    [
      {
        id: 'save-3',
        userId: 'user-1',
        postId: 'post-3',
        category: 'Quick',
        createdAt: '2025-01-03T00:00:00Z',
      },
    ],
  ],
]);

beforeEach(() => {
  recipeStore.reset();
  recipeStore.setCategories(categories);
  recipeStore.setSavedRecipes(new Map(savedRecipes));
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  recipeStore.reset();
});

describe('CategoryManager', () => {
  it('renders categories in order with counts', () => {
    render(CategoryManager);

    const rows = screen.getAllByTestId('category-row');
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent('Favorites');
    expect(rows[0]).toHaveTextContent('2');
    expect(rows[1]).toHaveTextContent('Quick');
    expect(rows[1]).toHaveTextContent('1');
  });

  it('creates a category from the add flow', async () => {
    const createSpy = vi.spyOn(recipeStore, 'createCategory').mockResolvedValue();

    render(CategoryManager);

    await fireEvent.click(screen.getByTestId('add-category'));
    await fireEvent.input(screen.getByTestId('category-add-input'), {
      target: { value: 'New Category' },
    });
    await fireEvent.click(screen.getByTestId('category-add-save'));

    expect(createSpy).toHaveBeenCalledWith('New Category');
  });

  it('renames a category from the edit flow', async () => {
    const updateSpy = vi.spyOn(recipeStore, 'updateCategory').mockResolvedValue();

    render(CategoryManager);

    await fireEvent.click(screen.getByTestId('category-edit-cat-1'));
    await fireEvent.input(screen.getByTestId('category-edit-input'), {
      target: { value: 'Top Picks' },
    });
    await fireEvent.click(screen.getByTestId('category-edit-save'));

    expect(updateSpy).toHaveBeenCalledWith('cat-1', 'Top Picks');
  });

  it('confirms before deleting a category', async () => {
    const deleteSpy = vi.spyOn(recipeStore, 'deleteCategory').mockResolvedValue();

    render(CategoryManager);

    await fireEvent.click(screen.getByTestId('category-delete-cat-2'));
    expect(screen.getByTestId('category-delete-confirm')).toBeInTheDocument();
    expect(screen.getByTestId('category-delete-destination')).toHaveValue('Uncategorized');

    await fireEvent.click(screen.getByTestId('category-delete-confirm-button'));

    expect(deleteSpy).toHaveBeenCalledWith('cat-2');
  });

  it('moves recipes to a selected destination before deleting', async () => {
    const saveSpy = vi.spyOn(recipeStore, 'saveRecipe').mockResolvedValue();
    const unsaveSpy = vi.spyOn(recipeStore, 'unsaveRecipe').mockResolvedValue();
    const deleteSpy = vi.spyOn(recipeStore, 'deleteCategory').mockResolvedValue();

    render(CategoryManager);

    await fireEvent.click(screen.getByTestId('category-delete-cat-1'));
    await fireEvent.change(screen.getByTestId('category-delete-destination'), {
      target: { value: 'Quick' },
    });
    await fireEvent.click(screen.getByTestId('category-delete-confirm-button'));

    await waitFor(() => {
      expect(saveSpy).toHaveBeenCalledWith('post-1', ['Quick']);
      expect(saveSpy).toHaveBeenCalledWith('post-2', ['Quick']);
      expect(unsaveSpy).toHaveBeenCalledWith('post-1', 'Favorites');
      expect(unsaveSpy).toHaveBeenCalledWith('post-2', 'Favorites');
      expect(deleteSpy).toHaveBeenCalledWith('cat-1');
    });
  });

  it('reorders categories with the move buttons', async () => {
    const updateSpy = vi.spyOn(recipeStore, 'updateCategory').mockResolvedValue();

    render(CategoryManager);

    await fireEvent.click(screen.getByTestId('category-move-down-cat-1'));

    expect(updateSpy).toHaveBeenCalledWith('cat-1', undefined, 2);
    expect(updateSpy).toHaveBeenCalledWith('cat-2', undefined, 1);
  });
});
