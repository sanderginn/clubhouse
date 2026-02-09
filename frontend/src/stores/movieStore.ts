import { derived, get, writable } from 'svelte/store';
import {
  api,
  type WatchlistItem as ApiWatchlistItem,
  type WatchlistItemWithPost as ApiWatchlistItemWithPost,
  type WatchlistCategory as ApiWatchlistCategory,
  type WatchLog as ApiWatchLog,
  type WatchLogWithPost as ApiWatchLogWithPost,
} from '../services/api';
import { currentUser } from './authStore';
import { mapApiPost, type ApiPost } from './postMapper';
import { postStore, type Post } from './postStore';

const DEFAULT_WATCHLIST_CATEGORY = 'Uncategorized';

export interface WatchlistItem {
  id: string;
  userId: string;
  postId: string;
  category: string;
  createdAt: string;
  post?: Post;
}

export interface WatchlistCategory {
  id: string;
  name: string;
  position: number;
}

export interface WatchLog {
  id: string;
  userId: string;
  postId: string;
  rating: number;
  notes?: string;
  watchedAt: string;
  post?: Post;
}

export interface MovieStoreState {
  watchlist: Map<string, WatchlistItem[]>;
  categories: WatchlistCategory[];
  watchLogs: WatchLog[];
  isLoadingWatchlist: boolean;
  isLoadingCategories: boolean;
  isLoadingWatchLogs: boolean;
  error: string | null;
}

function mapApiWatchlistCategory(category: ApiWatchlistCategory): WatchlistCategory {
  return {
    id: category.id,
    name: category.name,
    position: category.position,
  };
}

function mapApiWatchlistItem(item: ApiWatchlistItem | ApiWatchlistItemWithPost): WatchlistItem {
  const post = 'post' in item && item.post ? mapApiPost(item.post as ApiPost) : undefined;
  return {
    id: item.id,
    userId: item.userId,
    postId: item.postId,
    category: item.category,
    createdAt: item.createdAt,
    post,
  };
}

function mapApiWatchLog(log: ApiWatchLog | ApiWatchLogWithPost): WatchLog {
  const post = 'post' in log && log.post ? mapApiPost(log.post as ApiPost) : undefined;
  return {
    id: log.id,
    userId: log.userId,
    postId: log.postId,
    rating: log.rating,
    notes: log.notes,
    watchedAt: log.watchedAt,
    post,
  };
}

function buildWatchlistMap(
  categories: { name: string; items: ApiWatchlistItemWithPost[] }[]
): Map<string, WatchlistItem[]> {
  const entries = new Map<string, WatchlistItem[]>();
  for (const category of categories) {
    entries.set(category.name, (category.items ?? []).map(mapApiWatchlistItem));
  }
  return entries;
}

function addWatchlistItemToMap(
  map: Map<string, WatchlistItem[]>,
  watchlistItem: WatchlistItem
): Map<string, WatchlistItem[]> {
  const next = new Map(map);
  const existing = next.get(watchlistItem.category) ?? [];
  const filtered = existing.filter(
    (item) => item.id !== watchlistItem.id && item.postId !== watchlistItem.postId
  );
  next.set(watchlistItem.category, [...filtered, watchlistItem]);
  return next;
}

function removeWatchlistItemFromMap(
  map: Map<string, WatchlistItem[]>,
  postId: string,
  category?: string
): Map<string, WatchlistItem[]> {
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

  for (const [key, items] of next.entries()) {
    const filtered = items.filter((item) => item.postId !== postId);
    if (filtered.length > 0) {
      next.set(key, filtered);
    } else {
      next.delete(key);
    }
  }

  return next;
}

function moveCategoryWatchlistItems(
  map: Map<string, WatchlistItem[]>,
  fromCategory: string,
  toCategory: string
): Map<string, WatchlistItem[]> {
  if (fromCategory === toCategory) {
    return map;
  }

  const next = new Map(map);
  const existing = next.get(fromCategory) ?? [];
  next.delete(fromCategory);
  if (existing.length === 0) {
    return next;
  }

  const updated = existing.map((item) => ({ ...item, category: toCategory }));
  const target = next.get(toCategory) ?? [];
  next.set(toCategory, [...target, ...updated]);
  return next;
}

