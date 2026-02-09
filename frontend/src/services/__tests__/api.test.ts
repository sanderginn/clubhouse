import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api } from '../api';

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  api.clearCsrfToken();
});

describe('api client', () => {
  const findCall = (path: string) =>
    fetchMock.mock.calls.find(([url]) => (url as string).includes(path));

  it('returns parsed JSON on success', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue({ data: 'ok' }),
    });

    const response = await api.get('/ping');
    expect(response).toEqual({ data: 'ok' });
  });

  it('returns empty object on 204', async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: vi.fn().mockResolvedValue({ token: 'csrf-token' }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 204,
        json: vi.fn(),
      });

    const response = await api.delete('/noop');
    expect(response).toEqual({});
  });

  it('throws error with JSON error response', async () => {
    fetchMock.mockResolvedValue({
      ok: false,
      status: 400,
      json: vi.fn().mockResolvedValue({ error: 'Bad', code: 'BAD' }),
    });

    let caught: unknown = null;
    try {
      await api.get('/bad');
    } catch (error) {
      caught = error;
    }

    expect(caught).toBeInstanceOf(Error);
    expect((caught as Error).message).toBe('Bad');
    expect((caught as Error & { code?: string }).code).toBe('BAD');
  });

  it('throws default error when JSON parse fails', async () => {
    fetchMock.mockResolvedValue({
      ok: false,
      status: 500,
      json: vi.fn().mockRejectedValue(new Error('no json')),
    });

    await expect(api.get('/bad')).rejects.toThrow('An unexpected error occurred');
  });

  it('includes credentials and content-type headers', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue({}),
    });

    await api.get('/headers');

    const [, options] = fetchMock.mock.calls[0];
    expect(options.credentials).toBe('include');
    expect((options.headers as Headers).get('Content-Type')).toBe('application/json');
  });

  it('createPost maps fields', async () => {
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith('/auth/csrf')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: vi.fn().mockResolvedValue({ token: 'csrf-token' }),
        });
      }
      return Promise.resolve({
        ok: true,
        status: 200,
        json: vi.fn().mockResolvedValue({
          post: {
            id: 'post-1',
            user_id: 'user-1',
            section_id: 'section-1',
            content: 'Hello',
            created_at: '2025-01-01T00:00:00Z',
          },
        }),
      });
    });

    const response = await api.createPost({
      sectionId: 'section-1',
      content: 'Hello',
      links: [{ url: 'https://example.com' }],
      images: [{ url: '/api/v1/uploads/user-1/photo.png' }],
    });

    const postCall = findCall('/posts');
    const body = JSON.parse(postCall?.[1]?.body as string);
    expect(body.section_id).toBe('section-1');
    expect(body.content).toBe('Hello');
    expect(body.images).toEqual([{ url: '/api/v1/uploads/user-1/photo.png' }]);
    expect(response.post.createdAt).toBe('2025-01-01T00:00:00Z');
    expect(response.post.userId).toBe('user-1');
  });

  it('getPost maps fields', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue({
        post: {
          id: 'post-2',
          user_id: 'user-2',
          section_id: 'section-9',
          content: 'Hello',
          created_at: '2025-01-02T00:00:00Z',
        },
      }),
    });

    const response = await api.getPost('post-2');
    expect(response.post?.id).toBe('post-2');
    expect(response.post?.userId).toBe('user-2');
    expect(response.post?.sectionId).toBe('section-9');
  });

  it('createComment maps fields', async () => {
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith('/auth/csrf')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: vi.fn().mockResolvedValue({ token: 'csrf-token' }),
        });
      }
      return Promise.resolve({
        ok: true,
        status: 200,
        json: vi.fn().mockResolvedValue({ comment: {} }),
      });
    });

    await api.createComment({
      postId: 'post-1',
      parentCommentId: 'comment-1',
      imageId: 'image-1',
      content: 'Reply',
      links: [{ url: 'https://example.com' }],
    });

    const commentCall = findCall('/comments');
    const body = JSON.parse(commentCall?.[1]?.body as string);
    expect(body.post_id).toBe('post-1');
    expect(body.parent_comment_id).toBe('comment-1');
    expect(body.image_id).toBe('image-1');
  });

  it('getFeed builds query params', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue({ data: {} }),
    });

    await api.getFeed('section-1', 10, 'cursor-1');

    const [url] = fetchMock.mock.calls[0];
    expect(url).toContain('/sections/section-1/feed');
    expect(url).toContain('limit=10');
    expect(url).toContain('cursor=cursor-1');
  });

  it('getThreadComments builds query params', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue({ comments: [] }),
    });

    await api.getThreadComments('post-1', 25, 'cursor-1');

    const [url] = fetchMock.mock.calls[0];
    expect(url).toContain('/posts/post-1/comments');
    expect(url).toContain('limit=25');
    expect(url).toContain('cursor=cursor-1');
  });

  it('adds csrf header for mutations', async () => {
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith('/auth/csrf')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: vi.fn().mockResolvedValue({ token: 'csrf-123' }),
        });
      }
      return Promise.resolve({
        ok: true,
        status: 200,
        json: vi.fn().mockResolvedValue({}),
      });
    });

    await api.post('/posts', { content: 'hello' });

    const postCall = findCall('/posts');
    const options = postCall?.[1] as RequestInit;
    expect((options.headers as Headers).get('X-CSRF-Token')).toBe('csrf-123');
  });

  it('refreshes csrf token on 403 and retries', async () => {
    let csrfCallCount = 0;
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith('/auth/csrf')) {
        csrfCallCount += 1;
        return Promise.resolve({
          ok: true,
          status: 200,
          json: vi.fn().mockResolvedValue({ token: `csrf-${csrfCallCount}` }),
        });
      }

      if (csrfCallCount === 1) {
        return Promise.resolve({
          ok: false,
          status: 403,
          json: vi.fn().mockResolvedValue({ error: 'Invalid', code: 'INVALID_CSRF_TOKEN' }),
        });
      }

      return Promise.resolve({
        ok: true,
        status: 200,
        json: vi.fn().mockResolvedValue({ ok: true }),
      });
    });

    await api.post('/posts', { content: 'hello' });

    const postCalls = fetchMock.mock.calls.filter(([url]) => (url as string).includes('/posts'));
    const firstHeaders = postCalls[0]?.[1]?.headers as Headers;
    const secondHeaders = postCalls[1]?.[1]?.headers as Headers;
    expect(firstHeaders.get('X-CSRF-Token')).toBe('csrf-1');
    expect(secondHeaders.get('X-CSRF-Token')).toBe('csrf-2');
  });

  it('skips csrf for public auth endpoints', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue({ message: 'ok' }),
    });

    await api.post('/auth/password-reset/redeem', { token: 't', password: 'p' });

    expect(findCall('/auth/csrf')).toBeUndefined();
  });

  it('addToWatchlist posts categories and maps response fields', async () => {
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith('/auth/csrf')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: vi.fn().mockResolvedValue({ token: 'csrf-token' }),
        });
      }

      return Promise.resolve({
        ok: true,
        status: 200,
        json: vi.fn().mockResolvedValue({
          watchlist_items: [
            {
              id: 'wl-1',
              user_id: 'user-1',
              post_id: 'post-1',
              category: 'Favorites',
              created_at: '2026-02-01T00:00:00Z',
            },
          ],
        }),
      });
    });

    const response = await api.addToWatchlist('post-1', ['Favorites']);

    const addCall = findCall('/posts/post-1/watchlist');
    const body = JSON.parse(addCall?.[1]?.body as string);
    expect(body.categories).toEqual(['Favorites']);
    expect(response.watchlistItems).toEqual([
      {
        id: 'wl-1',
        userId: 'user-1',
        postId: 'post-1',
        category: 'Favorites',
        createdAt: '2026-02-01T00:00:00Z',
      },
    ]);
  });

  it('getPostWatchlistInfo maps snake_case response fields', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue({
        save_count: 3,
        users: [{ id: 'user-1', username: 'alice', profile_picture_url: '/avatar.png' }],
        viewer_saved: true,
        viewer_categories: ['Favorites'],
      }),
    });

    const response = await api.getPostWatchlistInfo('post-1');

    expect(response).toEqual({
      saveCount: 3,
      users: [{ id: 'user-1', username: 'alice', displayName: 'alice', avatar: '/avatar.png' }],
      viewerSaved: true,
      viewerCategories: ['Favorites'],
    });
  });

  it('logWatch sends watched_at and maps watch_log response', async () => {
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith('/auth/csrf')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: vi.fn().mockResolvedValue({ token: 'csrf-token' }),
        });
      }

      return Promise.resolve({
        ok: true,
        status: 201,
        json: vi.fn().mockResolvedValue({
          watch_log: {
            id: 'log-1',
            user_id: 'user-1',
            post_id: 'post-1',
            rating: 5,
            notes: 'Great film',
            watched_at: '2026-02-01T12:00:00Z',
          },
        }),
      });
    });

    const response = await api.logWatch('post-1', 5, 'Great film', '2026-02-01T12:00:00Z');

    const call = findCall('/posts/post-1/watch-log');
    const body = JSON.parse(call?.[1]?.body as string);
    expect(body).toEqual({
      rating: 5,
      notes: 'Great film',
      watched_at: '2026-02-01T12:00:00Z',
    });
    expect(response).toEqual({
      watchLog: {
        id: 'log-1',
        userId: 'user-1',
        postId: 'post-1',
        rating: 5,
        notes: 'Great film',
        watchedAt: '2026-02-01T12:00:00Z',
      },
    });
  });

  it('getMyWatchLogs adds query params and maps nextCursor', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue({
        watch_logs: [
          {
            id: 'log-1',
            user_id: 'user-1',
            post_id: 'post-1',
            rating: 4,
            notes: null,
            watched_at: '2026-02-01T12:00:00Z',
          },
        ],
        next_cursor: 'next-1',
      }),
    });

    const response = await api.getMyWatchLogs(10, 'cursor-1');

    const [url] = fetchMock.mock.calls[0];
    expect(url).toContain('/me/watch-logs');
    expect(url).toContain('limit=10');
    expect(url).toContain('cursor=cursor-1');
    expect(response).toEqual({
      watchLogs: [
        {
          id: 'log-1',
          userId: 'user-1',
          postId: 'post-1',
          rating: 4,
          notes: undefined,
          watchedAt: '2026-02-01T12:00:00Z',
          post: undefined,
          user: undefined,
        },
      ],
      nextCursor: 'next-1',
    });
  });
});
