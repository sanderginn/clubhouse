import { derived, get, writable } from 'svelte/store';
import {
  api,
  type SavedRecipe as ApiSavedRecipe,
  type SavedRecipeCategory as ApiSavedRecipeCategory,
  type RecipeCategory as ApiRecipeCategory,
  type CookLog as ApiCookLog,
  type CookLogWithPost as ApiCookLogWithPost,
} from '../services/api';
import { mapApiPost, type ApiPost } from './postMapper';
import { postStore, type Post } from './postStore';

const DEFAULT_RECIPE_CATEGORY = 'Uncategorized';

export interface SavedRecipe {
  id: string;
  userId: string;
  postId: string;
  category: string;
  createdAt: string;
  deletedAt?: string | null;
  post?: Post;
}

export interface RecipeCategory {
  id: string;
  userId: string;
  name: string;
  position: number;
  createdAt: string;
}

export interface CookLog {
  id: string;
  userId: string;
  postId: string;
  rating: number;
  notes?: string | null;
  createdAt: string;
  updatedAt?: string | null;
  deletedAt?: string | null;
  post?: Post;
}

export interface RecipeState {
  savedRecipes: Map<string, SavedRecipe[]>;
  categories: RecipeCategory[];
  cookLogs: CookLog[];
  isLoadingSaved: boolean;
  isLoadingCategories: boolean;
  isLoadingCookLogs: boolean;
  error: string | null;
}

function mapApiRecipeCategory(category: ApiRecipeCategory): RecipeCategory {
  return {
    id: category.id,
    userId: category.user_id,
    name: category.name,
    position: category.position,
    createdAt: category.created_at,
  };
}

function mapApiSavedRecipe(recipe: ApiSavedRecipe): SavedRecipe {
  return {
    id: recipe.id,
    userId: recipe.user_id,
    postId: recipe.post_id,
    category: recipe.category,
    createdAt: recipe.created_at,
    deletedAt: recipe.deleted_at ?? undefined,
    post: recipe.post ? mapApiPost(recipe.post) : undefined,
  };
}

function mapApiCookLog(log: ApiCookLog | ApiCookLogWithPost): CookLog {
  const post = 'post' in log && log.post ? mapApiPost(log.post as ApiPost) : undefined;
  return {
    id: log.id,
    userId: log.user_id,
    postId: log.post_id,
    rating: log.rating,
    notes: log.notes ?? undefined,
    createdAt: log.created_at,
    updatedAt: log.updated_at ?? undefined,
    deletedAt: log.deleted_at ?? undefined,
    post,
  };
}

function buildSavedRecipesMap(categories: ApiSavedRecipeCategory[]): Map<string, SavedRecipe[]> {
  const entries = new Map<string, SavedRecipe[]>();
  for (const category of categories) {
    const recipes = (category.recipes ?? []).map(mapApiSavedRecipe);
    entries.set(category.name, recipes);
  }
  return entries;
}

function addSavedRecipeToMap(
  map: Map<string, SavedRecipe[]>,
  savedRecipe: SavedRecipe
): Map<string, SavedRecipe[]> {
  const next = new Map(map);
  const existing = next.get(savedRecipe.category) ?? [];
  const filtered = existing.filter((item) => item.id !== savedRecipe.id && item.postId !== savedRecipe.postId);
  next.set(savedRecipe.category, [...filtered, savedRecipe]);
  return next;
}

function getSavedCategoriesForPost(
  map: Map<string, SavedRecipe[]>,
  postId: string
): Set<string> {
  const categories = new Set<string>();
  for (const [category, recipes] of map.entries()) {
    if (recipes.some((recipe) => recipe.postId === postId)) {
      categories.add(category);
    }
  }
  return categories;
}

function removeSavedRecipeFromMap(
  map: Map<string, SavedRecipe[]>,
  postId: string,
  category?: string
): Map<string, SavedRecipe[]> {
  const next = new Map(map);
  if (category) {
    const existing = next.get(category) ?? [];
    const filtered = existing.filter((item) => item.postId !== postId);
    if (filtered.length > 0) {
      next.set(category, filtered);
    } else {
      next.delete(category);
    }
    return next;
  }

  for (const [key, recipes] of next.entries()) {
    const filtered = recipes.filter((item) => item.postId !== postId);
    if (filtered.length > 0) {
      next.set(key, filtered);
    } else {
      next.delete(key);
    }
  }

  return next;
}

