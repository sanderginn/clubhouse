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

function clampMovieCount(value: number): number {
  return Math.max(0, Number.isFinite(value) ? value : 0);
}

function normalizeMovieRating(value: number | null | undefined): number | null {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return null;
  }
  return Math.min(5, Math.max(0, value));
}

function getMovieStats(post: Post): MovieStats {
  const current = post.movieStats ?? post.movie_stats;
  return {
    watchlistCount: clampMovieCount(current?.watchlistCount ?? 0),
    watchCount: clampMovieCount(current?.watchCount ?? 0),
    averageRating: normalizeMovieRating(current?.averageRating ?? null),
    viewerWatchlisted: Boolean(current?.viewerWatchlisted),
    viewerWatched: Boolean(current?.viewerWatched),
    viewerRating: normalizeMovieRating(current?.viewerRating ?? null),
    viewerCategories: current?.viewerCategories ?? [],
  };
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
    setMovieStats: (postId: string, stats: Partial<MovieStats>) =>
      update((state) => ({
        ...state,
        posts: state.posts.map((post) => {
          if (post.id !== postId) {
            return post;
          }

          const currentStats = getMovieStats(post);
          const nextStats: MovieStats = { ...currentStats };

          if ('watchlistCount' in stats) {
            nextStats.watchlistCount = clampMovieCount(stats.watchlistCount ?? 0);
          }
          if ('watchCount' in stats) {
            nextStats.watchCount = clampMovieCount(stats.watchCount ?? 0);
          }
          if ('averageRating' in stats) {
            nextStats.averageRating = normalizeMovieRating(stats.averageRating ?? null);
          }
          if ('viewerWatchlisted' in stats) {
            nextStats.viewerWatchlisted = Boolean(stats.viewerWatchlisted);
          }
          if ('viewerWatched' in stats) {
            nextStats.viewerWatched = Boolean(stats.viewerWatched);
          }
          if ('viewerRating' in stats) {
            nextStats.viewerRating = normalizeMovieRating(stats.viewerRating ?? null);
          }
          if ('viewerCategories' in stats) {
            nextStats.viewerCategories = Array.isArray(stats.viewerCategories)
              ? stats.viewerCategories
              : [];
          }

          return {
            ...post,
            movieStats: nextStats,
            movie_stats: nextStats,
          };
        }),
      })),
    setMovieWatchlistState: (
      postId: string,
      viewerWatchlisted: boolean,
      viewerCategories: string[] = []
    ) =>
      update((state) => ({
        ...state,
        posts: state.posts.map((post) => {
          if (post.id !== postId) {
            return post;
          }

          const currentStats = getMovieStats(post);
          let nextWatchlistCount = currentStats.watchlistCount;

          if (!currentStats.viewerWatchlisted && viewerWatchlisted) {
            nextWatchlistCount = clampMovieCount(nextWatchlistCount + 1);
          } else if (currentStats.viewerWatchlisted && !viewerWatchlisted) {
            nextWatchlistCount = clampMovieCount(nextWatchlistCount - 1);
          }

          const normalizedCategories = viewerWatchlisted
            ? viewerCategories
                .map((category) => category.trim())
                .filter((category) => category.length > 0)
            : [];

          const nextStats: MovieStats = {
            ...currentStats,
            watchlistCount: nextWatchlistCount,
            viewerWatchlisted,
            viewerCategories: normalizedCategories,
          };

          return {
            ...post,
            movieStats: nextStats,
            movie_stats: nextStats,
          };
        }),
      })),
    setMovieWatchState: (
      postId: string,
      viewerWatched: boolean,
      viewerRating: number | null = null
    ) =>
      update((state) => ({
        ...state,
        posts: state.posts.map((post) => {
          if (post.id !== postId) {
            return post;
          }

          const currentStats = getMovieStats(post);
          const previousWatchCount = clampMovieCount(currentStats.watchCount);
          const previousViewerRating = normalizeMovieRating(currentStats.viewerRating ?? null);
          const nextViewerRating = viewerWatched ? normalizeMovieRating(viewerRating) : null;

          let nextWatchCount = previousWatchCount;
          let ratingTotal =
            previousWatchCount > 0 && currentStats.averageRating !== null
              ? currentStats.averageRating * previousWatchCount
              : 0;

          if (!currentStats.viewerWatched && viewerWatched) {
            nextWatchCount = clampMovieCount(previousWatchCount + 1);
            ratingTotal += nextViewerRating ?? 0;
          } else if (currentStats.viewerWatched && !viewerWatched) {
            nextWatchCount = clampMovieCount(previousWatchCount - 1);
            ratingTotal -= previousViewerRating ?? 0;
          } else if (
            currentStats.viewerWatched &&
            viewerWatched &&
            previousViewerRating !== null &&
            nextViewerRating !== null
          ) {
            ratingTotal += nextViewerRating - previousViewerRating;
          }

          const nextAverageRating = nextWatchCount <= 0 ? null : ratingTotal / nextWatchCount;

          const nextStats: MovieStats = {
            ...currentStats,
            watchCount: nextWatchCount,
            averageRating: nextAverageRating,
            viewerWatched,
            viewerRating: nextViewerRating,
          };

          return {
            ...post,
            movieStats: nextStats,
            movie_stats: nextStats,
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