function upsertWatchLog(logs: WatchLog[], nextLog: WatchLog): WatchLog[] {
  const index = logs.findIndex((entry) => entry.postId === nextLog.postId);
  if (index === -1) {
    return [nextLog, ...logs];
  }

  const existing = logs[index];
  const merged: WatchLog = {
    ...existing,
    ...nextLog,
    post: nextLog.post ?? existing.post,
  };

  const updated = [...logs];
  updated[index] = merged;
  return updated;
}

function extractWatchlistItems(payload: unknown): (ApiWatchlistItem | ApiWatchlistItemWithPost)[] {
  if (!payload || typeof payload !== 'object') {
    return [];
  }

  const record = payload as Record<string, unknown>;
  if (Array.isArray(record.watchlist_items)) {
    return record.watchlist_items as (ApiWatchlistItem | ApiWatchlistItemWithPost)[];
  }
  if (Array.isArray(record.watchlistItems)) {
    return record.watchlistItems as (ApiWatchlistItem | ApiWatchlistItemWithPost)[];
  }
  if (record.watchlist_item && typeof record.watchlist_item === 'object') {
    return [record.watchlist_item as ApiWatchlistItem | ApiWatchlistItemWithPost];
  }
  if (record.watchlistItem && typeof record.watchlistItem === 'object') {
    return [record.watchlistItem as ApiWatchlistItem | ApiWatchlistItemWithPost];
  }

  return [];
}

function extractWatchLog(payload: unknown): ApiWatchLog | ApiWatchLogWithPost | null {
  if (!payload || typeof payload !== 'object') {
    return null;
  }

  const record = payload as Record<string, unknown>;
  if (record.watch_log && typeof record.watch_log === 'object') {
    return record.watch_log as ApiWatchLog;
  }
  if (record.watchLog && typeof record.watchLog === 'object') {
    return record.watchLog as ApiWatchLog;
  }
  if (record.log && typeof record.log === 'object') {
    return record.log as ApiWatchLog;
  }

  return null;
}

function extractPostId(payload: unknown): string | null {
  if (!payload || typeof payload !== 'object') {
    return null;
  }

  const record = payload as Record<string, unknown>;
  const postId = record.post_id ?? record.postId;
  if (typeof postId === 'string' && postId.length > 0) {
    return postId;
  }

  const nestedWatchlistItem =
    record.watchlist_item ?? record.watchlistItem ?? record.watch_log ?? record.watchLog ?? record.log;
  if (nestedWatchlistItem && typeof nestedWatchlistItem === 'object') {
    const nestedRecord = nestedWatchlistItem as Record<string, unknown>;
    const nestedPostID = nestedRecord.post_id ?? nestedRecord.postId;
    if (typeof nestedPostID === 'string' && nestedPostID.length > 0) {
      return nestedPostID;
    }
  }

  return null;
}

function extractUserId(payload: unknown): string | null {
  if (!payload || typeof payload !== 'object') {
    return null;
  }

  const record = payload as Record<string, unknown>;
  const userId = record.user_id ?? record.userId;
  return typeof userId === 'string' && userId.length > 0 ? userId : null;
}

function extractCategoryName(payload: unknown): string | null {
  if (!payload || typeof payload !== 'object') {
    return null;
  }

  const record = payload as Record<string, unknown>;
  const category = record.category ?? record.categoryName ?? record.category_name;
  return typeof category === 'string' && category.length > 0 ? category : null;
}

function extractCategories(payload: unknown): string[] {
  if (!payload || typeof payload !== 'object') {
    return [];
  }

  const record = payload as Record<string, unknown>;
  if (Array.isArray(record.categories)) {
    return record.categories
      .filter((value): value is string => typeof value === 'string')
      .map((value) => value.trim())
      .filter((value) => value.length > 0);
  }

  const category = extractCategoryName(payload);
  return category ? [category] : [];
}

function extractRating(payload: unknown): number | null {
  if (!payload || typeof payload !== 'object') {
    return null;
  }

  const record = payload as Record<string, unknown>;
  const rating = record.rating;
  return typeof rating === 'number' && Number.isFinite(rating) ? rating : null;
}

function shouldHandleCurrentUserEvent(payload: unknown): boolean {
  const actingUserID = extractUserId(payload);
  const currentUserID = get(currentUser)?.id;
  return !!actingUserID && !!currentUserID && actingUserID === currentUserID;
}

