import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';

const apiSaveRecipe = vi.hoisted(() => vi.fn());
const apiUnsaveRecipe = vi.hoisted(() => vi.fn());
const apiLogCook = vi.hoisted(() => vi.fn());
const apiUpdateCookLog = vi.hoisted(() => vi.fn());
const apiRemoveCookLog = vi.hoisted(() => vi.fn());
const apiGetSavedRecipes = vi.hoisted(() => vi.fn());
const apiGetRecipeCategories = vi.hoisted(() => vi.fn());
const apiCreateCategory = vi.hoisted(() => vi.fn());
const apiUpdateCategory = vi.hoisted(() => vi.fn());
const apiDeleteCategory = vi.hoisted(() => vi.fn());
const apiGetCookLogs = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    saveRecipe: apiSaveRecipe,
    unsaveRecipe: apiUnsaveRecipe,
    logCook: apiLogCook,
    updateCookLog: apiUpdateCookLog,
    removeCookLog: apiRemoveCookLog,
    getMySavedRecipes: apiGetSavedRecipes,
    getMyRecipeCategories: apiGetRecipeCategories,
    createRecipeCategory: apiCreateCategory,
    updateRecipeCategory: apiUpdateCategory,
    deleteRecipeCategory: apiDeleteCategory,
    getMyCookLogs: apiGetCookLogs,
  },
}));

const {
  recipeStore,
  savedRecipesByCategory,
  sortedCategories,
  handleRecipeSavedEvent,
  handleRecipeCategoryUpdatedEvent,
  handleRecipeCategoryDeletedEvent,
  handleCookLogRemovedEvent,
} = await import('../recipeStore');

beforeEach(() => {
  recipeStore.reset();
  apiSaveRecipe.mockReset();
  apiUnsaveRecipe.mockReset();
  apiLogCook.mockReset();
  apiUpdateCookLog.mockReset();
  apiRemoveCookLog.mockReset();
  apiGetSavedRecipes.mockReset();
  apiGetRecipeCategories.mockReset();
  apiCreateCategory.mockReset();
  apiUpdateCategory.mockReset();
  apiDeleteCategory.mockReset();
  apiGetCookLogs.mockReset();
});

