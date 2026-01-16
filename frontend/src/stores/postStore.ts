import { writable, derived } from 'svelte/store';

export interface Link {
  id?: string;
  url: string;
  metadata?: LinkMetadata;
}

export interface LinkMetadata {
  url: string;
  provider?: string;
  title?: string;
  description?: string;
  image?: string;
  author?: string;
  duration?: number;
  embedUrl?: string;
}

export interface Post {
  id: string;
  userId: string;
  sectionId: string;
  content: string;
  links?: Link[];
  user?: {
    id: string;
    username: string;
    profilePictureUrl?: string;
  };
  reactionCounts?: Record<string, number>;
  commentCount?: number;
  createdAt: string;
  updatedAt?: string;
}

export interface CreatePostRequest {
  sectionId: string;
  content: string;
  links?: { url: string }[];
}

interface PostState {
  posts: Post[];
  isLoading: boolean;
  error: string | null;
  cursor: string | null;
  hasMore: boolean;
}

function createPostStore() {
  const { subscribe, set, update } = writable<PostState>({
    posts: [],
    isLoading: false,
    error: null,
    cursor: null,
    hasMore: true,
  });

  return {
    subscribe,
    setPosts: (posts: Post[], cursor: string | null, hasMore: boolean) =>
      update((state) => ({
        ...state,
        posts,
        cursor,
        hasMore,
        isLoading: false,
        error: null,
      })),
    addPost: (post: Post) =>
      update((state) => ({
        ...state,
        posts: [post, ...state.posts],
      })),
    appendPosts: (posts: Post[], cursor: string | null, hasMore: boolean) =>
      update((state) => ({
        ...state,
        posts: [...state.posts, ...posts],
        cursor,
        hasMore,
        isLoading: false,
      })),
    removePost: (postId: string) =>
      update((state) => ({
        ...state,
        posts: state.posts.filter((p) => p.id !== postId),
      })),
    setLoading: (isLoading: boolean) => update((state) => ({ ...state, isLoading })),
    setError: (error: string | null) => update((state) => ({ ...state, error, isLoading: false })),
    reset: () =>
      set({
        posts: [],
        isLoading: false,
        error: null,
        cursor: null,
        hasMore: true,
      }),
  };
}

export const postStore = createPostStore();

export const posts = derived(postStore, ($postStore) => $postStore.posts);
export const isLoadingPosts = derived(postStore, ($postStore) => $postStore.isLoading);
export const postsError = derived(postStore, ($postStore) => $postStore.error);
export const hasMorePosts = derived(postStore, ($postStore) => $postStore.hasMore);
