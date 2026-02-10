import { get, writable } from 'svelte/store';
import {
  api,
  type BookshelfCategory,
  type BookshelfItem,
  type ReadLog,
  type PostReadLogsResponse,
} from '../services/api';
import { currentUser } from './authStore';
import { postStore, type BookStats } from './postStore';

const DEFAULT_BOOKSHELF_CATEGORY = 'Uncategorized';
const DEFAULT_PAGE_SIZE = 20;

export const bookshelfCategories = writable<BookshelfCategory[]>([]);
export const myBookshelf = writable<Map<string, BookshelfItem[]>>(new Map());
export const allBookshelf = writable<Map<string, BookshelfItem[]>>(new Map());
export const readHistory = writable<ReadLog[]>([]);

interface BookStoreLoadState {
  categories: boolean;
  myBookshelf: boolean;
  allBookshelf: boolean;
  readHistory: boolean;
}

interface BookStoreCursorState {
  myBookshelf: string | null;
  allBookshelf: string | null;
  readHistory: string | null;
}

interface BookStoreMetaState {
  loading: BookStoreLoadState;
  cursors: BookStoreCursorState;
  error: string | null;
}

const initialMetaState: BookStoreMetaState = {
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
};

export const bookStoreMeta = writable<BookStoreMetaState>({ ...initialMetaState });

function setLoading(key: keyof BookStoreLoadState, value: boolean): void {
  bookStoreMeta.update((state) => ({
    ...state,
    loading: {
      ...state.loading,
      [key]: value,
    },
    error: value ? null : state.error,
  }));
}

function setCursor(key: keyof BookStoreCursorState, cursor?: string): void {
  bookStoreMeta.update((state) => ({
    ...state,
    cursors: {
      ...state.cursors,
      [key]: cursor ?? null,
    },
    error: null,
  }));
}

function setStoreError(error: string | null): void {
  bookStoreMeta.update((state) => ({
    ...state,
    loading: {
      categories: false,
      myBookshelf: false,
      allBookshelf: false,
      readHistory: false,
    },
    error,
  }));
}

function normalizeCategoryName(name: string): string {
  return name.trim();
}

function normalizeCategoryList(categories: string[]): string[] {
  if (!Array.isArray(categories)) {
    return [];
  }

  const seen = new Set<string>();
  const normalized: string[] = [];
  for (const category of categories) {
    const trimmed = normalizeCategoryName(category);
    if (!trimmed || trimmed.toLowerCase() === DEFAULT_BOOKSHELF_CATEGORY.toLowerCase()) {
      continue;
    }
    if (seen.has(trimmed)) {
      continue;
    }
    seen.add(trimmed);
    normalized.push(trimmed);
  }
  return normalized;
}

function buildCategoryIdToName(categories: BookshelfCategory[]): Map<string, string> {
  const categoryMap = new Map<string, string>();
  for (const category of categories) {
    categoryMap.set(category.id, category.name);
  }
  return categoryMap;
}

function resolveCategoryKey(
  item: BookshelfItem,
  categoryMap: Map<string, string>,
  fallbackCategory?: string
): string {
  if (item.categoryId) {
    return categoryMap.get(item.categoryId) ?? fallbackCategory ?? item.categoryId;
  }
  return fallbackCategory ?? DEFAULT_BOOKSHELF_CATEGORY;
}

function removeBookshelfPost(
  source: Map<string, BookshelfItem[]>,
  postId: string
): Map<string, BookshelfItem[]> {
  const next = new Map(source);
  for (const [category, items] of next.entries()) {
    const filtered = items.filter((item) => item.postId !== postId);
    if (filtered.length > 0) {
      next.set(category, filtered);
    } else {
      next.delete(category);
    }
  }
  return next;
}

function upsertBookshelfItem(
  source: Map<string, BookshelfItem[]>,
  item: BookshelfItem,
  categoryMap: Map<string, string>,
  fallbackCategory?: string,
  placement: 'prepend' | 'append' = 'prepend'
): Map<string, BookshelfItem[]> {
  const category = resolveCategoryKey(item, categoryMap, fallbackCategory);
  const next = removeBookshelfPost(source, item.postId);
  const existing = next.get(category) ?? [];
  next.set(category, placement === 'append' ? [...existing, item] : [item, ...existing]);
  return next;
}

function mergeBookshelfItems(
  source: Map<string, BookshelfItem[]>,
  items: BookshelfItem[],
  categories: BookshelfCategory[],
  fallbackCategory?: string,
  placement: 'prepend' | 'append' = 'append'
): Map<string, BookshelfItem[]> {
  const categoryMap = buildCategoryIdToName(categories);
  let next = source;
  for (const item of items) {
    next = upsertBookshelfItem(next, item, categoryMap, fallbackCategory, placement);
  }
  return next;
}

