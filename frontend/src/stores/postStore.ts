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
  podcast?: PodcastMetadata;
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

export interface MovieSeason {
  seasonNumber: number;
  episodeCount?: number;
  airDate?: string;
  name?: string;
  overview?: string;
  poster?: string;
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
  rottenTomatoesScore?: number;
  metacriticScore?: number;
  trailerKey?: string;
  tmdbId?: number;
  tmdbMediaType?: 'movie' | 'tv';
  seasons?: MovieSeason[];
}

export interface PodcastHighlightEpisode {
  title: string;
  url: string;
  note?: string;
}

export interface PodcastMetadata {
  kind?: 'show' | 'episode';
  highlightEpisodes?: PodcastHighlightEpisode[];
}

export interface PodcastMetadataInput {
  kind?: string;
  highlightEpisodes?: PodcastHighlightEpisode[];
  highlight_episodes?: PodcastHighlightEpisode[];
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

export interface BookStats {
  bookshelfCount: number;
  readCount: number;
  averageRating: number | null;
  ratedCount?: number;
  viewerOnBookshelf?: boolean;
  viewerCategories?: string[];
  viewerRead?: boolean;
  viewerRating?: number | null;
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
  bookStats?: BookStats;
  book_stats?: BookStats;
  createdAt: string;
  updatedAt?: string;
}

export interface CreatePostRequest {
  sectionId: string;
  content: string;
  links?: { url: string; highlights?: Highlight[]; podcast?: PodcastMetadataInput }[];
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

function clampBookCount(value: number): number {
  return Math.max(0, Number.isFinite(value) ? value : 0);
}

function normalizeBookRating(value: number | null | undefined): number | null {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return null;
  }
  return Math.min(5, Math.max(0, value));
}

function getBookStats(post: Post): BookStats {
  const current = post.bookStats ?? post.book_stats;
  const rawRatedCount = current?.ratedCount;
  const ratedCount =
    typeof rawRatedCount === 'number' && Number.isFinite(rawRatedCount)
      ? clampBookCount(rawRatedCount)
      : undefined;
  return {
    bookshelfCount: clampBookCount(current?.bookshelfCount ?? 0),
    readCount: clampBookCount(current?.readCount ?? 0),
    averageRating: normalizeBookRating(current?.averageRating ?? null),
    ...(ratedCount !== undefined ? { ratedCount } : {}),
    viewerOnBookshelf: Boolean(current?.viewerOnBookshelf),
    viewerCategories: current?.viewerCategories ?? [],
    viewerRead: Boolean(current?.viewerRead),
    viewerRating: normalizeBookRating(current?.viewerRating ?? null),
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
    setBookStats: (postId: string, stats: Partial<BookStats>) =>
      update((state) => ({
        ...state,
        posts: state.posts.map((post) => {
          if (post.id !== postId) {
            return post;
          }

          const currentStats = getBookStats(post);
          const nextStats: BookStats = { ...currentStats };

          if ('bookshelfCount' in stats) {
            nextStats.bookshelfCount = clampBookCount(stats.bookshelfCount ?? 0);
          }
          if ('readCount' in stats) {
            nextStats.readCount = clampBookCount(stats.readCount ?? 0);
          }
          if ('averageRating' in stats) {
            nextStats.averageRating = normalizeBookRating(stats.averageRating ?? null);
          }
          if ('ratedCount' in stats) {
            nextStats.ratedCount = clampBookCount(stats.ratedCount ?? 0);
          }
          if ('viewerOnBookshelf' in stats) {
            nextStats.viewerOnBookshelf = Boolean(stats.viewerOnBookshelf);
          }
          if ('viewerCategories' in stats) {
            nextStats.viewerCategories = Array.isArray(stats.viewerCategories)
              ? stats.viewerCategories
              : [];
          }
          if ('viewerRead' in stats) {
            nextStats.viewerRead = Boolean(stats.viewerRead);
          }
          if ('viewerRating' in stats) {
            nextStats.viewerRating = normalizeBookRating(stats.viewerRating ?? null);
          }

          return {
            ...post,
            bookStats: nextStats,
            book_stats: nextStats,
          };
        }),
      })),
    setBookBookshelfState: (
      postId: string,
      viewerOnBookshelf: boolean,
      viewerCategories: string[] = []
    ) =>
      update((state) => ({
        ...state,
        posts: state.posts.map((post) => {
          if (post.id !== postId) {
            return post;
          }

          const currentStats = getBookStats(post);
          let nextBookshelfCount = currentStats.bookshelfCount;

          if (!currentStats.viewerOnBookshelf && viewerOnBookshelf) {
            nextBookshelfCount = clampBookCount(nextBookshelfCount + 1);
          } else if (currentStats.viewerOnBookshelf && !viewerOnBookshelf) {
            nextBookshelfCount = clampBookCount(nextBookshelfCount - 1);
          }

          const normalizedCategories = viewerOnBookshelf
            ? viewerCategories
                .map((category) => category.trim())
                .filter((category) => category.length > 0)
            : [];

          const nextStats: BookStats = {
            ...currentStats,
            bookshelfCount: nextBookshelfCount,
            viewerOnBookshelf,
            viewerCategories: normalizedCategories,
          };

          return {
            ...post,
            bookStats: nextStats,
            book_stats: nextStats,
          };
        }),
      })),
    setBookReadState: (postId: string, viewerRead: boolean, viewerRating: number | null = null) =>
      update((state) => ({
        ...state,
        posts: state.posts.map((post) => {
          if (post.id !== postId) {
            return post;
          }

          const currentStats = getBookStats(post);
          const previousReadCount = clampBookCount(currentStats.readCount);
          const previousViewerRating = normalizeBookRating(currentStats.viewerRating ?? null);
          const inferredRatedCount =
            currentStats.averageRating === null || previousReadCount === 0
              ? 0
              : currentStats.viewerRead
                ? Math.max(1, previousReadCount - 1)
                : previousReadCount;
          const currentRatedCount =
            typeof currentStats.ratedCount === 'number'
              ? clampBookCount(currentStats.ratedCount)
              : inferredRatedCount;
          const nextViewerRating = viewerRead ? normalizeBookRating(viewerRating) : null;
          const previouslyRatedByViewer = currentStats.viewerRead && previousViewerRating !== null;
          const nextRatedByViewer = viewerRead && nextViewerRating !== null;

          let nextReadCount = previousReadCount;
          let nextRatedCount = currentRatedCount;
          let ratingTotal =
            currentStats.averageRating !== null
              ? currentStats.averageRating * currentRatedCount
              : 0;

          if (!currentStats.viewerRead && viewerRead) {
            nextReadCount = clampBookCount(previousReadCount + 1);
          } else if (currentStats.viewerRead && !viewerRead) {
            nextReadCount = clampBookCount(previousReadCount - 1);
          }

          if (!previouslyRatedByViewer && nextRatedByViewer) {
            nextRatedCount = clampBookCount(currentRatedCount + 1);
            ratingTotal += nextViewerRating ?? 0;
          } else if (previouslyRatedByViewer && !nextRatedByViewer) {
            nextRatedCount = clampBookCount(currentRatedCount - 1);
            ratingTotal -= previousViewerRating ?? 0;
          } else if (previouslyRatedByViewer && nextRatedByViewer) {
            nextRatedCount = currentRatedCount;
            ratingTotal += (nextViewerRating ?? 0) - (previousViewerRating ?? 0);
          }

          let nextAverageRating = currentStats.averageRating;
          if (nextReadCount === 0) {
            nextAverageRating = null;
            nextRatedCount = 0;
          } else {
            nextAverageRating = nextRatedCount <= 0 ? null : ratingTotal / nextRatedCount;
          }

          const nextStats: BookStats = {
            ...currentStats,
            readCount: nextReadCount,
            averageRating: nextAverageRating,
            ratedCount: nextRatedCount,
            viewerRead,
            viewerRating: nextViewerRating,
          };

          return {
            ...post,
            bookStats: nextStats,
            book_stats: nextStats,
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
