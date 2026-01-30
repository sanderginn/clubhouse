import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';

const apiGet = vi.hoisted(() => vi.fn());
const apiAddCommentReaction = vi.hoisted(() => vi.fn());
const apiRemoveCommentReaction = vi.hoisted(() => vi.fn());
const apiLookupUserByUsername = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    get: apiGet,
    addCommentReaction: apiAddCommentReaction,
    removeCommentReaction: apiRemoveCommentReaction,
    lookupUserByUsername: apiLookupUserByUsername,
  },
}));

const { default: UserProfile } = await import('../UserProfile.svelte');

const flushProfileLoad = async () => {
  await tick();
  await Promise.resolve();
  await tick();
};

afterEach(() => {
  cleanup();
  apiGet.mockReset();
  apiAddCommentReaction.mockReset();
  apiRemoveCommentReaction.mockReset();
  apiLookupUserByUsername.mockReset();
});

describe('UserProfile', () => {
  it('renders profile details from the API', async () => {
    apiGet.mockImplementation((endpoint: string) => {
      if (endpoint === '/users/user-1') {
        return Promise.resolve({
          id: 'user-1',
          username: 'Lena',
          created_at: '2025-01-01T00:00:00Z',
          stats: {
            post_count: 2,
            comment_count: 1,
          },
        });
      }
      if (endpoint.startsWith('/users/user-1/posts')) {
        return Promise.resolve({ posts: [], meta: { cursor: null, hasMore: false } });
      }
      if (endpoint.startsWith('/users/user-1/comments')) {
        return Promise.resolve({ comments: [], meta: { cursor: null, hasMore: false } });
      }
      return Promise.resolve({});
    });
    apiLookupUserByUsername.mockResolvedValue({
      user: { id: 'user-1', username: 'user-1', profile_picture_url: null },
    });

    render(UserProfile, { userId: 'user-1' });
    await flushProfileLoad();

    expect(await screen.findByText('Lena')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Posts' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Comments' })).toBeInTheDocument();
  });

  it('shows comment thread context on profile comments', async () => {
    apiGet.mockImplementation((endpoint: string) => {
      if (endpoint === '/users/user-2') {
        return Promise.resolve({
          id: 'user-2',
          username: 'Sam',
          created_at: '2025-01-01T00:00:00Z',
          stats: {
            post_count: 1,
            comment_count: 1,
          },
        });
      }
      if (endpoint.startsWith('/users/user-2/posts')) {
        return Promise.resolve({ posts: [], meta: { cursor: null, hasMore: false } });
      }
      if (endpoint.startsWith('/users/user-2/comments')) {
        return Promise.resolve({
          comments: [
            {
              id: 'comment-1',
              user_id: 'user-2',
              post_id: 'post-9',
              parent_comment_id: 'comment-parent',
              content: 'Replying with more context.',
              created_at: '2025-02-01T10:00:00Z',
              user: { id: 'user-2', username: 'Sam', profile_picture_url: null },
            },
          ],
          meta: { cursor: null, hasMore: false },
        });
      }
      if (endpoint === '/posts/post-9') {
        return Promise.resolve({
          post: {
            id: 'post-9',
            user_id: 'user-3',
            section_id: 'section-1',
            content: 'A post about the new album drop.',
            created_at: '2025-01-20T10:00:00Z',
            user: { id: 'user-3', username: 'Riley', profile_picture_url: null },
          },
        });
      }
      if (endpoint === '/comments/comment-parent') {
        return Promise.resolve({
          comment: {
            id: 'comment-parent',
            user_id: 'user-4',
            post_id: 'post-9',
            content: 'Parent comment message.',
            created_at: '2025-02-01T09:00:00Z',
            user: { id: 'user-4', username: 'Avery', profile_picture_url: null },
          },
        });
      }
      if (endpoint.startsWith('/posts/post-9/comments')) {
        return Promise.resolve({
          comments: [
            {
              id: 'comment-parent',
              user_id: 'user-4',
              post_id: 'post-9',
              content: 'Parent comment message.',
              created_at: '2025-02-01T09:00:00Z',
              user: { id: 'user-4', username: 'Avery', profile_picture_url: null },
              replies: [
                {
                  id: 'comment-1',
                  user_id: 'user-2',
                  post_id: 'post-9',
                  parent_comment_id: 'comment-parent',
                  content: 'Replying with more context.',
                  created_at: '2025-02-01T10:00:00Z',
                  user: { id: 'user-2', username: 'Sam', profile_picture_url: null },
                },
              ],
            },
          ],
        });
      }
      return Promise.resolve({});
    });
    apiLookupUserByUsername.mockResolvedValue({
      user: { id: 'user-2', username: 'user-2', profile_picture_url: null },
    });

    render(UserProfile, { userId: 'user-2' });
    await flushProfileLoad();

    const commentsTab = await screen.findByRole('button', { name: 'Comments' });
    await commentsTab.click();
    await flushProfileLoad();

    expect(await screen.findByText('Thread context')).toBeInTheDocument();
    expect(await screen.findByText('In reply to')).toBeInTheDocument();
    expect(await screen.findByText('View full thread ->')).toBeInTheDocument();
  });
});