function moveCategoryRecipes(
  map: Map<string, SavedRecipe[]>,
  fromCategory: string,
  toCategory: string
): Map<string, SavedRecipe[]> {
  if (fromCategory === toCategory) {
    return map;
  }

  const next = new Map(map);
  const existing = next.get(fromCategory) ?? [];
  next.delete(fromCategory);
  if (existing.length === 0) {
    return next;
  }

  const updated = existing.map((recipe) => ({ ...recipe, category: toCategory }));
  const target = next.get(toCategory) ?? [];
  next.set(toCategory, [...target, ...updated]);
  return next;
}

function upsertCookLog(logs: CookLog[], nextLog: CookLog): CookLog[] {
  const index = logs.findIndex((entry) => entry.postId === nextLog.postId);
  if (index === -1) {
    return [nextLog, ...logs];
  }
  const existing = logs[index];
  const merged: CookLog = {
    ...existing,
    ...nextLog,
    post: nextLog.post ?? existing.post,
  };
  const updated = [...logs];
  updated[index] = merged;
  return updated;
}

function extractSavedRecipes(payload: unknown): ApiSavedRecipe[] {
  if (!payload || typeof payload !== 'object') {
    return [];
  }
  const record = payload as Record<string, unknown>;
  if (Array.isArray(record.saved_recipes)) {
    return record.saved_recipes as ApiSavedRecipe[];
  }
  if (Array.isArray(record.savedRecipes)) {
    return record.savedRecipes as ApiSavedRecipe[];
  }
  if (record.saved_recipe) {
    return [record.saved_recipe as ApiSavedRecipe];
  }
  if (record.savedRecipe) {
    return [record.savedRecipe as ApiSavedRecipe];
  }
  return [];
}

function extractCookLog(payload: unknown): ApiCookLog | ApiCookLogWithPost | null {
  if (!payload || typeof payload !== 'object') {
    return null;
  }
  const record = payload as Record<string, unknown>;
  if (record.cook_log) {
    return record.cook_log as ApiCookLog;
  }
  if (record.cookLog) {
    return record.cookLog as ApiCookLog;
  }
  if (record.log && typeof record.log === 'object') {
    return record.log as ApiCookLog;
  }
  return null;
}

function extractPostId(payload: unknown): string | null {
  if (!payload || typeof payload !== 'object') {
    return null;
  }
  const record = payload as Record<string, unknown>;
  const postId = record.post_id ?? record.postId;
  return typeof postId === 'string' && postId.length > 0 ? postId : null;
}

function extractCategoryName(payload: unknown): string | null {
  if (!payload || typeof payload !== 'object') {
    return null;
  }
  const record = payload as Record<string, unknown>;
  const category = record.category ?? record.categoryName ?? record.category_name;
  return typeof category === 'string' && category.length > 0 ? category : null;
}

function extractRecipeCategory(payload: unknown): ApiRecipeCategory | null {
  if (!payload || typeof payload !== 'object') {
    return null;
  }
  const record = payload as Record<string, unknown>;
  if (record.category && typeof record.category === 'object') {
    return record.category as ApiRecipeCategory;
  }
  if (record.recipe_category && typeof record.recipe_category === 'object') {
    return record.recipe_category as ApiRecipeCategory;
  }
  if (record.recipeCategory && typeof record.recipeCategory === 'object') {
    return record.recipeCategory as ApiRecipeCategory;
  }
  return null;
}

function extractCategoryId(payload: unknown): string | null {
  if (!payload || typeof payload !== 'object') {
    return null;
  }
  const record = payload as Record<string, unknown>;
  const categoryId = record.category_id ?? record.categoryId ?? record.id;
  return typeof categoryId === 'string' && categoryId.length > 0 ? categoryId : null;
}