function buildWatchlistItemsFromCategories(payload: unknown): WatchlistItem[] {
  const postId = extractPostId(payload);
  const actingUserID = extractUserId(payload);
  const currentUserID = get(currentUser)?.id;
  if (!postId || !actingUserID || !currentUserID || actingUserID !== currentUserID) {
    return [];
  }

  const categories = extractCategories(payload);
  if (categories.length === 0) {
    return [];
  }

  const createdAt = new Date().toISOString();
  return categories.map((category) => ({
    id: `ws-${postId}-${category}-${Date.now()}`,
    userId: currentUserID,
    postId,
    category,
    createdAt,
  }));
}

function buildWatchLogFromPayload(payload: unknown): WatchLog | null {
  const postId = extractPostId(payload);
  const actingUserID = extractUserId(payload);
  const currentUserID = get(currentUser)?.id;
  const rating = extractRating(payload);
  if (!postId || !actingUserID || !currentUserID || actingUserID !== currentUserID) {
    return null;
  }
  if (rating === null) {
    return null;
  }

  return {
    id: `ws-watch-${postId}-${Date.now()}`,
    userId: currentUserID,
    postId,
    rating,
    watchedAt: new Date().toISOString(),
  };
}

async function refreshPostMovieStats(postId: string): Promise<void> {
  try {
    const [watchlistInfo, watchLogInfo] = await Promise.all([
      api.getPostWatchlistInfo(postId),
      api.getPostWatchLogs(postId),
    ]);

    postStore.setMovieStats(postId, {
      watchlistCount: watchlistInfo.saveCount ?? 0,
      watchCount: watchLogInfo.watchCount ?? 0,
      averageRating:
        typeof watchLogInfo.avgRating === 'number' && Number.isFinite(watchLogInfo.avgRating)
          ? watchLogInfo.avgRating
          : null,
      viewerWatchlisted: watchlistInfo.viewerSaved ?? false,
      viewerWatched: watchLogInfo.viewerWatched ?? false,
      viewerRating:
        typeof watchLogInfo.viewerRating === 'number' && Number.isFinite(watchLogInfo.viewerRating)
          ? watchLogInfo.viewerRating
          : null,
      viewerCategories: watchlistInfo.viewerCategories ?? [],
    });
  } catch {
    // Ignore transient refresh errors; future events or feed refresh will reconcile.
  }
}

const initialState: MovieStoreState = {
  watchlist: new Map(),
  categories: [],
  watchLogs: [],
  isLoadingWatchlist: false,
  isLoadingCategories: false,
  isLoadingWatchLogs: false,
  error: null,
};

