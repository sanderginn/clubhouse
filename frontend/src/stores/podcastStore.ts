import { derived, get, writable } from 'svelte/store';
import { api, type PostPodcastSaveInfo, type RecentPodcastItem } from '../services/api';
import type { Post } from './postStore';

const DEFAULT_PAGE_SIZE = 20;

export interface PodcastStoreState {
  recentItems: RecentPodcastItem[];
  recentSectionId: string | null;
  recentCursor: string | null;
  recentHasMore: boolean;
  isLoadingRecent: boolean;
  recentError: string | null;
  saveInfoByPostId: Record<string, PostPodcastSaveInfo>;
  savedPostIds: Set<string>;
  savedPosts: Post[];
  sectionId: string | null;
  cursor: string | null;
  hasMore: boolean;
  isLoadingSaved: boolean;
  isLoadingSaveInfo: boolean;
  isTogglingSave: boolean;
  error: string | null;
}

const initialState: PodcastStoreState = {
  recentItems: [],
  recentSectionId: null,
  recentCursor: null,
  recentHasMore: false,
  isLoadingRecent: false,
  recentError: null,
  saveInfoByPostId: {},
  savedPostIds: new Set(),
  savedPosts: [],
  sectionId: null,
  cursor: null,
  hasMore: false,
  isLoadingSaved: false,
  isLoadingSaveInfo: false,
  isTogglingSave: false,
  error: null,
};

function dedupePosts(existing: Post[], nextPosts: Post[]): Post[] {
  const seen = new Set(existing.map((post) => post.id));
  const merged = [...existing];
  for (const post of nextPosts) {
    if (seen.has(post.id)) {
      continue;
    }
    seen.add(post.id);
    merged.push(post);
  }
  return merged;
}

function dedupeRecentPodcasts(
  existing: RecentPodcastItem[],
  nextItems: RecentPodcastItem[]
): RecentPodcastItem[] {
  const seen = new Set(existing.map((item) => item.linkId));
  const merged = [...existing];
  for (const item of nextItems) {
    if (seen.has(item.linkId)) {
      continue;
    }
    seen.add(item.linkId);
    merged.push(item);
  }
  return merged;
}

function buildSavedPostIds(state: PodcastStoreState, posts: Post[]): Set<string> {
  const ids = new Set<string>();

  for (const [postId, info] of Object.entries(state.saveInfoByPostId)) {
    if (info.viewerSaved) {
      ids.add(postId);
    }
  }

  for (const post of posts) {
    ids.add(post.id);
  }

  return ids;
}

function isPostSaved(state: PodcastStoreState, postId: string): boolean {
  return state.saveInfoByPostId[postId]?.viewerSaved ?? state.savedPostIds.has(postId);
}

