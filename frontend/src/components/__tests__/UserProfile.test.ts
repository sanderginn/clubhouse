import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';

const apiGet = vi.hoisted(() => vi.fn());
const apiAddCommentReaction = vi.hoisted(() => vi.fn());
const apiRemoveCommentReaction = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    get: apiGet,
    addCommentReaction: apiAddCommentReaction,
    removeCommentReaction: apiRemoveCommentReaction,
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
});

describe('UserProfile', () => {
  it('renders profile details from the API', async () => {
    apiGet.mockImplementation((endpoint: string) => {
      if (endpoint === '/users/user-1') {
        return Promise.resolve({
          user: {
            id: 'user-1',
            username: 'Lena',
            created_at: '2025-01-01T00:00:00Z',
          },
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

    render(UserProfile, { userId: 'user-1' });
    await flushProfileLoad();

    expect(await screen.findByText('Lena')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Posts' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Comments' })).toBeInTheDocument();
  });
});