function moveCategoryItems(
  source: Map<string, BookshelfItem[]>,
  fromCategory: string,
  toCategory: string
): Map<string, BookshelfItem[]> {
  if (fromCategory === toCategory) {
    return source;
  }

  const next = new Map(source);
  const items = next.get(fromCategory) ?? [];
  next.delete(fromCategory);
  if (items.length === 0) {
    return next;
  }

  const target = next.get(toCategory) ?? [];
  next.set(toCategory, [...target, ...items]);
  return next;
}

function upsertReadLog(logs: ReadLog[], nextLog: ReadLog, placement: 'prepend' | 'append' = 'prepend'): ReadLog[] {
  const index = logs.findIndex((entry) => entry.postId === nextLog.postId);
  if (index === -1) {
    return placement === 'append' ? [...logs, nextLog] : [nextLog, ...logs];
  }
  const updated = [...logs];
  updated[index] = {
    ...updated[index],
    ...nextLog,
  };
  return updated;
}

function normalizeAverageRating(value: number, readCount: number): number | null {
  if (readCount <= 0) {
    return null;
  }
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) {
    return null;
  }
  return value;
}

function getCurrentBookStats(postId: string): BookStats | null {
  const post = get(postStore).posts.find((entry) => entry.id === postId);
  if (!post) {
    return null;
  }
  return post.bookStats ?? post.book_stats ?? null;
}

function restoreBookStats(postId: string, previous: BookStats | null): void {
  if (previous) {
    postStore.setBookStats(postId, previous);
    return;
  }

  postStore.setBookStats(postId, {
    bookshelfCount: 0,
    readCount: 0,
    averageRating: null,
    viewerOnBookshelf: false,
    viewerCategories: [],
    viewerRead: false,
    viewerRating: null,
  });
}

async function refreshPostReadStats(postId: string): Promise<void> {
  try {
    const readStats: PostReadLogsResponse = await api.getPostReadLogs(postId);
    const ratedCount =
      typeof readStats.ratedCount === 'number' && Number.isFinite(readStats.ratedCount)
        ? readStats.ratedCount
        : (readStats.readers ?? []).filter(
            (reader) => typeof reader.rating === 'number' && Number.isFinite(reader.rating)
          ).length;
    postStore.setBookStats(postId, {
      readCount: readStats.readCount ?? 0,
      averageRating: normalizeAverageRating(readStats.averageRating ?? 0, readStats.readCount ?? 0),
      ratedCount,
      viewerRead: readStats.viewerRead ?? false,
      viewerRating: readStats.viewerRating ?? null,
    });
  } catch {
    // Ignore transient reconciliation errors.
  }
}

function getCategoryIdForName(categoryName: string): string | undefined {
  const categories = get(bookshelfCategories);
  const match = categories.find((entry) => entry.name === categoryName);
  return match?.id;
}

function buildOptimisticBookshelfItem(postId: string, categoryName?: string): BookshelfItem {
  const selectedCategory = categoryName ? normalizeCategoryName(categoryName) : '';
  const userId = get(currentUser)?.id ?? 'current-user';
  return {
    id: `optimistic-${postId}-${Date.now()}`,
    userId,
    postId,
    categoryId: selectedCategory ? getCategoryIdForName(selectedCategory) : undefined,
    createdAt: new Date().toISOString(),
  };
}

async function loadBookshelfCategories(): Promise<void> {
  setLoading('categories', true);
  try {
    const response = await api.getBookshelfCategories();
    bookshelfCategories.set(response.categories ?? []);
    setLoading('categories', false);
  } catch (error) {
    setStoreError(error instanceof Error ? error.message : 'Failed to load bookshelf categories');
  }
}

async function addToBookshelf(postId: string, categories: string[]): Promise<void> {
  const previousMyBookshelf = get(myBookshelf);
  const previousAllBookshelf = get(allBookshelf);
  const previousStats = getCurrentBookStats(postId);
  const normalizedCategories = normalizeCategoryList(categories);
  const selectedCategory = normalizedCategories[0] ?? DEFAULT_BOOKSHELF_CATEGORY;
  const optimisticItem = buildOptimisticBookshelfItem(
    postId,
    selectedCategory === DEFAULT_BOOKSHELF_CATEGORY ? undefined : selectedCategory
  );

  const categoryMap = buildCategoryIdToName(get(bookshelfCategories));
  myBookshelf.set(upsertBookshelfItem(previousMyBookshelf, optimisticItem, categoryMap, selectedCategory));
  allBookshelf.set(upsertBookshelfItem(previousAllBookshelf, optimisticItem, categoryMap, selectedCategory));
  postStore.setBookBookshelfState(
    postId,
    true,
    selectedCategory === DEFAULT_BOOKSHELF_CATEGORY ? [] : [selectedCategory]
  );

  try {
    await api.addToBookshelf(postId, categories);
    void refreshPostReadStats(postId);
    setStoreError(null);
  } catch (error) {
    myBookshelf.set(previousMyBookshelf);
    allBookshelf.set(previousAllBookshelf);
    restoreBookStats(postId, previousStats);
    setStoreError(error instanceof Error ? error.message : 'Failed to add to bookshelf');
  }
}