function createPodcastStore() {
  const { subscribe, update, set } = writable<PodcastStoreState>({
    ...initialState,
    savedPostIds: new Set(),
  });

  return {
    subscribe,
    setLoadingRecent: (isLoading: boolean) =>
      update((state) => ({
        ...state,
        isLoadingRecent: isLoading,
        recentError: isLoading ? null : state.recentError,
      })),
    setRecentError: (error: string | null) =>
      update((state) => ({
        ...state,
        recentError: error,
        isLoadingRecent: false,
      })),
    setRecentItems: (
      items: RecentPodcastItem[],
      cursor: string | null,
      hasMore: boolean,
      sectionId: string
    ) =>
      update((state) => ({
        ...state,
        recentItems: items,
        recentCursor: cursor,
        recentHasMore: hasMore,
        recentSectionId: sectionId,
        isLoadingRecent: false,
        recentError: null,
      })),
    appendRecentItems: (items: RecentPodcastItem[], cursor: string | null, hasMore: boolean) =>
      update((state) => ({
        ...state,
        recentItems: dedupeRecentPodcasts(state.recentItems, items),
        recentCursor: cursor,
        recentHasMore: hasMore,
        isLoadingRecent: false,
        recentError: null,
      })),
    setLoadingSaved: (isLoading: boolean) =>
      update((state) => ({
        ...state,
        isLoadingSaved: isLoading,
        error: isLoading ? null : state.error,
      })),
    setLoadingSaveInfo: (isLoading: boolean) =>
      update((state) => ({
        ...state,
        isLoadingSaveInfo: isLoading,
        error: isLoading ? null : state.error,
      })),
    setTogglingSave: (isLoading: boolean) =>
      update((state) => ({
        ...state,
        isTogglingSave: isLoading,
        error: isLoading ? null : state.error,
      })),
    setError: (error: string | null) =>
      update((state) => ({
        ...state,
        error,
        isLoadingSaved: false,
        isLoadingSaveInfo: false,
        isTogglingSave: false,
      })),
    setPostSaveInfo: (postId: string, info: PostPodcastSaveInfo) =>
      update((state) => {
        const nextInfo = {
          ...state.saveInfoByPostId,
          [postId]: info,
        };
        const nextSavedIds = new Set(state.savedPostIds);
        if (info.viewerSaved) {
          nextSavedIds.add(postId);
        } else {
          nextSavedIds.delete(postId);
        }

        return {
          ...state,
          saveInfoByPostId: nextInfo,
          savedPostIds: nextSavedIds,
          isLoadingSaveInfo: false,
          isTogglingSave: false,
          error: null,
        };
      }),
    setSavedPosts: (
      posts: Post[],
      cursor: string | null,
      hasMore: boolean,
      sectionId: string
    ) =>
      update((state) => ({
        ...state,
        savedPosts: posts,
        savedPostIds: buildSavedPostIds(state, posts),
        cursor,
        hasMore,
        sectionId,
        isLoadingSaved: false,
        error: null,
      })),
    appendSavedPosts: (posts: Post[], cursor: string | null, hasMore: boolean) =>
      update((state) => {
        const mergedPosts = dedupePosts(state.savedPosts, posts);
        return {
          ...state,
          savedPosts: mergedPosts,
          savedPostIds: buildSavedPostIds(state, mergedPosts),
          cursor,
          hasMore,
          isLoadingSaved: false,
          error: null,
        };
      }),
    removeSavedPost: (postId: string) =>
      update((state) => ({
        ...state,
        savedPosts: state.savedPosts.filter((post) => post.id !== postId),
        savedPostIds: new Set([...state.savedPostIds].filter((id) => id !== postId)),
      })),
    loadRecentPodcasts: async (sectionId: string, limit = DEFAULT_PAGE_SIZE): Promise<void> => {
      podcastStore.setLoadingRecent(true);
      try {
        const response = await api.getSectionRecentPodcasts(sectionId, limit);
        podcastStore.setRecentItems(
          response.items ?? [],
          response.nextCursor ?? null,
          response.hasMore ?? false,
          sectionId
        );
      } catch (error) {
        podcastStore.setRecentError(
          error instanceof Error ? error.message : 'Failed to load recent podcasts'
        );
      }
    },
    loadMoreRecentPodcasts: async (limit = DEFAULT_PAGE_SIZE): Promise<void> => {
      const state = get(podcastStore);
      if (
        !state.recentSectionId ||
        !state.recentCursor ||
        !state.recentHasMore ||
        state.isLoadingRecent
      ) {
        return;
      }

      podcastStore.setLoadingRecent(true);
      try {
        const response = await api.getSectionRecentPodcasts(
          state.recentSectionId,
          limit,
          state.recentCursor
        );
        podcastStore.appendRecentItems(
          response.items ?? [],
          response.nextCursor ?? null,
          response.hasMore ?? false
        );
      } catch (error) {
        podcastStore.setRecentError(
          error instanceof Error ? error.message : 'Failed to load more recent podcasts'
        );
      }
    },
    loadPostSaveInfo: async (postId: string): Promise<void> => {
      podcastStore.setLoadingSaveInfo(true);
      try {
        const info = await api.getPostPodcastSaveInfo(postId);
        podcastStore.setPostSaveInfo(postId, info);
      } catch (error) {
        podcastStore.setError(
          error instanceof Error ? error.message : 'Failed to load podcast save info'
        );
      }
    },
    toggleSave: async (postId: string): Promise<void> => {
      const state = get(podcastStore);
      const currentlySaved = isPostSaved(state, postId);
      podcastStore.setTogglingSave(true);

      try {
        if (currentlySaved) {
          await api.unsavePodcast(postId);
        } else {
          await api.savePodcast(postId);
        }

        const info = await api.getPostPodcastSaveInfo(postId);
        podcastStore.setPostSaveInfo(postId, info);
        if (!info.viewerSaved) {
          podcastStore.removeSavedPost(postId);
        }
      } catch (error) {
        podcastStore.setError(error instanceof Error ? error.message : 'Failed to update podcast save');
      }
    },
    loadSavedPodcasts: async (sectionId: string, limit = DEFAULT_PAGE_SIZE): Promise<void> => {
      podcastStore.setLoadingSaved(true);
      try {
        const response = await api.getSectionSavedPodcasts(sectionId, limit);
        podcastStore.setSavedPosts(
          response.posts ?? [],
          response.nextCursor ?? null,
          response.hasMore ?? false,
          sectionId
        );
      } catch (error) {
        podcastStore.setError(
          error instanceof Error ? error.message : 'Failed to load saved podcasts'
        );
      }
    },
    loadMoreSavedPodcasts: async (limit = DEFAULT_PAGE_SIZE): Promise<void> => {
      const state = get(podcastStore);
      if (!state.sectionId || !state.cursor || !state.hasMore || state.isLoadingSaved) {
        return;
      }

      podcastStore.setLoadingSaved(true);
      try {
        const response = await api.getSectionSavedPodcasts(state.sectionId, limit, state.cursor);
        podcastStore.appendSavedPosts(
          response.posts ?? [],
          response.nextCursor ?? null,
          response.hasMore ?? false
        );
      } catch (error) {
        podcastStore.setError(
          error instanceof Error ? error.message : 'Failed to load more saved podcasts'
        );
      }
    },
    isPostSaved: (postId: string): boolean => isPostSaved(get(podcastStore), postId),
    reset: (): void =>
      set({
        ...initialState,
        savedPostIds: new Set(),
      }),
  };
}

export const podcastStore = createPodcastStore();

export const recentPodcastItems = derived(podcastStore, ($store) => $store.recentItems);

export const podcastSaveInfoByPostId = derived(podcastStore, ($store) => $store.saveInfoByPostId);

export const savedPodcastPostIds = derived(podcastStore, ($store) => $store.savedPostIds);

export const savedPodcastPosts = derived(podcastStore, ($store) => $store.savedPosts);