async function refreshPostSaveCount(postId: string): Promise<void> {
  try {
    const response = await api.getPostSaves(postId);
    postStore.setRecipeSaveCount(postId, response.save_count);
  } catch {
    // Ignore transient failures; the next event or refresh will reconcile.
  }
}

const initialState: RecipeState = {
  savedRecipes: new Map(),
  categories: [],
  cookLogs: [],
  isLoadingSaved: false,
  isLoadingCategories: false,
  isLoadingCookLogs: false,
  error: null,
};

function createRecipeStore() {
  const { subscribe, update, set } = writable<RecipeState>({ ...initialState });

  return {
    subscribe,
    setSavedRecipes: (savedRecipes: Map<string, SavedRecipe[]>) =>
      update((state) => ({
        ...state,
        savedRecipes,
        isLoadingSaved: false,
        error: null,
      })),
    setCategories: (categories: RecipeCategory[]) =>
      update((state) => ({
        ...state,
        categories,
        isLoadingCategories: false,
        error: null,
      })),
    setCookLogs: (cookLogs: CookLog[]) =>
      update((state) => ({
        ...state,
        cookLogs,
        isLoadingCookLogs: false,
        error: null,
      })),
    setLoadingSaved: (isLoading: boolean) =>
      update((state) => ({
        ...state,
        isLoadingSaved: isLoading,
        error: isLoading ? null : state.error,
      })),
    setLoadingCategories: (isLoading: boolean) =>
      update((state) => ({
        ...state,
        isLoadingCategories: isLoading,
        error: isLoading ? null : state.error,
      })),
    setLoadingCookLogs: (isLoading: boolean) =>
      update((state) => ({
        ...state,
        isLoadingCookLogs: isLoading,
        error: isLoading ? null : state.error,
      })),
    setError: (error: string | null) =>
      update((state) => ({
        ...state,
        error,
        isLoadingSaved: false,
        isLoadingCategories: false,
        isLoadingCookLogs: false,
      })),
    reset: () => set({ ...initialState, savedRecipes: new Map() }),
    applySavedRecipes: (savedRecipes: SavedRecipe[]) =>
      update((state) => {
        let nextMap = state.savedRecipes;
        for (const recipe of savedRecipes) {
          nextMap = addSavedRecipeToMap(nextMap, recipe);
        }
        return {
          ...state,
          savedRecipes: nextMap,
          error: null,
        };
      }),
    applyUnsave: (postId: string, category?: string) =>
      update((state) => ({
        ...state,
        savedRecipes: removeSavedRecipeFromMap(state.savedRecipes, postId, category),
        error: null,
      })),
    applyCookLog: (cookLog: CookLog) =>
      update((state) => ({
        ...state,
        cookLogs: upsertCookLog(state.cookLogs, cookLog),
        error: null,
      })),
    applyCookLogRemoval: (postId: string) =>
      update((state) => ({
        ...state,
        cookLogs: state.cookLogs.filter((log) => log.postId !== postId),
        error: null,
      })),
    applyCategory: (category: RecipeCategory) =>
      update((state) => {
        const existingIndex = state.categories.findIndex((item) => item.id === category.id);
        const nextCategories = [...state.categories];
        if (existingIndex === -1) {
          nextCategories.push(category);
        } else {
          nextCategories[existingIndex] = { ...nextCategories[existingIndex], ...category };
        }
        return {
          ...state,
          categories: nextCategories,
          error: null,
        };
      }),
    applyCategoryDeletion: (categoryId: string, categoryName?: string) =>
      update((state) => {
        const existing = state.categories.find((item) => item.id === categoryId);
        const nameToDelete = categoryName ?? existing?.name ?? '';
        const nextCategories = state.categories.filter((item) => item.id !== categoryId);
        let nextMap = state.savedRecipes;
        if (nameToDelete) {
          nextMap = moveCategoryRecipes(nextMap, nameToDelete, DEFAULT_RECIPE_CATEGORY);
        }
        return {
          ...state,
          categories: nextCategories,
          savedRecipes: nextMap,
          error: null,
        };
      }),
    saveRecipe: async (postId: string, categories: string[]): Promise<void> => {
      try {
        const existing = getSavedCategoriesForPost(get(recipeStore).savedRecipes, postId);
        const normalized = categories
          .map((category) => category.trim())
          .filter((category) => category.length > 0);
        const unique = Array.from(new Set(normalized));
        const wasSaved = existing.size > 0;
        const willBeSaved = wasSaved || unique.length > 0;
        const response = await api.saveRecipe(postId, categories);
        const savedRecipes = (response.saved_recipes ?? []).map(mapApiSavedRecipe);
        recipeStore.applySavedRecipes(savedRecipes);
        if (!wasSaved && willBeSaved) {
          postStore.updateRecipeSaveCount(postId, 1);
        }
      } catch (error) {
        recipeStore.setError(error instanceof Error ? error.message : 'Failed to save recipe');
      }
    },
    unsaveRecipe: async (postId: string, category?: string): Promise<void> => {
      try {
        const existing = getSavedCategoriesForPost(get(recipeStore).savedRecipes, postId);
        const wasSaved = existing.size > 0;
        const remaining =
          category === undefined
            ? 0
            : existing.size - (existing.has(category) ? 1 : 0);
        const willBeSaved = remaining > 0;
        await api.unsaveRecipe(postId, category);
        recipeStore.applyUnsave(postId, category);
        if (wasSaved && !willBeSaved) {
          postStore.updateRecipeSaveCount(postId, -1);
        }
      } catch (error) {
        recipeStore.setError(error instanceof Error ? error.message : 'Failed to unsave recipe');
      }
    },
    logCook: async (postId: string, rating: number, notes?: string): Promise<void> => {
      try {
        const response = await api.logCook(postId, rating, notes);
        recipeStore.applyCookLog(mapApiCookLog(response.cook_log));
      } catch (error) {
        recipeStore.setError(error instanceof Error ? error.message : 'Failed to log cook');
      }
    },
    updateCookLog: async (postId: string, rating: number, notes?: string): Promise<void> => {
      try {
        const response = await api.updateCookLog(postId, rating, notes);
        recipeStore.applyCookLog(mapApiCookLog(response.cook_log));
      } catch (error) {
        recipeStore.setError(error instanceof Error ? error.message : 'Failed to update cook log');
      }
    },
    removeCookLog: async (postId: string): Promise<void> => {
      try {
        await api.removeCookLog(postId);
        recipeStore.applyCookLogRemoval(postId);
      } catch (error) {
        recipeStore.setError(error instanceof Error ? error.message : 'Failed to remove cook log');
      }
    },
    createCategory: async (name: string): Promise<void> => {
      try {
        const response = await api.createRecipeCategory(name);
        recipeStore.applyCategory(mapApiRecipeCategory(response.category));
      } catch (error) {
        recipeStore.setError(error instanceof Error ? error.message : 'Failed to create category');
      }
    },
    updateCategory: async (id: string, name?: string, position?: number): Promise<void> => {
      const state = get(recipeStore);
      const existing = state.categories.find((category) => category.id === id);
      try {
        const response = await api.updateRecipeCategory(id, { name, position });
        const updated = mapApiRecipeCategory(response.category);
        recipeStore.applyCategory(updated);
        if (existing && existing.name !== updated.name) {
          recipeStore.updateSavedRecipeCategory(existing.name, updated.name);
        }
      } catch (error) {
        recipeStore.setError(error instanceof Error ? error.message : 'Failed to update category');
      }
    },
    deleteCategory: async (id: string): Promise<void> => {
      const state = get(recipeStore);
      const existing = state.categories.find((category) => category.id === id);
      try {
        await api.deleteRecipeCategory(id);
        recipeStore.applyCategoryDeletion(id, existing?.name);
      } catch (error) {
        recipeStore.setError(error instanceof Error ? error.message : 'Failed to delete category');
      }
    },
    updateSavedRecipeCategory: (fromCategory: string, toCategory: string) =>
      update((state) => ({
        ...state,
        savedRecipes: moveCategoryRecipes(state.savedRecipes, fromCategory, toCategory),
      })),
    loadSavedRecipes: async (): Promise<void> => {
      recipeStore.setLoadingSaved(true);
      try {
        const response = await api.getMySavedRecipes();
        const savedRecipes = buildSavedRecipesMap(response.categories ?? []);
        recipeStore.setSavedRecipes(savedRecipes);
      } catch (error) {
        recipeStore.setError(
          error instanceof Error ? error.message : 'Failed to load saved recipes'
        );
      }
    },
    loadCategories: async (): Promise<void> => {
      recipeStore.setLoadingCategories(true);
      try {
        const response = await api.getMyRecipeCategories();
        const categories = (response.categories ?? []).map(mapApiRecipeCategory);
        recipeStore.setCategories(categories);
      } catch (error) {
        recipeStore.setError(
          error instanceof Error ? error.message : 'Failed to load recipe categories'
        );
      }
    },
    loadCookLogs: async (): Promise<void> => {
      recipeStore.setLoadingCookLogs(true);
      try {
        const response = await api.getMyCookLogs();
        const cookLogs = (response.cook_logs ?? []).map(mapApiCookLog);
        recipeStore.setCookLogs(cookLogs);
      } catch (error) {
        recipeStore.setError(error instanceof Error ? error.message : 'Failed to load cook logs');
      }
    },
  };
}

