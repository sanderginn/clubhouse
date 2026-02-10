import type {
  Post,
  CreatePostRequest,
  LinkMetadata,
  PodcastMetadataInput,
  PodcastMetadata,
  PodcastHighlightEpisode,
} from '../stores/postStore';
import type { CreateCommentRequest, Comment } from '../stores/commentStore';
import type { SectionLink } from '../stores/sectionLinksStore';
import { mapApiComment, type ApiComment } from '../stores/commentMapper';
import { mapApiPost, type ApiPost } from '../stores/postMapper';
import { logError, logWarn } from '../lib/observability/logger';
import { recordApiTiming } from '../lib/observability/performance';
import { context, propagation, trace, SpanStatusCode } from '@opentelemetry/api';

const API_BASE = '/api/v1';
const CSRF_ENDPOINT = '/auth/csrf';
const CSRF_HEADER = 'X-CSRF-Token';
const CSRF_EXEMPT_ENDPOINTS = new Set([
  '/auth/login',
  '/auth/register',
  '/auth/password-reset/redeem',
]);
const CSRF_ERROR_CODES = new Set(['CSRF_TOKEN_REQUIRED', 'INVALID_CSRF_TOKEN']);
const PODCAST_KIND_SELECTION_REQUIRED_CODE = 'PODCAST_KIND_SELECTION_REQUIRED';
const PODCAST_KIND_SELECTION_REQUIRED_MESSAGE =
  'Could not determine whether this podcast link is a show or an episode. Please select one and try again.';

interface ApiError {
  error: string;
  code: string;
  mfa_required?: boolean;
  mfaRequired?: boolean;
}

type ApiClientError = Error & {
  code?: string;
  mfaRequired?: boolean;
  podcastKindSelectionRequired?: boolean;
};

interface ApiResponse<T> {
  data: T;
  meta?: {
    cursor?: string;
    hasMore?: boolean;
  };
}

interface ApiSectionLink {
  id: string;
  url: string;
  metadata?: LinkMetadata;
  post_id: string;
  user_id: string;
  username: string;
  created_at: string;
}

interface ApiSectionLinksResponse {
  links: ApiSectionLink[];
  has_more?: boolean;
  next_cursor?: string | null;
}

type PostHighlightRequest = {
  timestamp: number;
  label?: string;
};

type PostLinkRequest = {
  url: string;
  highlights?: PostHighlightRequest[];
  podcast?: PodcastMetadataInput;
};

type ApiPodcastHighlightEpisodeRequest = {
  title: string;
  url: string;
  note?: string;
};

type ApiPodcastMetadataRequest = {
  kind?: string;
  highlight_episodes?: ApiPodcastHighlightEpisodeRequest[];
};

type ApiPostLinkRequest = {
  url: string;
  highlights?: PostHighlightRequest[];
  podcast?: ApiPodcastMetadataRequest;
};

interface ApiWatchlistItem {
  id: string;
  user_id: string;
  post_id: string;
  category: string;
  created_at: string;
  post?: ApiPost;
}

interface ApiWatchlistCategory {
  id: string;
  name: string;
  position: number;
}

interface ApiPostWatchlistInfo {
  save_count: number;
  users: ApiReactionUser[];
  viewer_saved: boolean;
  viewer_categories?: string[];
}

interface ApiPodcastSave {
  id: string;
  user_id: string;
  post_id: string;
  created_at: string;
  deleted_at?: string | null;
}

interface ApiPostPodcastSaveInfo {
  save_count: number;
  users: ApiReactionUser[];
  viewer_saved: boolean;
}

interface ApiPodcastHighlightEpisodeResponse {
  title?: string;
  url?: string;
  note?: string | null;
}

interface ApiPodcastMetadataResponse {
  kind?: string;
  highlight_episodes?: ApiPodcastHighlightEpisodeResponse[];
  highlightEpisodes?: ApiPodcastHighlightEpisodeResponse[];
}

interface ApiRecentPodcastItem {
  post_id: string;
  link_id: string;
  url: string;
  podcast?: ApiPodcastMetadataResponse;
  user_id: string;
  username: string;
  post_created_at: string;
  link_created_at: string;
}

interface ApiSectionRecentPodcastsResponse {
  items?: ApiRecentPodcastItem[];
  has_more?: boolean;
  next_cursor?: string | null;
}

interface ApiWatchLog {
  id: string;
  user_id: string;
  post_id: string;
  rating: number;
  notes?: string | null;
  watched_at: string;
  post?: ApiPost;
}

interface ApiWatchLogUser {
  id: string;
  username: string;
  profile_picture_url?: string | null;
}

interface ApiWatchLogResponse {
  watch_log: ApiWatchLog;
  user: ApiWatchLogUser;
}

interface ApiPostWatchLogsResponse {
  watch_count: number;
  avg_rating?: number | null;
  logs: ApiWatchLogResponse[];
  viewer_watched: boolean;
  viewer_rating?: number | null;
}

interface ApiBookshelfCategory {
  id: string;
  name: string;
  position: number;
}

interface ApiBookshelfItem {
  id: string;
  user_id: string;
  post_id: string;
  category_id?: string | null;
  created_at: string;
  deleted_at?: string | null;
}

interface ApiBookshelfResponse {
  bookshelf_items: ApiBookshelfItem[];
  next_cursor?: string | null;
}

interface ApiReadLog {
  id: string;
  user_id: string;
  post_id: string;
  rating?: number | null;
  created_at: string;
  deleted_at?: string | null;
}

interface ApiReadLogReader {
  id: string;
  username: string;
  profile_picture_url?: string | null;
  rating?: number | null;
}

interface ApiPostReadLogsResponse {
  read_count: number;
  average_rating: number;
  viewer_read: boolean;
  viewer_rating?: number | null;
  readers: ApiReadLogReader[];
}

interface ApiBookQuote {
  id: string;
  post_id: string;
  user_id: string;
  quote_text: string;
  page_number?: number | null;
  chapter?: string | null;
  note?: string | null;
  created_at: string;
  updated_at: string;
  deleted_at?: string | null;
}

interface ApiBookQuoteWithUser extends ApiBookQuote {
  username: string;
  display_name: string;
}

interface SectionLinksResponse {
  links: SectionLink[];
  hasMore: boolean;
  nextCursor: string | null;
}

function mapApiSectionLink(link: ApiSectionLink): SectionLink {
  return {
    id: link.id,
    url: link.url,
    metadata: link.metadata,
    postId: link.post_id,
    userId: link.user_id,
    username: link.username,
    createdAt: link.created_at,
  };
}

