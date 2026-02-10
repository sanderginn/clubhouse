import { derived, get, writable } from 'svelte/store';
import { api, type PostPodcastSaveInfo } from '../services/api';
import type { Post } from './postStore';

const DEFAULT_PAGE_SIZE = 20;

export interface PodcastStoreState {
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

export const podcastSaveInfoByPostId = derived(podcastStore, ($store) => $store.saveInfoByPostId);

export const savedPodcastPostIds = derived(podcastStore, ($store) => $store.savedPostIds);

export const savedPodcastPosts = derived(podcastStore, ($store) => $store.savedPosts);