describe('recipeStore', () => {
  it('loadSavedRecipes populates saved recipes map', async () => {
    apiGetSavedRecipes.mockResolvedValue({
      categories: [
        {
          name: 'Favorites',
          recipes: [
            {
              id: 'save-1',
              user_id: 'user-1',
              post_id: 'post-1',
              category: 'Favorites',
              created_at: '2024-01-01T00:00:00Z',
              post: {
                id: 'post-1',
                user_id: 'user-1',
                section_id: 'section-1',
                content: 'Recipe 1',
                created_at: '2024-01-01T00:00:00Z',
              },
            },
          ],
        },
      ],
    });

    await recipeStore.loadSavedRecipes();
    const state = get(recipeStore);
    const favorites = state.savedRecipes.get('Favorites') ?? [];

    expect(state.isLoadingSaved).toBe(false);
    expect(favorites).toHaveLength(1);
    expect(favorites[0].post?.id).toBe('post-1');
  });

  it('saveRecipe merges new saved recipes into map', async () => {
    apiSaveRecipe.mockResolvedValue({
      saved_recipes: [
        {
          id: 'save-2',
          user_id: 'user-1',
          post_id: 'post-2',
          category: 'Favorites',
          created_at: '2024-01-02T00:00:00Z',
        },
      ],
    });

    await recipeStore.saveRecipe('post-2', ['Favorites']);
    const state = get(recipeStore);
    const favorites = state.savedRecipes.get('Favorites') ?? [];

    expect(favorites).toHaveLength(1);
    expect(favorites[0].postId).toBe('post-2');
  });

  it('unsaveRecipe removes saved recipes from category', async () => {
    apiUnsaveRecipe.mockResolvedValue(undefined);

    recipeStore.applySavedRecipes([
      {
        id: 'save-3',
        userId: 'user-1',
        postId: 'post-3',
        category: 'Favorites',
        createdAt: '2024-01-01T00:00:00Z',
      },
    ]);

    await recipeStore.unsaveRecipe('post-3', 'Favorites');
    const state = get(recipeStore);

    expect(state.savedRecipes.get('Favorites')).toBeUndefined();
  });

  it('updateCategory renames saved recipe categories', async () => {
    apiUpdateCategory.mockResolvedValue({
      category: {
        id: 'cat-1',
        user_id: 'user-1',
        name: 'Top Picks',
        position: 1,
        created_at: '2024-01-01T00:00:00Z',
      },
    });

    recipeStore.setCategories([
      { id: 'cat-1', userId: 'user-1', name: 'Favorites', position: 1, createdAt: 'now' },
    ]);
    recipeStore.applySavedRecipes([
      {
        id: 'save-4',
        userId: 'user-1',
        postId: 'post-4',
        category: 'Favorites',
        createdAt: '2024-01-01T00:00:00Z',
      },
    ]);

    await recipeStore.updateCategory('cat-1', 'Top Picks');
    const state = get(recipeStore);

    expect(state.savedRecipes.get('Favorites')).toBeUndefined();
    const renamed = state.savedRecipes.get('Top Picks') ?? [];
    expect(renamed).toHaveLength(1);
    expect(renamed[0].category).toBe('Top Picks');
  });

  it('deleteCategory moves recipes to Uncategorized', async () => {
    apiDeleteCategory.mockResolvedValue(undefined);

    recipeStore.setCategories([
      { id: 'cat-2', userId: 'user-1', name: 'Weeknight', position: 2, createdAt: 'now' },
    ]);
    recipeStore.applySavedRecipes([
      {
        id: 'save-5',
        userId: 'user-1',
        postId: 'post-5',
        category: 'Weeknight',
        createdAt: '2024-01-01T00:00:00Z',
      },
    ]);

    await recipeStore.deleteCategory('cat-2');
    const state = get(recipeStore);
    const uncategorized = state.savedRecipes.get('Uncategorized') ?? [];

    expect(state.categories).toHaveLength(0);
    expect(uncategorized).toHaveLength(1);
    expect(uncategorized[0].category).toBe('Uncategorized');
  });

  it('loadCookLogs populates cook logs', async () => {
    apiGetCookLogs.mockResolvedValue({
      cook_logs: [
        {
          id: 'log-1',
          user_id: 'user-1',
          post_id: 'post-1',
          rating: 4,
          notes: 'Nice',
          created_at: '2024-01-03T00:00:00Z',
          post: {
            id: 'post-1',
            user_id: 'user-1',
            section_id: 'section-1',
            content: 'Recipe',
            created_at: '2024-01-01T00:00:00Z',
          },
        },
      ],
    });

    await recipeStore.loadCookLogs();
    const state = get(recipeStore);

    expect(state.cookLogs).toHaveLength(1);
    expect(state.cookLogs[0].post?.id).toBe('post-1');
  });

  it('derived stores expose sorted categories and map', () => {
    recipeStore.setCategories([
      { id: 'cat-1', userId: 'user-1', name: 'Zed', position: 2, createdAt: 'now' },
      { id: 'cat-2', userId: 'user-1', name: 'Alpha', position: 1, createdAt: 'now' },
    ]);

    const sorted = get(sortedCategories);
    const map = get(savedRecipesByCategory);

    expect(sorted[0].name).toBe('Alpha');
    expect(map).toBeInstanceOf(Map);
  });

  it('realtime handlers apply events', () => {
    handleRecipeSavedEvent({
      saved_recipe: {
        id: 'save-10',
        user_id: 'user-1',
        post_id: 'post-10',
        category: 'Favorites',
        created_at: '2024-01-01T00:00:00Z',
      },
    });

    handleCookLogRemovedEvent({ post_id: 'post-10' });

    const state = get(recipeStore);
    const favorites = state.savedRecipes.get('Favorites') ?? [];
    expect(favorites).toHaveLength(1);
  });

  it('realtime category updates migrate saved recipe map keys', () => {
    recipeStore.setCategories([
      { id: 'cat-10', userId: 'user-1', name: 'Favorites', position: 1, createdAt: 'now' },
    ]);
    recipeStore.applySavedRecipes([
      {
        id: 'save-20',
        userId: 'user-1',
        postId: 'post-20',
        category: 'Favorites',
        createdAt: '2024-01-01T00:00:00Z',
      },
    ]);

    handleRecipeCategoryUpdatedEvent({
      category: {
        id: 'cat-10',
        user_id: 'user-1',
        name: 'Top Picks',
        position: 1,
        created_at: '2024-01-01T00:00:00Z',
      },
    });

    const state = get(recipeStore);
    expect(state.savedRecipes.get('Favorites')).toBeUndefined();
    const renamed = state.savedRecipes.get('Top Picks') ?? [];
    expect(renamed).toHaveLength(1);
  });

  it('realtime category delete supports category_id', () => {
    recipeStore.setCategories([
      { id: 'cat-11', userId: 'user-1', name: 'Weeknight', position: 2, createdAt: 'now' },
    ]);
    recipeStore.applySavedRecipes([
      {
        id: 'save-21',
        userId: 'user-1',
        postId: 'post-21',
        category: 'Weeknight',
        createdAt: '2024-01-01T00:00:00Z',
      },
    ]);

    handleRecipeCategoryDeletedEvent({ category_id: 'cat-11', category_name: 'Weeknight' });

    const state = get(recipeStore);
    const uncategorized = state.savedRecipes.get('Uncategorized') ?? [];
    expect(uncategorized).toHaveLength(1);
  });
});