function toApiClientError(errorData: ApiError | null, fallbackMessage: string): ApiClientError {
  const code = errorData?.code ?? 'UNKNOWN_ERROR';
  const podcastKindSelectionRequired = code === PODCAST_KIND_SELECTION_REQUIRED_CODE;
  const message = podcastKindSelectionRequired
    ? PODCAST_KIND_SELECTION_REQUIRED_MESSAGE
    : errorData?.error ?? fallbackMessage;

  const error = new Error(message) as ApiClientError;
  error.code = code;
  error.mfaRequired = errorData?.mfa_required ?? errorData?.mfaRequired ?? false;
  error.podcastKindSelectionRequired = podcastKindSelectionRequired;
  return error;
}

function mapPodcastMetadataRequest(
  podcast?: PodcastMetadataInput
): ApiPodcastMetadataRequest | undefined {
  if (!podcast) {
    return undefined;
  }

  const kind =
    typeof podcast.kind === 'string' && podcast.kind.trim().length > 0
      ? podcast.kind.trim().toLowerCase()
      : undefined;
  const rawHighlightEpisodes = podcast.highlight_episodes ?? podcast.highlightEpisodes;
  const highlightEpisodes = Array.isArray(rawHighlightEpisodes)
    ? rawHighlightEpisodes
        .map((episode): ApiPodcastHighlightEpisodeRequest | null => {
          const title =
            typeof episode?.title === 'string' && episode.title.trim().length > 0
              ? episode.title.trim()
              : undefined;
          const url =
            typeof episode?.url === 'string' && episode.url.trim().length > 0
              ? episode.url.trim()
              : undefined;
          if (!title || !url) {
            return null;
          }
          const note =
            typeof episode?.note === 'string' && episode.note.trim().length > 0
              ? episode.note.trim()
              : undefined;
          return {
            title,
            url,
            ...(note ? { note } : {}),
          };
        })
        .filter((episode): episode is ApiPodcastHighlightEpisodeRequest => episode !== null)
    : undefined;

  if (!kind && !(highlightEpisodes && highlightEpisodes.length > 0)) {
    return {};
  }

  return {
    ...(kind ? { kind } : {}),
    ...(highlightEpisodes && highlightEpisodes.length > 0
      ? { highlight_episodes: highlightEpisodes }
      : {}),
  };
}

function mapPostLinkRequest(link: PostLinkRequest): ApiPostLinkRequest {
  const podcast = mapPodcastMetadataRequest(link.podcast);
  return {
    url: link.url,
    ...(link.highlights && link.highlights.length > 0 ? { highlights: link.highlights } : {}),
    ...(podcast ? { podcast } : {}),
  };
}

function mapApiWatchlistItem(item: ApiWatchlistItem): WatchlistItem {
  return {
    id: item.id,
    userId: item.user_id,
    postId: item.post_id,
    category: item.category,
    createdAt: item.created_at,
  };
}

function mapApiWatchlistItemWithPost(item: ApiWatchlistItem): WatchlistItemWithPost {
  return {
    ...mapApiWatchlistItem(item),
    post: item.post,
  };
}

function mapApiWatchlistCategory(category: ApiWatchlistCategory): WatchlistCategory {
  return {
    id: category.id,
    name: category.name,
    position: category.position,
  };
}

function mapApiWatchlistUser(user: ApiReactionUser): WatchlistUser {
  return {
    id: user.id,
    username: user.username,
    displayName: user.username,
    avatar: user.profile_picture_url ?? undefined,
  };
}

function mapApiPodcastSave(save: ApiPodcastSave): PodcastSave {
  return {
    id: save.id,
    userId: save.user_id,
    postId: save.post_id,
    createdAt: save.created_at,
    deletedAt: save.deleted_at ?? undefined,
  };
}

function mapApiPodcastSaveUser(user: ApiReactionUser): PodcastSaveUser {
  return {
    id: user.id,
    username: user.username,
    displayName: user.username,
    avatar: user.profile_picture_url ?? undefined,
  };
}

function mapApiPodcastHighlightEpisode(
  episode: ApiPodcastHighlightEpisodeResponse
): PodcastHighlightEpisode | null {
  const title = typeof episode.title === 'string' ? episode.title.trim() : '';
  const url = typeof episode.url === 'string' ? episode.url.trim() : '';
  if (!title || !url) {
    return null;
  }

  const note =
    typeof episode.note === 'string' && episode.note.trim().length > 0
      ? episode.note.trim()
      : undefined;
  return {
    title,
    url,
    ...(note ? { note } : {}),
  };
}

function mapApiPodcastMetadata(podcast?: ApiPodcastMetadataResponse): PodcastMetadata {
  const rawKind = typeof podcast?.kind === 'string' ? podcast.kind.trim().toLowerCase() : '';
  const kind = rawKind === 'show' || rawKind === 'episode' ? rawKind : undefined;
  const rawEpisodes = podcast?.highlight_episodes ?? podcast?.highlightEpisodes;
  const highlightEpisodes = Array.isArray(rawEpisodes)
    ? rawEpisodes
        .map(mapApiPodcastHighlightEpisode)
        .filter((episode): episode is PodcastHighlightEpisode => episode !== null)
    : undefined;

  return {
    ...(kind ? { kind } : {}),
    ...(highlightEpisodes && highlightEpisodes.length > 0 ? { highlightEpisodes } : {}),
  };
}

function mapApiRecentPodcastItem(item: ApiRecentPodcastItem): RecentPodcastItem {
  return {
    postId: item.post_id,
    linkId: item.link_id,
    url: item.url,
    podcast: mapApiPodcastMetadata(item.podcast),
    userId: item.user_id,
    username: item.username,
    postCreatedAt: item.post_created_at,
    linkCreatedAt: item.link_created_at,
  };
}

function mapApiWatchLogUser(user: ApiWatchLogUser): WatchLogUser {
  return {
    id: user.id,
    username: user.username,
    displayName: user.username,
    avatar: user.profile_picture_url ?? undefined,
  };
}

function mapApiWatchLog(log: ApiWatchLog, user?: ApiWatchLogUser): WatchLog {
  return {
    id: log.id,
    userId: log.user_id,
    postId: log.post_id,
    rating: log.rating,
    notes: log.notes ?? undefined,
    watchedAt: log.watched_at,
    user: user ? mapApiWatchLogUser(user) : undefined,
  };
}

function mapApiWatchLogWithPost(log: ApiWatchLog): WatchLogWithPost {
  return {
    ...mapApiWatchLog(log),
    post: log.post,
  };
}

function mapApiBookshelfCategory(category: ApiBookshelfCategory): BookshelfCategory {
  return {
    id: category.id,
    name: category.name,
    position: category.position,
  };
}