export const recipeStore = createRecipeStore();

export const savedRecipesByCategory = derived(recipeStore, ($store) => $store.savedRecipes);

export const sortedCategories = derived(recipeStore, ($store) =>
  [...$store.categories].sort((a, b) => {
    if (a.position !== b.position) {
      return a.position - b.position;
    }
    return a.name.localeCompare(b.name);
  })
);

export async function handleRecipeSavedEvent(payload: unknown): Promise<void> {
  const recipes = extractSavedRecipes(payload).map(mapApiSavedRecipe);
  if (recipes.length > 0) {
    recipeStore.applySavedRecipes(recipes);
  }
  const postId = extractPostId(payload);
  if (postId) {
    await refreshPostSaveCount(postId);
  }
}

export async function handleRecipeUnsavedEvent(payload: unknown): Promise<void> {
  const postId = extractPostId(payload);
  if (!postId) {
    return;
  }
  const category = extractCategoryName(payload) ?? undefined;
  recipeStore.applyUnsave(postId, category);
  await refreshPostSaveCount(postId);
}

export function handleCookLogCreatedEvent(payload: unknown): void {
  const log = extractCookLog(payload);
  if (!log) {
    return;
  }
  recipeStore.applyCookLog(mapApiCookLog(log));
}

export function handleCookLogUpdatedEvent(payload: unknown): void {
  const log = extractCookLog(payload);
  if (!log) {
    return;
  }
  recipeStore.applyCookLog(mapApiCookLog(log));
}