async function removeFromBookshelf(postId: string): Promise<void> {
  const previousMyBookshelf = get(myBookshelf);
  const previousAllBookshelf = get(allBookshelf);
  const previousStats = getCurrentBookStats(postId);

  myBookshelf.set(removeBookshelfPost(previousMyBookshelf, postId));
  allBookshelf.set(removeBookshelfPost(previousAllBookshelf, postId));
  postStore.setBookBookshelfState(postId, false, []);

  try {
    await api.removeFromBookshelf(postId);
    void refreshPostReadStats(postId);
    setStoreError(null);
  } catch (error) {
    myBookshelf.set(previousMyBookshelf);
    allBookshelf.set(previousAllBookshelf);
    restoreBookStats(postId, previousStats);
    setStoreError(error instanceof Error ? error.message : 'Failed to remove from bookshelf');
  }
}

async function createCategory(name: string): Promise<void> {
  try {
    const response = await api.createBookshelfCategory(name);
    const category = response.category;
    bookshelfCategories.update((existing) => {
      const index = existing.findIndex((item) => item.id === category.id);
      if (index === -1) {
        return [...existing, category];
      }
      const updated = [...existing];
      updated[index] = category;
      return updated;
    });
    setStoreError(null);
  } catch (error) {
    setStoreError(error instanceof Error ? error.message : 'Failed to create category');
  }
}

async function updateCategory(id: string, name: string, position: number): Promise<void> {
  const previousCategories = get(bookshelfCategories);
  const existingCategory = previousCategories.find((category) => category.id === id);
  try {
    const response = await api.updateBookshelfCategory(id, name, position);
    const updatedCategory = response.category;
    bookshelfCategories.update((current) => {
      const index = current.findIndex((item) => item.id === updatedCategory.id);
      if (index === -1) {
        return [...current, updatedCategory];
      }
      const updated = [...current];
      updated[index] = updatedCategory;
      return updated;
    });

    if (existingCategory && existingCategory.name !== updatedCategory.name) {
      myBookshelf.update((current) =>
        moveCategoryItems(current, existingCategory.name, updatedCategory.name)
      );
      allBookshelf.update((current) =>
        moveCategoryItems(current, existingCategory.name, updatedCategory.name)
      );
    }
    setStoreError(null);
  } catch (error) {
    setStoreError(error instanceof Error ? error.message : 'Failed to update category');
  }
}

async function deleteCategory(id: string): Promise<void> {
  const previousCategories = get(bookshelfCategories);
  const existingCategory = previousCategories.find((category) => category.id === id);
  const deletedCategoryName = existingCategory?.name ?? '';

  try {
    await api.deleteBookshelfCategory(id);
    bookshelfCategories.set(previousCategories.filter((category) => category.id !== id));
    if (deletedCategoryName) {
      myBookshelf.update((current) =>
        moveCategoryItems(current, deletedCategoryName, DEFAULT_BOOKSHELF_CATEGORY)
      );
      allBookshelf.update((current) =>
        moveCategoryItems(current, deletedCategoryName, DEFAULT_BOOKSHELF_CATEGORY)
      );
    }
    setStoreError(null);
  } catch (error) {
    setStoreError(error instanceof Error ? error.message : 'Failed to delete category');
  }
}

async function logRead(postId: string, rating?: number): Promise<void> {
  const previousHistory = get(readHistory);
  const previousStats = getCurrentBookStats(postId);
  const existingLog = previousHistory.find((entry) => entry.postId === postId);
  const userId = get(currentUser)?.id ?? existingLog?.userId ?? 'current-user';
  const optimisticLog: ReadLog = {
    id: existingLog?.id ?? `optimistic-read-${postId}`,
    userId,
    postId,
    rating,
    createdAt: existingLog?.createdAt ?? new Date().toISOString(),
    deletedAt: undefined,
  };

  readHistory.set(upsertReadLog(previousHistory, optimisticLog));
  postStore.setBookReadState(postId, true, rating ?? existingLog?.rating ?? null);

  try {
    const response = await api.logRead(postId, rating);
    readHistory.update((current) => upsertReadLog(current, response.readLog));
    void refreshPostReadStats(postId);
    setStoreError(null);
  } catch (error) {
    readHistory.set(previousHistory);
    restoreBookStats(postId, previousStats);
    setStoreError(error instanceof Error ? error.message : 'Failed to log read');
  }
}