function mapApiBookshelfItem(item: ApiBookshelfItem): BookshelfItem {
  return {
    id: item.id,
    userId: item.user_id,
    postId: item.post_id,
    categoryId: item.category_id ?? undefined,
    createdAt: item.created_at,
    deletedAt: item.deleted_at ?? undefined,
  };
}

function mapApiReadLog(log: ApiReadLog): ReadLog {
  return {
    id: log.id,
    userId: log.user_id,
    postId: log.post_id,
    rating: log.rating ?? undefined,
    createdAt: log.created_at,
    deletedAt: log.deleted_at ?? undefined,
  };
}

function mapApiReadLogReader(reader: ApiReadLogReader): ReadLogReader {
  return {
    id: reader.id,
    username: reader.username,
    displayName: reader.username,
    avatar: reader.profile_picture_url ?? undefined,
    rating: reader.rating ?? undefined,
  };
}

function mapApiBookQuote(quote: ApiBookQuote): BookQuote {
  return {
    id: quote.id,
    postId: quote.post_id,
    userId: quote.user_id,
    quoteText: quote.quote_text,
    pageNumber: quote.page_number ?? undefined,
    chapter: quote.chapter ?? undefined,
    note: quote.note ?? undefined,
    createdAt: quote.created_at,
    updatedAt: quote.updated_at,
    deletedAt: quote.deleted_at ?? undefined,
  };
}

function mapApiBookQuoteWithUser(quote: ApiBookQuoteWithUser): BookQuoteWithUser {
  return {
    ...mapApiBookQuote(quote),
    username: quote.username,
    displayName: quote.display_name,
  };
}

interface LogOptions {
  suppressStatuses?: number[];
}

export interface ApiReactionUser {
  id: string;
  username: string;
  profile_picture_url?: string | null;
}

export interface ApiReactionGroup {
  emoji: string;
  users: ApiReactionUser[];
}

export interface ApiHighlightReactionResponse {
  highlight_id: string;
  heart_count: number;
  viewer_reacted: boolean;
}

export interface ApiUserSummary {
  id: string;
  username: string;
  profile_picture_url?: string | null;
}

export interface SavedRecipe {
  id: string;
  user_id: string;
  post_id: string;
  category: string;
  created_at: string;
  deleted_at?: string | null;
  post?: ApiPost;
}

export interface SavedRecipeCategory {
  name: string;
  recipes: SavedRecipe[];
}

export interface RecipeCategory {
  id: string;
  user_id: string;
  name: string;
  position: number;
  created_at: string;
}

export interface PostSaveInfo {
  save_count: number;
  users: ApiReactionUser[];
  viewer_saved: boolean;
  viewer_categories?: string[];
}

export interface CookLog {
  id: string;
  user_id: string;
  post_id: string;
  rating: number;
  notes?: string | null;
  created_at: string;
  updated_at?: string | null;
  deleted_at?: string | null;
}

export interface CookLogUser {
  id: string;
  username: string;
  profile_picture_url?: string | null;
  rating: number;
  created_at: string;
}

export interface PostCookInfo {
  cook_count: number;
  avg_rating?: number | null;
  users: CookLogUser[];
  viewer_cooked: boolean;
  viewer_cook_log?: CookLog;
}

export interface CookLogWithPost extends CookLog {
  post?: ApiPost;
}

export interface WatchlistItem {
  id: string;
  userId: string;
  postId: string;
  category: string;
  createdAt: string;
}

export interface WatchlistCategory {
  id: string;
  name: string;
  position: number;
}

export interface WatchlistUser {
  id: string;
  username: string;
  displayName: string;
  avatar?: string;
}

export interface PostWatchlistInfo {
  saveCount: number;
  users: WatchlistUser[];
  viewerSaved: boolean;
  viewerCategories: string[];
}

export interface PodcastSave {
  id: string;
  userId: string;
  postId: string;
  createdAt: string;
  deletedAt?: string;
}

export interface PodcastSaveUser {
  id: string;
  username: string;
  displayName: string;
  avatar?: string;
}

export interface PostPodcastSaveInfo {
  saveCount: number;
  users: PodcastSaveUser[];
  viewerSaved: boolean;
}

export interface RecentPodcastItem {
  postId: string;
  linkId: string;
  url: string;
  podcast: PodcastMetadata;
  userId: string;
  username: string;
  postCreatedAt: string;
  linkCreatedAt: string;
}

export interface SectionRecentPodcastsResponse {
  items: RecentPodcastItem[];
  hasMore: boolean;
  nextCursor?: string;
}

export interface WatchlistItemWithPost extends WatchlistItem {
  post?: ApiPost;
}

export interface WatchLogUser {
  id: string;
  username: string;
  displayName: string;
  avatar?: string;
}

export interface WatchLog {
  id: string;
  userId: string;
  postId: string;
  rating: number;
  notes?: string;
  watchedAt: string;
  user?: WatchLogUser;
}

export interface PostWatchLogsResponse {
  watchCount: number;
  avgRating?: number;
  logs: WatchLog[];
  viewerWatched: boolean;
  viewerRating?: number;
}

export interface WatchLogWithPost extends WatchLog {
  post?: ApiPost;
}

export interface BookshelfCategory {
  id: string;
  name: string;
  position: number;
}

export interface BookshelfItem {
  id: string;
  userId: string;
  postId: string;
  categoryId?: string;
  createdAt: string;
  deletedAt?: string;
}

export interface BookshelfResponse {
  bookshelfItems: BookshelfItem[];
  nextCursor?: string;
}

export interface ReadLog {
  id: string;
  userId: string;
  postId: string;
  rating?: number;
  createdAt: string;
  deletedAt?: string;
}

export interface ReadLogReader {
  id: string;
  username: string;
  displayName: string;
  avatar?: string;
  rating?: number;
}

export interface PostReadLogsResponse {
  readCount: number;
  averageRating: number;
  viewerRead: boolean;
  viewerRating?: number;
  readers: ReadLogReader[];
}

export interface ReadHistoryResponse {
  readLogs: ReadLog[];
  nextCursor?: string;
}

export interface BookQuote {
  id: string;
  postId: string;
  userId: string;
  quoteText: string;
  pageNumber?: number;
  chapter?: string;
  note?: string;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string;
}

export interface BookQuoteWithUser extends BookQuote {
  username: string;
  displayName: string;
}

export interface CreateBookQuoteRequest {
  quoteText: string;
  pageNumber?: number;
  chapter?: string;
  note?: string;
}

export interface UpdateBookQuoteRequest {
  quoteText?: string;
  pageNumber?: number;
  chapter?: string;
  note?: string;
}

export interface BookQuoteResponse {
  quote: BookQuoteWithUser;
}