export function handleCookLogRemovedEvent(payload: unknown): void {
  const postId = extractPostId(payload);
  if (!postId) {
    return;
  }
  recipeStore.applyCookLogRemoval(postId);
}

export function handleRecipeCategoryCreatedEvent(payload: unknown): void {
  const category = extractRecipeCategory(payload);
  if (!category) {
    return;
  }
  recipeStore.applyCategory(mapApiRecipeCategory(category));
}

export function handleRecipeCategoryUpdatedEvent(payload: unknown): void {
  const category = extractRecipeCategory(payload);
  if (!category) {
    return;
  }
  const current = get(recipeStore).categories.find((item) => item.id === category.id);
  const mapped = mapApiRecipeCategory(category);
  recipeStore.applyCategory(mapped);
  if (current && current.name !== mapped.name) {
    recipeStore.updateSavedRecipeCategory(current.name, mapped.name);
  }
}

export function handleRecipeCategoryDeletedEvent(payload: unknown): void {
  const category = extractRecipeCategory(payload);
  const categoryId = category?.id ?? extractCategoryId(payload);
  if (!categoryId) {
    return;
  }
  const categoryName = category?.name ?? extractCategoryName(payload) ?? undefined;
  recipeStore.applyCategoryDeletion(categoryId, categoryName);
}
