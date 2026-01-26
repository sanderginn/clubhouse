import type { Post, CreatePostRequest, LinkMetadata } from '../stores/postStore';
import type { CreateCommentRequest } from '../stores/commentStore';
import type { ApiComment } from '../stores/commentMapper';
import { mapApiPost, type ApiPost } from '../stores/postMapper';

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
}

interface ApiResponse<T> {
  data: T;
  meta?: {
    cursor?: string;
    hasMore?: boolean;
  };
}

class ApiClient {
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
        return null;
      }

      const data: { token?: string; csrf_token?: string; csrfToken?: string } | null =
        await response.json().catch(() => null);
      const token = data?.token ?? data?.csrf_token ?? data?.csrfToken ?? null;
      if (typeof token === 'string' && token.length > 0) {
        this.csrfToken = token;
        return token;
      }
    } catch {
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
    retry = true
  ): Promise<T> {
    const url = `${API_BASE}${endpoint}`;
    const method = (options.method ?? 'GET').toUpperCase();
    const headers = new Headers(options.headers ?? {});
    headers.set('Content-Type', 'application/json');

    if (this.shouldAttachCsrf(method, endpoint)) {
      const csrfToken = await this.ensureCsrfToken();
      if (csrfToken) {
        headers.set(CSRF_HEADER, csrfToken);
      }
    }

    const response = await fetch(url, {
      ...options,
      method,
      headers,
      credentials: 'include',
    });

    if (!response.ok) {
      const errorData: ApiError = await response.json().catch(() => ({
        error: 'An unexpected error occurred',
        code: 'UNKNOWN_ERROR',
      }));

      if (
        retry &&
        this.shouldAttachCsrf(method, endpoint) &&
        (response.status === 403 || response.status === 419 || CSRF_ERROR_CODES.has(errorData.code))
      ) {
        await this.refreshCsrfToken();
        return this.request<T>(endpoint, options, false);
      }

      const error = new Error(errorData.error) as Error & { code?: string };
      error.code = errorData.code;
      throw error;
    }

    if (response.status === 204) {
      return {} as T;
    }

    return response.json();
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

  async createPost(data: CreatePostRequest): Promise<{ post: Post }> {
    const response = await this.post<{ post: ApiPost }>('/posts', {
      section_id: data.sectionId,
      content: data.content,
      links: data.links,
    });
    return { post: mapApiPost(response.post) };
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

  async deletePost(postId: string): Promise<void> {
    return this.delete(`/posts/${postId}`);
  }

  async previewLink(url: string): Promise<{ metadata: LinkMetadata }> {
    return this.post('/links/preview', { url });
  }

  async createComment(data: CreateCommentRequest): Promise<{ comment: ApiComment }> {
    return this.post('/comments', {
      post_id: data.postId,
      parent_comment_id: data.parentCommentId,
      content: data.content,
      links: data.links,
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

  async addPostReaction(postId: string, emoji: string): Promise<void> {
    await this.post(`/posts/${postId}/reactions`, { emoji });
  }

  async removePostReaction(postId: string, emoji: string): Promise<void> {
    await this.delete(`/posts/${postId}/reactions/${encodeURIComponent(emoji)}`);
  }

  async addCommentReaction(commentId: string, emoji: string): Promise<void> {
    await this.post(`/comments/${commentId}/reactions`, { emoji });
  }

  async removeCommentReaction(commentId: string, emoji: string): Promise<void> {
    await this.delete(`/comments/${commentId}/reactions/${encodeURIComponent(emoji)}`);
  }
}

export const api = new ApiClient();