export interface BookQuotesListResponse {
  quotes: BookQuoteWithUser[];
  nextCursor?: string;
  hasMore: boolean;
}

class ApiClient {
  private tracer = trace.getTracer('clubhouse-frontend');
  private csrfToken: string | null = null;
  private csrfTokenPromise: Promise<string | null> | null = null;

  private isMutation(method: string): boolean {
    return ['POST', 'PUT', 'PATCH', 'DELETE'].includes(method);
  }

  private shouldAttachCsrf(method: string, endpoint: string): boolean {
    return this.isMutation(method) && !CSRF_EXEMPT_ENDPOINTS.has(endpoint);
  }

  private async fetchCsrfToken(): Promise<string | null> {
    try {
      const response = await fetch(`${API_BASE}${CSRF_ENDPOINT}`, {
        method: 'GET',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        logWarn('Failed to fetch CSRF token', { status: response.status });
        return null;
      }

      const data: { token?: string; csrf_token?: string; csrfToken?: string } | null =
        await response.json().catch(() => null);
      const token = data?.token ?? data?.csrf_token ?? data?.csrfToken ?? null;
      if (typeof token === 'string' && token.length > 0) {
        this.csrfToken = token;
        return token;
      }
    } catch (error) {
      logWarn('Failed to fetch CSRF token', { error });
      return null;
    }

    return null;
  }

  private async ensureCsrfToken(): Promise<string | null> {
    if (this.csrfToken) {
      return this.csrfToken;
    }

    if (this.csrfTokenPromise) {
      return this.csrfTokenPromise;
    }

    this.csrfTokenPromise = this.fetchCsrfToken().finally(() => {
      this.csrfTokenPromise = null;
    });
    return this.csrfTokenPromise;
  }

  private async refreshCsrfToken(): Promise<string | null> {
    this.csrfToken = null;
    this.csrfTokenPromise = null;
    return this.ensureCsrfToken();
  }

  clearCsrfToken(): void {
    this.csrfToken = null;
    this.csrfTokenPromise = null;
  }

  async prefetchCsrfToken(): Promise<void> {
    await this.ensureCsrfToken();
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {},
    retry = true,
    logOptions?: LogOptions
  ): Promise<T> {
    const url = `${API_BASE}${endpoint}`;
    const method = (options.method ?? 'GET').toUpperCase();
    const startTime = typeof performance !== 'undefined' ? performance.now() : null;
    const headers = new Headers(options.headers ?? {});
    headers.set('Content-Type', 'application/json');

    const span = this.tracer.startSpan(`api ${method} ${endpoint}`, {
      attributes: {
        'http.method': method,
        'http.url': url,
        'http.target': endpoint,
      },
    });
    let spanEnded = false;
    const endSpan = () => {
      if (spanEnded) return;
      spanEnded = true;
      span.end();
    };

    return context.with(trace.setSpan(context.active(), span), async () => {
      if (this.shouldAttachCsrf(method, endpoint)) {
        const csrfToken = await this.ensureCsrfToken();
        if (csrfToken) {
          headers.set(CSRF_HEADER, csrfToken);
        }
      }

      propagation.inject(context.active(), headers, {
        set: (carrier: Headers, key: string, value: string) => {
          carrier.set(key, value);
        },
      });

      let response: Response;
      try {
        response = await fetch(url, {
          ...options,
          method,
          headers,
          credentials: 'include',
        });
      } catch (error) {
        if (startTime !== null) {
          recordApiTiming(endpoint, method, 0, performance.now() - startTime);
        }
        span.recordException(error as Error);
        span.setStatus({ code: SpanStatusCode.ERROR });
        logError('API request failed', { endpoint, method, url }, error);
        endSpan();
        throw error;
      }

      if (startTime !== null) {
        recordApiTiming(endpoint, method, response.status, performance.now() - startTime);
      }

      span.setAttribute('http.status_code', response.status);

      if (!response.ok) {
        span.setStatus({ code: SpanStatusCode.ERROR });
        const errorData: ApiError | null = await response.json().catch(() => null);
        if (
          retry &&
          this.shouldAttachCsrf(method, endpoint) &&
          (response.status === 403 ||
            response.status === 419 ||
            (errorData?.code ? CSRF_ERROR_CODES.has(errorData.code) : false))
        ) {
          endSpan();
          await this.refreshCsrfToken();
          return this.request<T>(endpoint, options, false, logOptions);
        }

        const error = toApiClientError(errorData, 'An unexpected error occurred');
        const logContext = {
          endpoint,
          method,
          url,
          status: response.status,
          code: error.code,
        };
        const shouldLog = !logOptions?.suppressStatuses?.includes(response.status);
        if (shouldLog) {
          if (response.status >= 500) {
            logError('API request failed', logContext, error);
          } else {
            logWarn('API request failed', logContext);
          }
        }
        endSpan();
        throw error;
      }

      if (response.status === 204) {
        endSpan();
        return {} as T;
      }

      try {
        return await response.json();
      } finally {
        endSpan();
      }
    });
  }

  async get<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    return this.request<T>(endpoint, { method: 'GET', ...options });
  }

  async post<T>(endpoint: string, data?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'POST',
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  async put<T>(endpoint: string, data?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'PUT',
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  async patch<T>(endpoint: string, data?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'PATCH',
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  async delete<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'DELETE' });
  }

  async searchUsers(query: string, limit = 8): Promise<{ users: ApiUserSummary[] }> {
    const params = new URLSearchParams();
    if (query) {
      params.set('q', query);
    } else {
      params.set('q', '');
    }
    params.set('limit', String(limit));
    return this.get(`/users/autocomplete?${params.toString()}`);
  }

  async lookupUserByUsername(
    username: string,
    options: { suppressNotFound?: boolean } = {}
  ): Promise<{ user: ApiUserSummary | null }> {
    const params = new URLSearchParams({ username });
    const logOptions = options.suppressNotFound ? { suppressStatuses: [404] } : undefined;
    return this.request(`/users/lookup?${params.toString()}`, { method: 'GET' }, true, logOptions);
  }

