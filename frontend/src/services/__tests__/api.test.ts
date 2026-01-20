import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api } from '../api';

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

describe('api client', () => {
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
    fetchMock.mockResolvedValue({
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

    await expect(api.get('/bad')).rejects.toThrow('Bad');
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
    expect(options.headers['Content-Type']).toBe('application/json');
  });

  it('createPost maps fields', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue({ post: {} }),
    });

    await api.createPost({
      sectionId: 'section-1',
      content: 'Hello',
      links: [{ url: 'https://example.com' }],
    });

    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string);
    expect(body.section_id).toBe('section-1');
    expect(body.content).toBe('Hello');
  });

  it('createComment maps fields', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue({ comment: {} }),
    });

    await api.createComment({
      postId: 'post-1',
      parentCommentId: 'comment-1',
      content: 'Reply',
      links: [{ url: 'https://example.com' }],
    });

    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string);
    expect(body.post_id).toBe('post-1');
    expect(body.parent_comment_id).toBe('comment-1');
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
});
