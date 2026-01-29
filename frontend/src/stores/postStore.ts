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
  type?: string;
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
  viewerReactions?: string[];
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
  paginationError: string | null;
  cursor: string | null;
  hasMore: boolean;
}

function createPostStore() {
  const { subscribe, set, update } = writable<PostState>({
    posts: [],
    isLoading: false,
    error: null,
    paginationError: null,
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
        paginationError: null,
      })),
    addPost: (post: Post) =>
      update((state) => {
        const nextPosts = state.posts.filter((existing) => existing.id !== post.id);
        return {
          ...state,
          posts: [post, ...nextPosts],
        };
      }),
    upsertPost: (post: Post) =>
      update((state) => {
        const index = state.posts.findIndex((p) => p.id === post.id);
        if (index === -1) {
          return {
            ...state,
            posts: [post, ...state.posts],
          };
        }
        const updated = [...state.posts];
        updated[index] = { ...updated[index], ...post };
        return {
          ...state,
          posts: updated,
        };
      }),
    updatePostContent: (postId: string, content: string) =>
      update((state) => ({
        ...state,
        posts: state.posts.map((post) =>
          post.id === postId
            ? {
                ...post,
                content,
              }
            : post
        ),
      })),
    appendPosts: (posts: Post[], cursor: string | null, hasMore: boolean) =>
      update((state) => {
        const seen = new Set(state.posts.map((post) => post.id));
        const unique = posts.filter((post) => {
          if (seen.has(post.id)) {
            return false;
          }
          seen.add(post.id);
          return true;
        });
        return {
          ...state,
          posts: [...state.posts, ...unique],
          cursor,
          hasMore,
          isLoading: false,
          error: null,
          paginationError: null,
        };
      }),
    removePost: (postId: string) =>
      update((state) => ({
        ...state,
        posts: state.posts.filter((p) => p.id !== postId),
      })),
    incrementCommentCount: (postId: string, delta: number) =>
      update((state) => ({
        ...state,
        posts: state.posts.map((post) =>
          post.id === postId
            ? {
                ...post,
                commentCount: Math.max(0, (post.commentCount ?? 0) + delta),
              }
            : post
        ),
      })),
    updateReactionCount: (postId: string, emoji: string, delta: number) =>
      update((state) => ({
        ...state,
        posts: state.posts.map((post) => {
          if (post.id !== postId) {
            return post;
          }
          const counts = { ...(post.reactionCounts ?? {}) };
          const next = (counts[emoji] ?? 0) + delta;
          if (next <= 0) {
            delete counts[emoji];
          } else {
            counts[emoji] = next;
          }
          return {
            ...post,
            reactionCounts: counts,
          };
        }),
      })),
    toggleReaction: (postId: string, emoji: string) =>
      update((state) => ({
        ...state,
        posts: state.posts.map((post) => {
          if (post.id !== postId) {
            return post;
          }
          const viewerReactions = new Set(post.viewerReactions ?? []);
          const counts = { ...(post.reactionCounts ?? {}) };

          if (viewerReactions.has(emoji)) {
            viewerReactions.delete(emoji);
            const next = (counts[emoji] ?? 0) - 1;
            if (next <= 0) delete counts[emoji];
            else counts[emoji] = next;
          } else {
            viewerReactions.add(emoji);
            counts[emoji] = (counts[emoji] ?? 0) + 1;
          }

          return {
            ...post,
            reactionCounts: counts,
            viewerReactions: Array.from(viewerReactions),
          };
        }),
      })),
    setLoading: (isLoading: boolean) =>
      update((state) => ({
        ...state,
        isLoading,
        error: isLoading ? null : state.error,
        paginationError: isLoading ? null : state.paginationError,
      })),
    setError: (error: string | null) =>
      update((state) => ({ ...state, error, isLoading: false, paginationError: null })),
    setPaginationError: (error: string | null) =>
      update((state) => ({ ...state, paginationError: error, isLoading: false })),
    reset: () =>
      set({
        posts: [],
        isLoading: false,
        error: null,
        paginationError: null,
        cursor: null,
        hasMore: true,
      }),
    updateUserProfilePicture: (userId: string, profilePictureUrl?: string) =>
      update((state) => ({
        ...state,
        posts: state.posts.map((post) => {
          const shouldUpdate =
            post.user?.id === userId || (!post.user && post.userId === userId);
          if (!shouldUpdate || !post.user) {
            return post;
          }
          return {
            ...post,
            user: {
              ...post.user,
              profilePictureUrl,
            },
          };
        }),
      })),
  };
}

export const postStore = createPostStore();

export const posts = derived(postStore, ($postStore) => $postStore.posts);
export const isLoadingPosts = derived(postStore, ($postStore) => $postStore.isLoading);
export const postsError = derived(postStore, ($postStore) => $postStore.error);
export const postsPaginationError = derived(postStore, ($postStore) => $postStore.paginationError);
export const hasMorePosts = derived(postStore, ($postStore) => $postStore.hasMore);