  private async uploadWithRetry(
    file: File,
    onProgress?: (progress: number) => void,
    retry = true
  ): Promise<{ url: string }> {
    const csrfToken = await this.ensureCsrfToken();
    const startTime = typeof performance !== 'undefined' ? performance.now() : null;
    const span = this.tracer.startSpan('api POST /uploads', {
      attributes: {
        'http.method': 'POST',
        'http.url': `${API_BASE}/uploads`,
        'http.target': '/uploads',
      },
    });

    return context.with(trace.setSpan(context.active(), span), () => {
      return new Promise<{ url: string }>((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.open('POST', `${API_BASE}/uploads`);
        xhr.withCredentials = true;
        if (csrfToken) {
          xhr.setRequestHeader(CSRF_HEADER, csrfToken);
        }

        const traceHeaders: Record<string, string> = {};
        propagation.inject(context.active(), traceHeaders);
        Object.entries(traceHeaders).forEach(([key, value]) => {
          xhr.setRequestHeader(key, value);
        });

        xhr.upload.onprogress = (event) => {
          if (!onProgress || !event.lengthComputable) return;
          const percent = Math.round((event.loaded / event.total) * 100);
          onProgress(Math.min(100, Math.max(0, percent)));
        };

        xhr.onerror = () => {
          if (startTime !== null) {
            recordApiTiming('/uploads', 'POST', 0, performance.now() - startTime);
          }
          span.setStatus({ code: SpanStatusCode.ERROR });
          logError('Upload request failed', { endpoint: '/uploads', method: 'POST' });
          span.end();
          reject(new Error('Upload failed'));
        };

        xhr.onload = async () => {
          const status = xhr.status;
          if (startTime !== null) {
            recordApiTiming('/uploads', 'POST', status, performance.now() - startTime);
          }
          const responseText = xhr.responseText || '{}';
          span.setAttribute('http.status_code', status);
          if (status >= 200 && status < 300) {
            try {
              const data = JSON.parse(responseText) as { url: string };
              span.end();
              resolve(data);
            } catch {
              span.setStatus({ code: SpanStatusCode.ERROR });
              span.end();
              reject(new Error('Upload failed'));
            }
            return;
          }

          let errorData: ApiError | null = null;
          try {
            errorData = JSON.parse(responseText) as ApiError;
          } catch {
            errorData = null;
          }

          if (
            retry &&
            (status === 403 ||
              status === 419 ||
              (errorData?.code ? CSRF_ERROR_CODES.has(errorData.code) : false))
          ) {
            span.end();
            await this.refreshCsrfToken();
            this.uploadWithRetry(file, onProgress, false).then(resolve).catch(reject);
            return;
          }

          span.setStatus({ code: SpanStatusCode.ERROR });
          const error = toApiClientError(errorData, 'Upload failed');
          const logContext = {
            endpoint: '/uploads',
            method: 'POST',
            status,
            code: error.code,
          };
          if (status >= 500) {
            logError('Upload request failed', logContext, error);
          } else {
            logWarn('Upload request failed', logContext);
          }
          span.end();
          reject(error);
        };

        const formData = new FormData();
        formData.append('file', file);
        xhr.send(formData);
      });
    });
  }

  async uploadImage(file: File, onProgress?: (progress: number) => void): Promise<{ url: string }> {
    return this.uploadWithRetry(file, onProgress);
  }

  async createPost(data: CreatePostRequest): Promise<{ post: Post }> {
    const mappedLinks = data.links?.map(mapPostLinkRequest);
    const response = await this.post<{ post: ApiPost }>('/posts', {
      section_id: data.sectionId,
      content: data.content,
      links: mappedLinks,
      images: data.images?.map((image) => ({
        url: image.url,
        caption: image.caption,
        alt_text: image.altText,
      })),
      mention_usernames: data.mentionUsernames ?? [],
    });
    return { post: mapApiPost(response.post) };
  }

  async getPost(postId: string): Promise<{ post: Post | null }> {
    const response = await this.get<{ post: ApiPost | null }>(`/posts/${postId}`);
    return { post: response.post ? mapApiPost(response.post) : null };
  }

  async getFeed(
    sectionId: string,
    limit = 20,
    cursor?: string
  ): Promise<ApiResponse<{ posts: Post[] }>> {
    const params = new URLSearchParams({ limit: String(limit) });
    if (cursor) params.set('cursor', cursor);
    return this.get(`/sections/${sectionId}/feed?${params}`);
  }

  async getMoviePosts(
    limit = 20,
    cursor?: string,
    sectionType?: 'movie' | 'series'
  ): Promise<{ posts: Post[]; hasMore: boolean; nextCursor?: string }> {
    const params = new URLSearchParams({ limit: String(limit) });
    if (cursor) params.set('cursor', cursor);
    if (sectionType) params.set('section_type', sectionType);
    const response = await this.get<{
      posts: ApiPost[];
      has_more?: boolean;
      next_cursor?: string | null;
    }>(`/posts/movies?${params}`);
    return {
      posts: (response.posts ?? []).map(mapApiPost),
      hasMore: response.has_more ?? false,
      nextCursor: response.next_cursor ?? undefined,
    };
  }

  async getSectionLinks(
    sectionId: string,
    limit = 20,
    cursor?: string
  ): Promise<SectionLinksResponse> {
    const params = new URLSearchParams({ limit: String(limit) });
    if (cursor) params.set('cursor', cursor);
    const response = await this.get<ApiSectionLinksResponse>(
      `/sections/${sectionId}/links?${params}`
    );
    return {
      links: (response.links ?? []).map(mapApiSectionLink),
      hasMore: response.has_more ?? false,
      nextCursor: response.next_cursor ?? null,
    };
  }

  async deletePost(postId: string): Promise<void> {
    return this.delete(`/posts/${postId}`);
  }

  async updatePost(
    postId: string,
    data: {
      content: string;
      links?: PostLinkRequest[] | null;
      removeLinkMetadata?: boolean;
      mentionUsernames?: string[];
    }
  ): Promise<{ post: Post }> {
    const mappedLinks = data.links?.map(mapPostLinkRequest);
    const response = await this.patch<{ post: ApiPost }>(`/posts/${postId}`, {
      content: data.content,
      links: mappedLinks ?? undefined,
      remove_link_metadata: data.removeLinkMetadata ?? undefined,
      mention_usernames: data.mentionUsernames ?? [],
    });
    return { post: mapApiPost(response.post) };
  }

  async previewLink(url: string): Promise<{ metadata: LinkMetadata }> {
    return this.post('/links/preview', { url });
  }

  async parseRecipe(url: string): Promise<{ metadata: LinkMetadata }> {
    return this.post('/links/parse-recipe', { url });
  }

  async createComment(data: CreateCommentRequest): Promise<{ comment: ApiComment }> {
    return this.post('/comments', {
      post_id: data.postId,
      parent_comment_id: data.parentCommentId,
      image_id: data.imageId,
      content: data.content,
      contains_spoiler: data.containsSpoiler,
      timestamp_seconds: data.timestampSeconds,
      links: data.links,
      mention_usernames: data.mentionUsernames ?? [],
    });
  }

