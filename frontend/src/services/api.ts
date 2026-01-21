import type { Post, CreatePostRequest, LinkMetadata } from '../stores/postStore';
import type { CreateCommentRequest } from '../stores/commentStore';
import type { ApiComment } from '../stores/commentMapper';
import { mapApiPost, type ApiPost } from '../stores/postMapper';

const API_BASE = '/api/v1';

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
  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const url = `${API_BASE}${endpoint}`;
    const response = await fetch(url, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
      credentials: 'include',
    });

    if (!response.ok) {
      const errorData: ApiError = await response.json().catch(() => ({
        error: 'An unexpected error occurred',
        code: 'UNKNOWN_ERROR',
      }));
      throw new Error(errorData.error);
    }

    if (response.status === 204) {
      return {} as T;
    }

    return response.json();
  }

  async get<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'GET' });
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
}

export const api = new ApiClient();
