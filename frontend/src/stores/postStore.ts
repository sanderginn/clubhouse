import { writable, derived } from 'svelte/store';

export interface Highlight {
  id?: string;
  timestamp: number;
  label?: string;
  heartCount?: number;
  viewerReacted?: boolean;
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
  embed?: EmbedData;
  type?: string;
  recipe?: RecipeMetadata;
  movie?: MovieMetadata;
}

export interface EmbedData {
  type?: string;
  provider?: string;
  embedUrl: string;
  width?: number;
  height?: number;
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

export interface MovieCastMember {
  name: string;
  character?: string;
}

export interface MovieMetadata {
  title?: string;
  overview?: string;
  poster?: string;
  backdrop?: string;
  runtime?: number;
  genres?: string[];
  releaseDate?: string;
  cast?: MovieCastMember[];
  director?: string;
  tmdbRating?: number;
  trailerKey?: string;
  tmdbId?: number;
  tmdbMediaType?: 'movie' | 'tv';
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

export interface MovieStats {
  watchlistCount: number;
  watchCount: number;
  averageRating: number | null;
  viewerWatchlisted?: boolean;
  viewerWatched?: boolean;
  viewerRating?: number | null;
  viewerCategories?: string[];
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
  movieStats?: MovieStats;
  movie_stats?: MovieStats;
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
    updateRecipeSaveCount: (postId: string, delta: number) =>
      update((state) => ({
        ...state,
        posts: state.posts.map((post) => {
          if (post.id !== postId) {
            return post;
          }
          const currentStats = post.recipeStats ?? post.recipe_stats ?? {
            saveCount: 0,
            cookCount: 0,
            averageRating: null,
          };
          const nextSaveCount = Math.max(0, currentStats.saveCount + delta);
          const nextStats = { ...currentStats, saveCount: nextSaveCount };
          return {
            ...post,
            recipeStats: nextStats,
            recipe_stats: nextStats,
          };
        }),
      })),
    setRecipeSaveCount: (postId: string, saveCount: number) =>
      update((state) => ({
        ...state,
        posts: state.posts.map((post) => {
          if (post.id !== postId) {
            return post;
          }
          const currentStats = post.recipeStats ?? post.recipe_stats ?? {
            saveCount: 0,
            cookCount: 0,
            averageRating: null,
          };
          const nextSaveCount = Math.max(0, Number.isFinite(saveCount) ? saveCount : 0);
          const nextStats = { ...currentStats, saveCount: nextSaveCount };
          return {
            ...post,
            recipeStats: nextStats,
            recipe_stats: nextStats,
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
    updateHighlightReaction: (
      postId: string,
      linkId: string,
      highlightId: string,
      delta: number,
      viewerReacted?: boolean
    ) =>
      update((state) => ({
        ...state,
        posts: state.posts.map((post) => {
          if (post.id !== postId) {
            return post;
          }
          if (!post.links) {
            return post;
          }
          const links = post.links.map((link) => {
            if (link.id !== linkId || !link.highlights) {
              return link;
            }
            const highlights = link.highlights.map((highlight) => {
              if (highlight.id !== highlightId) {
                return highlight;
              }
              const nextCount = Math.max(0, (highlight.heartCount ?? 0) + delta);
              return {
                ...highlight,
                heartCount: nextCount,
                viewerReacted: viewerReacted ?? highlight.viewerReacted,
              };
            });
            return {
              ...link,
              highlights,
            };
          });
          return {
            ...post,
            links,
          };
        }),
      })),
    updateLinkMetadata: (postId: string, linkId: string, metadata: LinkMetadata) =>
      update((state) => {
        const postIndex = state.posts.findIndex((post) => post.id === postId);
        if (postIndex === -1) {
          return state;
        }

        const post = state.posts[postIndex];
        if (!post.links) {
          return state;
        }

        const linkIndex = post.links.findIndex((link) => link.id === linkId);
        if (linkIndex === -1) {
          return state;
        }

        const updatedLinks = [...post.links];
        updatedLinks[linkIndex] = {
          ...updatedLinks[linkIndex],
          metadata,
        };

        const updatedPosts = [...state.posts];
        updatedPosts[postIndex] = {
          ...post,
          links: updatedLinks,
        };

        return {
          ...state,
          posts: updatedPosts,
        };
      }),
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
export const postCursor = derived(postStore, ($postStore) => $postStore.cursor);