  async getThreadComments(
    postId: string,
    limit = 50,
    cursor?: string
  ): Promise<{ comments: ApiComment[]; meta?: { cursor?: string | null; has_more?: boolean } }> {
    const params = new URLSearchParams({ limit: String(limit) });
    if (cursor) params.set('cursor', cursor);
    return this.get(`/posts/${postId}/comments?${params}`);
  }

  async getComment(commentId: string): Promise<{ comment: ApiComment }> {
    return this.get(`/comments/${commentId}`);
  }

  async updateComment(
    commentId: string,
    data: {
      content: string;
      containsSpoiler?: boolean;
      links?: { url: string }[] | null;
      mentionUsernames?: string[];
    }
  ): Promise<{ comment: Comment }> {
    const response = await this.patch<{ comment: ApiComment }>(`/comments/${commentId}`, {
      content: data.content,
      contains_spoiler: data.containsSpoiler,
      links: data.links ?? undefined,
      mention_usernames: data.mentionUsernames ?? [],
    });
    return { comment: mapApiComment(response.comment) };
  }

  async deleteComment(commentId: string): Promise<void> {
    return this.delete(`/comments/${commentId}`);
  }

  async addPostReaction(postId: string, emoji: string): Promise<void> {
    await this.post(`/posts/${postId}/reactions`, { emoji });
  }

  async getPostReactions(postId: string): Promise<{ reactions: ApiReactionGroup[] }> {
    return this.get(`/posts/${postId}/reactions`);
  }

  async removePostReaction(postId: string, emoji: string): Promise<void> {
    await this.delete(`/posts/${postId}/reactions/${encodeURIComponent(emoji)}`);
  }

  async addCommentReaction(commentId: string, emoji: string): Promise<void> {
    await this.post(`/comments/${commentId}/reactions`, { emoji });
  }

  async getCommentReactions(commentId: string): Promise<{ reactions: ApiReactionGroup[] }> {
    return this.get(`/comments/${commentId}/reactions`);
  }

  async removeCommentReaction(commentId: string, emoji: string): Promise<void> {
    await this.delete(`/comments/${commentId}/reactions/${encodeURIComponent(emoji)}`);
  }

  async addHighlightReaction(
    postId: string,
    highlightId: string
  ): Promise<ApiHighlightReactionResponse> {
    return this.post(
      `/posts/${postId}/highlights/${encodeURIComponent(highlightId)}/reactions`,
      {}
    );
  }

  async removeHighlightReaction(
    postId: string,
    highlightId: string
  ): Promise<ApiHighlightReactionResponse> {
    return this.delete(`/posts/${postId}/highlights/${encodeURIComponent(highlightId)}/reactions`);
  }

  async getHighlightReactions(postId: string, highlightId: string): Promise<{ reactions: ApiReactionGroup[] }> {
    return this.get(`/posts/${postId}/highlights/${encodeURIComponent(highlightId)}/reactions`);
  }

  async saveRecipe(postId: string, categories: string[]): Promise<{ saved_recipes: SavedRecipe[] }> {
    return this.post(`/posts/${postId}/save`, { categories });
  }

  async unsaveRecipe(postId: string, category?: string): Promise<void> {
    const params = new URLSearchParams();
    if (category) {
      params.set('category', category);
    }
    const query = params.toString();
    const endpoint = query ? `/posts/${postId}/save?${query}` : `/posts/${postId}/save`;
    return this.delete(endpoint);
  }

  async getPostSaves(postId: string): Promise<PostSaveInfo> {
    return this.get(`/posts/${postId}/saves`);
  }

  async getMySavedRecipes(): Promise<{ categories: SavedRecipeCategory[] }> {
    return this.get('/me/saved-recipes');
  }

  async getMyRecipeCategories(): Promise<{ categories: RecipeCategory[] }> {
    return this.get('/me/recipe-categories');
  }

  async createRecipeCategory(name: string): Promise<{ category: RecipeCategory }> {
    return this.post('/me/recipe-categories', { name });
  }

  async updateRecipeCategory(
    id: string,
    data: { name?: string; position?: number }
  ): Promise<{ category: RecipeCategory }> {
    return this.patch(`/me/recipe-categories/${id}`, {
      name: data.name,
      position: data.position,
    });
  }

  async deleteRecipeCategory(id: string): Promise<void> {
    return this.delete(`/me/recipe-categories/${id}`);
  }

  async logCook(
    postId: string,
    rating: number,
    notes?: string
  ): Promise<{ cook_log: CookLog }> {
    return this.post(`/posts/${postId}/cook-log`, { rating, notes });
  }

  async updateCookLog(
    postId: string,
    rating: number,
    notes?: string
  ): Promise<{ cook_log: CookLog }> {
    return this.put(`/posts/${postId}/cook-log`, { rating, notes });
  }

  async removeCookLog(postId: string): Promise<void> {
    return this.delete(`/posts/${postId}/cook-log`);
  }

  async getPostCookLogs(postId: string): Promise<PostCookInfo> {
    return this.get(`/posts/${postId}/cook-logs`);
  }

  async getMyCookLogs(
    limit?: number,
    cursor?: string
  ): Promise<{ cook_logs: CookLogWithPost[]; has_more: boolean; cursor?: string }> {
    const params = new URLSearchParams();
    if (limit !== undefined) {
      params.set('limit', String(limit));
    }
    if (cursor) {
      params.set('cursor', cursor);
    }
    const query = params.toString();
    const response = await this.get<{
      cook_logs: CookLogWithPost[];
      meta?: { has_more?: boolean; cursor?: string | null };
    }>(`/me/cook-logs${query ? `?${query}` : ''}`);
    return {
      cook_logs: response.cook_logs ?? [],
      has_more: response.meta?.has_more ?? false,
      cursor: response.meta?.cursor ?? undefined,
    };
  }

  async savePodcast(postId: string): Promise<PodcastSave> {
    const response = await this.post<ApiPodcastSave>(`/posts/${postId}/podcast-save`);
    return mapApiPodcastSave(response);
  }

  async unsavePodcast(postId: string): Promise<void> {
    return this.delete(`/posts/${postId}/podcast-save`);
  }

  async getPostPodcastSaveInfo(postId: string): Promise<PostPodcastSaveInfo> {
    const response = await this.get<ApiPostPodcastSaveInfo>(`/posts/${postId}/podcast-save-info`);
    return {
      saveCount: response.save_count ?? 0,
      users: (response.users ?? []).map(mapApiPodcastSaveUser),
      viewerSaved: response.viewer_saved ?? false,
    };
  }

