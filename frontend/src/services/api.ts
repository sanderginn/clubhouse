import type { Post, CreatePostRequest, LinkMetadata } from '../stores/postStore';
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

interface ApiError {
  error: string;
  code: string;
  mfa_required?: boolean;
  mfaRequired?: boolean;
}

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

        const error = new Error(errorData?.error ?? 'An unexpected error occurred') as Error & {
          code?: string;
          mfaRequired?: boolean;
        };
        error.code = errorData?.code ?? 'UNKNOWN_ERROR';
        error.mfaRequired =
          errorData?.mfa_required ?? errorData?.mfaRequired ?? false;
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
          const error = new Error(errorData?.error ?? 'Upload failed') as Error & { code?: string };
          error.code = errorData?.code ?? 'UNKNOWN_ERROR';
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
    const response = await this.post<{ post: ApiPost }>('/posts', {
      section_id: data.sectionId,
      content: data.content,
      links: data.links,
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
      links?: { url: string; highlights?: { timestamp: number; label?: string }[] }[] | null;
      removeLinkMetadata?: boolean;
      mentionUsernames?: string[];
    }
  ): Promise<{ post: Post }> {
    const response = await this.patch<{ post: ApiPost }>(`/posts/${postId}`, {
      content: data.content,
      links: data.links ?? undefined,
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
    data: { content: string; links?: { url: string }[] | null; mentionUsernames?: string[] }
  ): Promise<{ comment: Comment }> {
    const response = await this.patch<{ comment: ApiComment }>(`/comments/${commentId}`, {
      content: data.content,
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
}

export const api = new ApiClient();
