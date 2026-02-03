import { writable, derived } from 'svelte/store';

export interface Highlight {
  timestamp: number;
  label?: string;
}

export interface Link {
  id?: string;
  url: string;
  metadata?: LinkMetadata;
  highlights?: Highlight[];
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
  embed?: LinkEmbed;
  type?: string;
  recipe?: RecipeMetadata;
}

export interface LinkEmbed {
  url: string;
  provider?: string;
  type?: string;
  height?: number;
  width?: number;
}

export interface RecipeNutritionInfo {
  calories?: string;
  servings?: string;
}

export interface RecipeMetadata {
  name?: string;
  description?: string;
  image?: string;
  ingredients?: string[];
  instructions?: string[];
  prep_time?: string;
  cook_time?: string;
  total_time?: string;
  yield?: string;
  author?: string;
  date_published?: string;
  cuisine?: string;
  category?: string;
  nutrition?: RecipeNutritionInfo;
}

export interface PostImage {
  id: string;
  url: string;
  position: number;
  caption?: string;
  altText?: string;
  createdAt?: string;
}

export interface RecipeStats {
  saveCount: number;
  cookCount: number;
  averageRating: number | null;
}

export interface Post {
  id: string;
  userId: string;
  sectionId: string;
  content: string;
  links?: Link[];
  images?: PostImage[];
  user?: {
    id: string;
    username: string;
    profilePictureUrl?: string;
  };
  reactionCounts?: Record<string, number>;
  viewerReactions?: string[];
  commentCount?: number;
  recipeStats?: RecipeStats;
  recipe_stats?: RecipeStats;
  createdAt: string;
  updatedAt?: string;
}

export interface CreatePostRequest {
  sectionId: string;
  content: string;
  links?: { url: string; highlights?: Highlight[] }[];
  images?: { url: string; caption?: string; altText?: string }[];
  mentionUsernames?: string[];
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
        posts: state.posts.map((post) =>
          post.id === postId
            ? {
                ...post,
                reactionCounts: {
                  ...(post.reactionCounts ?? {}),
                  [emoji]: Math.max(0, (post.reactionCounts?.[emoji] ?? 0) + delta),
                },
              }
            : post
        ),
      })),
    toggleReaction: (postId: string, emoji: string) =>
      update((state) => ({
        ...state,
        posts: state.posts.map((post) => {
          if (post.id !== postId) return post;
          const reactions = post.viewerReactions ?? [];
          const hasReaction = reactions.includes(emoji);
          return {
            ...post,
            viewerReactions: hasReaction
              ? reactions.filter((reaction) => reaction !== emoji)
              : [...reactions, emoji],
          };
        }),
      })),
  };
}

export const postStore = createPostStore();

export const sortedPosts = derived(postStore, ($store) =>
  [...$store.posts].sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime())
);

export const isLoadingPosts = derived(postStore, ($store) => $store.isLoading);

export const postError = derived(postStore, ($store) => $store.error);

export const postPaginationError = derived(postStore, ($store) => $store.paginationError);

export const hasMorePosts = derived(postStore, ($store) => $store.hasMore);

export const postCursor = derived(postStore, ($store) => $store.cursor);