  async getSectionSavedPodcasts(
    sectionId: string,
    limit = 20,
    cursor?: string
  ): Promise<{ posts: Post[]; hasMore: boolean; nextCursor?: string }> {
    const params = new URLSearchParams({ limit: String(limit) });
    if (cursor) {
      params.set('cursor', cursor);
    }
    const response = await this.get<{
      posts: ApiPost[];
      has_more?: boolean;
      next_cursor?: string | null;
    }>(`/sections/${sectionId}/podcast-saved?${params.toString()}`);
    return {
      posts: (response.posts ?? []).map(mapApiPost),
      hasMore: response.has_more ?? false,
      nextCursor: response.next_cursor ?? undefined,
    };
  }

  async getSectionRecentPodcasts(
    sectionId: string,
    limit = 20,
    cursor?: string
  ): Promise<SectionRecentPodcastsResponse> {
    const params = new URLSearchParams({ limit: String(limit) });
    if (cursor) {
      params.set('cursor', cursor);
    }
    const response = await this.get<ApiSectionRecentPodcastsResponse>(
      `/sections/${sectionId}/podcasts/recent?${params.toString()}`
    );
    return {
      items: (response.items ?? []).map(mapApiRecentPodcastItem),
      hasMore: response.has_more ?? false,
      nextCursor: response.next_cursor ?? undefined,
    };
  }

  async addToWatchlist(
    postId: string,
    categories: string[]
  ): Promise<{ watchlistItems: WatchlistItem[] }> {
    const response = await this.post<{ watchlist_items: ApiWatchlistItem[] }>(
      `/posts/${postId}/watchlist`,
      { categories }
    );
    return { watchlistItems: (response.watchlist_items ?? []).map(mapApiWatchlistItem) };
  }

  async removeFromWatchlist(postId: string, category?: string): Promise<void> {
    const params = new URLSearchParams();
    if (category) {
      params.set('category', category);
    }
    const query = params.toString();
    const endpoint = query ? `/posts/${postId}/watchlist?${query}` : `/posts/${postId}/watchlist`;
    return this.delete(endpoint);
  }

  async getPostWatchlistInfo(postId: string): Promise<PostWatchlistInfo> {
    const response = await this.get<ApiPostWatchlistInfo>(`/posts/${postId}/watchlist-info`);
    return {
      saveCount: response.save_count ?? 0,
      users: (response.users ?? []).map(mapApiWatchlistUser),
      viewerSaved: response.viewer_saved ?? false,
      viewerCategories: response.viewer_categories ?? [],
    };
  }

  async getMyWatchlist(
    sectionType?: 'movie' | 'series'
  ): Promise<{ categories: { name: string; items: WatchlistItemWithPost[] }[] }> {
    const params = new URLSearchParams();
    if (sectionType) {
      params.set('section_type', sectionType);
    }
    const query = params.toString();
    const response = await this.get<{
      categories: { name: string; items: ApiWatchlistItem[] }[];
    }>(query ? `/me/watchlist?${query}` : '/me/watchlist');
    return {
      categories: (response.categories ?? []).map((category) => ({
        name: category.name,
        items: (category.items ?? []).map(mapApiWatchlistItemWithPost),
      })),
    };
  }

  async getWatchlistCategories(): Promise<{ categories: WatchlistCategory[] }> {
    const response = await this.get<{ categories: ApiWatchlistCategory[] }>('/me/watchlist-categories');
    return { categories: (response.categories ?? []).map(mapApiWatchlistCategory) };
  }

  async createWatchlistCategory(name: string): Promise<{ category: WatchlistCategory }> {
    const response = await this.post<{ category: ApiWatchlistCategory }>('/me/watchlist-categories', {
      name,
    });
    return { category: mapApiWatchlistCategory(response.category) };
  }

  async updateWatchlistCategory(
    id: string,
    data: { name?: string; position?: number }
  ): Promise<{ category: WatchlistCategory }> {
    const response = await this.patch<{ category: ApiWatchlistCategory }>(
      `/me/watchlist-categories/${id}`,
      {
        name: data.name,
        position: data.position,
      }
    );
    return { category: mapApiWatchlistCategory(response.category) };
  }

  async deleteWatchlistCategory(id: string): Promise<void> {
    return this.delete(`/me/watchlist-categories/${id}`);
  }

  async logWatch(
    postId: string,
    rating: number,
    notes?: string,
    watchedAt?: string
  ): Promise<{ watchLog: WatchLog }> {
    const response = await this.post<{ watch_log: ApiWatchLog }>(`/posts/${postId}/watch-log`, {
      rating,
      notes,
      watched_at: watchedAt ?? new Date().toISOString(),
    });
    return { watchLog: mapApiWatchLog(response.watch_log) };
  }

  async updateWatchLog(
    postId: string,
    data: { rating?: number; notes?: string }
  ): Promise<{ watchLog: WatchLog }> {
    const response = await this.put<{ watch_log: ApiWatchLog }>(`/posts/${postId}/watch-log`, {
      rating: data.rating,
      notes: data.notes,
    });
    return { watchLog: mapApiWatchLog(response.watch_log) };
  }

  async removeWatchLog(postId: string): Promise<void> {
    return this.delete(`/posts/${postId}/watch-log`);
  }

  async getPostWatchLogs(postId: string): Promise<PostWatchLogsResponse> {
    const response = await this.get<ApiPostWatchLogsResponse>(`/posts/${postId}/watch-logs`);
    return {
      watchCount: response.watch_count ?? 0,
      avgRating: response.avg_rating ?? undefined,
      logs: (response.logs ?? []).map((log) => mapApiWatchLog(log.watch_log, log.user)),
      viewerWatched: response.viewer_watched ?? false,
      viewerRating: response.viewer_rating ?? undefined,
    };
  }

  async getMyWatchLogs(
    limit?: number,
    cursor?: string
  ): Promise<{ watchLogs: WatchLogWithPost[]; nextCursor?: string }> {
    const params = new URLSearchParams();
    if (limit !== undefined) {
      params.set('limit', String(limit));
    }
    if (cursor) {
      params.set('cursor', cursor);
    }
    const query = params.toString();
    const response = await this.get<{
      watch_logs: ApiWatchLog[];
      next_cursor?: string | null;
    }>(`/me/watch-logs${query ? `?${query}` : ''}`);
    return {
      watchLogs: (response.watch_logs ?? []).map(mapApiWatchLogWithPost),
      nextCursor: response.next_cursor ?? undefined,
    };
  }

  async createBookQuote(postId: string, req: CreateBookQuoteRequest): Promise<BookQuoteResponse> {
    const response = await this.post<{ quote: ApiBookQuoteWithUser }>(`/posts/${postId}/quotes`, {
      quote_text: req.quoteText,
      page_number: req.pageNumber,
      chapter: req.chapter,
      note: req.note,
    });
    return { quote: mapApiBookQuoteWithUser(response.quote) };
  }