function createMovieStore() {
  const { subscribe, update, set } = writable<MovieStoreState>({ ...initialState });

  return {
    subscribe,
    setWatchlist: (watchlist: Map<string, WatchlistItem[]>) =>
      update((state) => ({
        ...state,
        watchlist,
        isLoadingWatchlist: false,
        error: null,
      })),
    setCategories: (categories: WatchlistCategory[]) =>
      update((state) => ({
        ...state,
        categories,
        isLoadingCategories: false,
        error: null,
      })),
    setWatchLogs: (watchLogs: WatchLog[]) =>
      update((state) => ({
        ...state,
        watchLogs,
        isLoadingWatchLogs: false,
        error: null,
      })),
    setLoadingWatchlist: (isLoading: boolean) =>
      update((state) => ({
        ...state,
        isLoadingWatchlist: isLoading,
        error: isLoading ? null : state.error,
      })),
    setLoadingCategories: (isLoading: boolean) =>
      update((state) => ({
        ...state,
        isLoadingCategories: isLoading,
        error: isLoading ? null : state.error,
      })),
    setLoadingWatchLogs: (isLoading: boolean) =>
      update((state) => ({
        ...state,
        isLoadingWatchLogs: isLoading,
        error: isLoading ? null : state.error,
      })),
    setError: (error: string | null) =>
      update((state) => ({
        ...state,
        error,
        isLoadingWatchlist: false,
        isLoadingCategories: false,
        isLoadingWatchLogs: false,
      })),
    reset: () => set({ ...initialState, watchlist: new Map() }),
    applyWatchlistItems: (watchlistItems: WatchlistItem[]) =>
      update((state) => {
        let next = state.watchlist;
        for (const item of watchlistItems) {
          next = addWatchlistItemToMap(next, item);
        }
        return {
          ...state,
          watchlist: next,
          error: null,
        };
      }),
    applyUnwatchlist: (postId: string, category?: string) =>
      update((state) => ({
        ...state,
        watchlist: removeWatchlistItemFromMap(state.watchlist, postId, category),
        error: null,
      })),
    applyWatchLog: (watchLog: WatchLog) =>
      update((state) => ({
        ...state,
        watchLogs: upsertWatchLog(state.watchLogs, watchLog),
        error: null,
      })),
    applyWatchLogRemoval: (postId: string) =>
      update((state) => ({
        ...state,
        watchLogs: state.watchLogs.filter((log) => log.postId !== postId),
        error: null,
      })),
    applyCategory: (category: WatchlistCategory) =>
      update((state) => {
        const existingIndex = state.categories.findIndex((item) => item.id === category.id);
        const nextCategories = [...state.categories];
        if (existingIndex === -1) {
          nextCategories.push(category);
        } else {
          nextCategories[existingIndex] = {
            ...nextCategories[existingIndex],
            ...category,
          };
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
        let nextMap = state.watchlist;
        if (nameToDelete) {
          nextMap = moveCategoryWatchlistItems(nextMap, nameToDelete, DEFAULT_WATCHLIST_CATEGORY);
        }
        return {
          ...state,
          categories: nextCategories,
          watchlist: nextMap,
          error: null,
        };
      }),
    updateWatchlistCategoryName: (fromCategory: string, toCategory: string) =>
      update((state) => ({
        ...state,
        watchlist: moveCategoryWatchlistItems(state.watchlist, fromCategory, toCategory),
      })),
    loadWatchlist: async (): Promise<void> => {
      movieStore.setLoadingWatchlist(true);
      try {
        const response = await api.getMyWatchlist();
        movieStore.setWatchlist(buildWatchlistMap(response.categories ?? []));
      } catch (error) {
        movieStore.setError(error instanceof Error ? error.message : 'Failed to load watchlist');
      }
    },
    loadWatchlistCategories: async (): Promise<void> => {
      movieStore.setLoadingCategories(true);
      try {
        const response = await api.getWatchlistCategories();
        movieStore.setCategories((response.categories ?? []).map(mapApiWatchlistCategory));
      } catch (error) {
        movieStore.setError(
          error instanceof Error ? error.message : 'Failed to load watchlist categories'
        );
      }
    },
    addToWatchlist: async (postId: string, categories: string[]): Promise<void> => {
      try {
        const response = await api.addToWatchlist(postId, categories);
        movieStore.applyWatchlistItems((response.watchlistItems ?? []).map(mapApiWatchlistItem));
        void refreshPostMovieStats(postId);
      } catch (error) {
        movieStore.setError(error instanceof Error ? error.message : 'Failed to add to watchlist');
      }
    },
    removeFromWatchlist: async (postId: string, category?: string): Promise<void> => {
      try {
        await api.removeFromWatchlist(postId, category);
        movieStore.applyUnwatchlist(postId, category);
        void refreshPostMovieStats(postId);
      } catch (error) {
        movieStore.setError(
          error instanceof Error ? error.message : 'Failed to remove from watchlist'
        );
      }
    },
    createCategory: async (name: string): Promise<void> => {
      try {
        const response = await api.createWatchlistCategory(name);
        movieStore.applyCategory(mapApiWatchlistCategory(response.category));
      } catch (error) {
        movieStore.setError(error instanceof Error ? error.message : 'Failed to create category');
      }
    },
    updateCategory: async (id: string, data: { name?: string; position?: number }): Promise<void> => {
      const existing = get(movieStore).categories.find((category) => category.id === id);
      try {
        const response = await api.updateWatchlistCategory(id, data);
        const updated = mapApiWatchlistCategory(response.category);
        movieStore.applyCategory(updated);
        if (existing && existing.name !== updated.name) {
          movieStore.updateWatchlistCategoryName(existing.name, updated.name);
        }
      } catch (error) {
        movieStore.setError(error instanceof Error ? error.message : 'Failed to update category');
      }
    },
    deleteCategory: async (id: string): Promise<void> => {
      const existing = get(movieStore).categories.find((category) => category.id === id);
      try {
        await api.deleteWatchlistCategory(id);
        movieStore.applyCategoryDeletion(id, existing?.name);
      } catch (error) {
        movieStore.setError(error instanceof Error ? error.message : 'Failed to delete category');
      }
    },
    loadWatchLogs: async (limit?: number): Promise<void> => {
      movieStore.setLoadingWatchLogs(true);
      try {
        const response = await api.getMyWatchLogs(limit);
        movieStore.setWatchLogs((response.watchLogs ?? []).map(mapApiWatchLog));
      } catch (error) {
        movieStore.setError(error instanceof Error ? error.message : 'Failed to load watch logs');
      }
    },
    logWatch: async (postId: string, rating: number, notes?: string): Promise<void> => {
      try {
        const response = await api.logWatch(postId, rating, notes);
        movieStore.applyWatchLog(mapApiWatchLog(response.watchLog));
        void refreshPostMovieStats(postId);
      } catch (error) {
        movieStore.setError(error instanceof Error ? error.message : 'Failed to log watch');
      }
    },
    updateWatchLog: async (postId: string, rating?: number, notes?: string): Promise<void> => {
      try {
        const response = await api.updateWatchLog(postId, { rating, notes });
        movieStore.applyWatchLog(mapApiWatchLog(response.watchLog));
        void refreshPostMovieStats(postId);
      } catch (error) {
        movieStore.setError(error instanceof Error ? error.message : 'Failed to update watch log');
      }
    },
    removeWatchLog: async (postId: string): Promise<void> => {
      try {
        await api.removeWatchLog(postId);
        movieStore.applyWatchLogRemoval(postId);
        void refreshPostMovieStats(postId);
      } catch (error) {
        movieStore.setError(error instanceof Error ? error.message : 'Failed to remove watch log');
      }
    },
  };
}

export const movieStore = createMovieStore();

export const watchlistByCategory = derived(movieStore, ($store) => $store.watchlist);

export const sortedCategories = derived(movieStore, ($store) =>
  [...$store.categories].sort((a, b) => {
    if (a.position !== b.position) {
      return a.position - b.position;
    }
    return a.name.localeCompare(b.name);
  })
);

export function handleMovieWatchlistedEvent(payload: unknown): void {
  const postId = extractPostId(payload);
  if (!postId) {
    return;
  }

  if (shouldHandleCurrentUserEvent(payload)) {
    const watchlistItems = extractWatchlistItems(payload).map(mapApiWatchlistItem);
    const fallbackItems =
      watchlistItems.length > 0 ? watchlistItems : buildWatchlistItemsFromCategories(payload);
    if (fallbackItems.length > 0) {
      movieStore.applyWatchlistItems(fallbackItems);
    }
  }

  void refreshPostMovieStats(postId);
}

export function handleMovieUnwatchlistedEvent(payload: unknown): void {
  const postId = extractPostId(payload);
  if (!postId) {
    return;
  }

  if (shouldHandleCurrentUserEvent(payload)) {
    const category = extractCategoryName(payload);
    if (category) {
      movieStore.applyUnwatchlist(postId, category);
    } else {
      // Backend movie_unwatchlisted events currently omit category.
      // Reload the watchlist to avoid clearing all categories locally for this post.
      void movieStore.loadWatchlist();
    }
  }

  void refreshPostMovieStats(postId);
}

export function handleMovieWatchedEvent(payload: unknown): void {
  const postId = extractPostId(payload);
  if (!postId) {
    return;
  }

  if (shouldHandleCurrentUserEvent(payload)) {
    const watchLog = extractWatchLog(payload);
    const mappedWatchLog = watchLog ? mapApiWatchLog(watchLog) : buildWatchLogFromPayload(payload);
    if (mappedWatchLog) {
      movieStore.applyWatchLog(mappedWatchLog);
    }
  }

  void refreshPostMovieStats(postId);
}

export function handleMovieWatchRemovedEvent(payload: unknown): void {
  const postId = extractPostId(payload);
  if (!postId) {
    return;
  }

  if (shouldHandleCurrentUserEvent(payload)) {
    movieStore.applyWatchLogRemoval(postId);
  }

  void refreshPostMovieStats(postId);
}