async function removeRead(postId: string): Promise<void> {
  const previousHistory = get(readHistory);
  const previousStats = getCurrentBookStats(postId);

  readHistory.set(previousHistory.filter((entry) => entry.postId !== postId));
  postStore.setBookReadState(postId, false, null);

  try {
    await api.removeReadLog(postId);
    void refreshPostReadStats(postId);
    setStoreError(null);
  } catch (error) {
    readHistory.set(previousHistory);
    restoreBookStats(postId, previousStats);
    setStoreError(error instanceof Error ? error.message : 'Failed to remove read');
  }
}

async function updateRating(postId: string, rating: number): Promise<void> {
  const previousHistory = get(readHistory);
  const previousStats = getCurrentBookStats(postId);
  const existingLog = previousHistory.find((entry) => entry.postId === postId);
  const optimisticLog: ReadLog = existingLog
    ? {
        ...existingLog,
        rating,
      }
    : {
        id: `optimistic-read-${postId}`,
        userId: get(currentUser)?.id ?? 'current-user',
        postId,
        rating,
        createdAt: new Date().toISOString(),
      };

  readHistory.set(upsertReadLog(previousHistory, optimisticLog));
  postStore.setBookReadState(postId, true, rating);

  try {
    const response = await api.updateReadRating(postId, rating);
    readHistory.update((current) => upsertReadLog(current, response.readLog));
    void refreshPostReadStats(postId);
    setStoreError(null);
  } catch (error) {
    readHistory.set(previousHistory);
    restoreBookStats(postId, previousStats);
    setStoreError(error instanceof Error ? error.message : 'Failed to update rating');
  }
}

async function loadMyBookshelf(category?: string, cursor?: string): Promise<string | undefined> {
  setLoading('myBookshelf', true);
  try {
    const response = await api.getMyBookshelf(category, cursor, DEFAULT_PAGE_SIZE);
    const categories = get(bookshelfCategories);
    if (cursor) {
      myBookshelf.update((current) =>
        mergeBookshelfItems(current, response.bookshelfItems ?? [], categories, category, 'append')
      );
    } else {
      const grouped = mergeBookshelfItems(
        new Map(),
        response.bookshelfItems ?? [],
        categories,
        category,
        'append'
      );
      myBookshelf.set(grouped);
    }
    setCursor('myBookshelf', response.nextCursor);
    setLoading('myBookshelf', false);
    return response.nextCursor;
  } catch (error) {
    setStoreError(error instanceof Error ? error.message : 'Failed to load bookshelf');
    return undefined;
  }
}

async function loadAllBookshelf(category?: string, cursor?: string): Promise<string | undefined> {
  setLoading('allBookshelf', true);
  try {
    const response = await api.getAllBookshelfItems(category, cursor, DEFAULT_PAGE_SIZE);
    const categories = get(bookshelfCategories);
    if (cursor) {
      allBookshelf.update((current) =>
        mergeBookshelfItems(current, response.bookshelfItems ?? [], categories, category, 'append')
      );
    } else {
      const grouped = mergeBookshelfItems(
        new Map(),
        response.bookshelfItems ?? [],
        categories,
        category,
        'append'
      );
      allBookshelf.set(grouped);
    }
    setCursor('allBookshelf', response.nextCursor);
    setLoading('allBookshelf', false);
    return response.nextCursor;
  } catch (error) {
    setStoreError(error instanceof Error ? error.message : 'Failed to load community bookshelf');
    return undefined;
  }
}

async function loadReadHistory(cursor?: string): Promise<string | undefined> {
  setLoading('readHistory', true);
  try {
    const response = await api.getReadHistory(cursor, DEFAULT_PAGE_SIZE);
    if (cursor) {
      readHistory.update((current) => {
        let next = current;
        for (const log of response.readLogs ?? []) {
          next = upsertReadLog(next, log, 'append');
        }
        return next;
      });
    } else {
      readHistory.set(response.readLogs ?? []);
    }
    setCursor('readHistory', response.nextCursor);
    setLoading('readHistory', false);
    return response.nextCursor;
  } catch (error) {
    setStoreError(error instanceof Error ? error.message : 'Failed to load read history');
    return undefined;
  }
}

function reset(): void {
  bookshelfCategories.set([]);
  myBookshelf.set(new Map());
  allBookshelf.set(new Map());
  readHistory.set([]);
  bookStoreMeta.set({ ...initialMetaState });
}

export const bookStore = {
  loadBookshelfCategories,
  addToBookshelf,
  removeFromBookshelf,
  createCategory,
  updateCategory,
  deleteCategory,
  logRead,
  removeRead,
  updateRating,
  loadMyBookshelf,
  loadAllBookshelf,
  loadReadHistory,
  reset,
};