  async updateBookQuote(quoteId: string, req: UpdateBookQuoteRequest): Promise<BookQuoteResponse> {
    const response = await this.put<{ quote: ApiBookQuoteWithUser }>(`/quotes/${quoteId}`, {
      quote_text: req.quoteText,
      page_number: req.pageNumber,
      chapter: req.chapter,
      note: req.note,
    });
    return { quote: mapApiBookQuoteWithUser(response.quote) };
  }

  async deleteBookQuote(quoteId: string): Promise<void> {
    return this.delete(`/quotes/${quoteId}`);
  }

  async getPostQuotes(postId: string, cursor?: string, limit?: number): Promise<BookQuotesListResponse> {
    const params = new URLSearchParams();
    if (cursor) {
      params.set('cursor', cursor);
    }
    if (limit !== undefined) {
      params.set('limit', String(limit));
    }
    const query = params.toString();
    const response = await this.get<{
      quotes: ApiBookQuoteWithUser[];
      next_cursor?: string | null;
      has_more?: boolean;
    }>(`/posts/${postId}/quotes${query ? `?${query}` : ''}`);
    return {
      quotes: (response.quotes ?? []).map(mapApiBookQuoteWithUser),
      nextCursor: response.next_cursor ?? undefined,
      hasMore: response.has_more ?? false,
    };
  }

  async getUserQuotes(userId: string, cursor?: string, limit?: number): Promise<BookQuotesListResponse> {
    const params = new URLSearchParams();
    if (cursor) {
      params.set('cursor', cursor);
    }
    if (limit !== undefined) {
      params.set('limit', String(limit));
    }
    const query = params.toString();
    const response = await this.get<{
      quotes: ApiBookQuoteWithUser[];
      next_cursor?: string | null;
      has_more?: boolean;
    }>(`/users/${userId}/quotes${query ? `?${query}` : ''}`);
    return {
      quotes: (response.quotes ?? []).map(mapApiBookQuoteWithUser),
      nextCursor: response.next_cursor ?? undefined,
      hasMore: response.has_more ?? false,
    };
  }

  async createBookshelfCategory(name: string): Promise<{ category: BookshelfCategory }> {
    const response = await this.post<{ category: ApiBookshelfCategory }>('/bookshelf/categories', {
      name,
    });
    return { category: mapApiBookshelfCategory(response.category) };
  }

  async getBookshelfCategories(): Promise<{ categories: BookshelfCategory[] }> {
    const response = await this.get<{ categories: ApiBookshelfCategory[] }>('/bookshelf/categories');
    return { categories: (response.categories ?? []).map(mapApiBookshelfCategory) };
  }

  async updateBookshelfCategory(
    id: string,
    name: string,
    position: number
  ): Promise<{ category: BookshelfCategory }> {
    const response = await this.put<{ category: ApiBookshelfCategory }>(`/bookshelf/categories/${id}`, {
      name,
      position,
    });
    return { category: mapApiBookshelfCategory(response.category) };
  }

  async deleteBookshelfCategory(id: string): Promise<void> {
    return this.delete(`/bookshelf/categories/${id}`);
  }

  async reorderBookshelfCategories(categoryIds: string[]): Promise<void> {
    await this.post('/bookshelf/categories/reorder', { category_ids: categoryIds });
  }

  async addToBookshelf(postId: string, categories: string[]): Promise<void> {
    await this.post(`/posts/${postId}/bookshelf`, { categories });
  }

  async removeFromBookshelf(postId: string): Promise<void> {
    return this.delete(`/posts/${postId}/bookshelf`);
  }

  async getMyBookshelf(category?: string, cursor?: string, limit?: number): Promise<BookshelfResponse> {
    const params = new URLSearchParams();
    if (category) {
      params.set('category', category);
    }
    if (cursor) {
      params.set('cursor', cursor);
    }
    if (limit !== undefined) {
      params.set('limit', String(limit));
    }
    const query = params.toString();
    const response = await this.get<ApiBookshelfResponse>(`/bookshelf${query ? `?${query}` : ''}`);
    return {
      bookshelfItems: (response.bookshelf_items ?? []).map(mapApiBookshelfItem),
      nextCursor: response.next_cursor ?? undefined,
    };
  }

  async getAllBookshelfItems(category?: string, cursor?: string, limit?: number): Promise<BookshelfResponse> {
    const params = new URLSearchParams();
    if (category) {
      params.set('category', category);
    }
    if (cursor) {
      params.set('cursor', cursor);
    }
    if (limit !== undefined) {
      params.set('limit', String(limit));
    }
    const query = params.toString();
    const response = await this.get<ApiBookshelfResponse>(`/bookshelf/all${query ? `?${query}` : ''}`);
    return {
      bookshelfItems: (response.bookshelf_items ?? []).map(mapApiBookshelfItem),
      nextCursor: response.next_cursor ?? undefined,
    };
  }

  async logRead(postId: string, rating?: number): Promise<{ readLog: ReadLog }> {
    const response = await this.post<{ read_log: ApiReadLog }>(`/posts/${postId}/read`, {
      rating,
    });
    return { readLog: mapApiReadLog(response.read_log) };
  }

  async removeReadLog(postId: string): Promise<void> {
    return this.delete(`/posts/${postId}/read`);
  }

  async updateReadRating(postId: string, rating: number): Promise<{ readLog: ReadLog }> {
    const response = await this.put<{ read_log: ApiReadLog }>(`/posts/${postId}/read`, {
      rating,
    });
    return { readLog: mapApiReadLog(response.read_log) };
  }

  async getPostReadLogs(postId: string): Promise<PostReadLogsResponse> {
    const response = await this.get<ApiPostReadLogsResponse>(`/posts/${postId}/read`);
    return {
      readCount: response.read_count ?? 0,
      averageRating: response.average_rating ?? 0,
      viewerRead: response.viewer_read ?? false,
      viewerRating: response.viewer_rating ?? undefined,
      readers: (response.readers ?? []).map(mapApiReadLogReader),
    };
  }

  async getReadHistory(cursor?: string, limit?: number): Promise<ReadHistoryResponse> {
    const params = new URLSearchParams();
    if (cursor) {
      params.set('cursor', cursor);
    }
    if (limit !== undefined) {
      params.set('limit', String(limit));
    }
    const query = params.toString();
    const response = await this.get<{ read_logs: ApiReadLog[]; next_cursor?: string | null }>(
      `/read-history${query ? `?${query}` : ''}`
    );
    return {
      readLogs: (response.read_logs ?? []).map(mapApiReadLog),
      nextCursor: response.next_cursor ?? undefined,
    };
  }
}

export const api = new ApiClient();
