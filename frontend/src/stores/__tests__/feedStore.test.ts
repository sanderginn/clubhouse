import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { postStore } from '../postStore';
import { sectionStore } from '../sectionStore';

const apiGet = vi.hoisted(() => vi.fn());
const mapApiPost = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    get: apiGet,
  },
}));

vi.mock('../postMapper', () => ({
  mapApiPost: (post: unknown) => mapApiPost(post),
}));

const { loadFeed, loadMorePosts } = await import('../feedStore');

beforeEach(() => {
  apiGet.mockReset();
  mapApiPost.mockReset();
  postStore.reset();
  sectionStore.setActiveSection(null);
});

const mapPost = (id: string) => ({
  id,
  userId: 'user-1',
  sectionId: 'section-1',
  content: 'hello',
  createdAt: 'now',
});

describe('feedStore', () => {
  it('loadFeed success maps posts and sets cursor/hasMore', async () => {
    mapApiPost.mockImplementation((post: { id: string }) => mapPost(post.id));
    apiGet.mockResolvedValue({
      posts: [{ id: 'post-1' }, { id: 'post-2' }],
      has_more: true,
      next_cursor: 'cursor-1',
    });

    await loadFeed('section-1');
    const state = get(postStore);

    expect(state.posts).toHaveLength(2);
    expect(mapApiPost).toHaveBeenCalledTimes(2);
    expect(state.cursor).toBe('cursor-1');
    expect(state.hasMore).toBe(true);
    expect(state.isLoading).toBe(false);
  });

  it('loadFeed reads meta cursor when present', async () => {
    mapApiPost.mockImplementation((post: { id: string }) => mapPost(post.id));
    apiGet.mockResolvedValue({
      data: { posts: [{ id: 'post-1' }] },
      meta: { cursor: 'cursor-2', has_more: true },
    });

    await loadFeed('section-1');
    const state = get(postStore);

    expect(state.posts).toHaveLength(1);
    expect(state.cursor).toBe('cursor-2');
    expect(state.hasMore).toBe(true);
  });

  it('loadFeed failure sets error and stops loading', async () => {
    apiGet.mockRejectedValue(new Error('fail'));

    await loadFeed('section-1');
    const state = get(postStore);
    expect(state.error).toBe('fail');
    expect(state.isLoading).toBe(false);
  });

  it('loadMorePosts guard prevents request', async () => {
    postStore.reset();
    await loadMorePosts();
    expect(apiGet).not.toHaveBeenCalled();

    postStore.setPosts([], null, false);
    sectionStore.setActiveSection({
      id: 'section-1',
      name: 'Music',
      type: 'music',
      icon: '🎵',
      slug: 'music',
    });
    await loadMorePosts();
    expect(apiGet).not.toHaveBeenCalled();
  });

  it('loadMorePosts success appends posts', async () => {
    mapApiPost.mockImplementation((post: { id: string }) => mapPost(post.id));
    sectionStore.setActiveSection({
      id: 'section-1',
      name: 'Music',
      type: 'music',
      icon: '🎵',
      slug: 'music',
    });
    postStore.setPosts([mapPost('post-0')], 'cursor-1', true);

    apiGet.mockResolvedValue({
      posts: [{ id: 'post-1' }],
      has_more: false,
      next_cursor: null,
    });

    await loadMorePosts();
    const state = get(postStore);
    expect(state.posts).toHaveLength(2);
    expect(state.hasMore).toBe(false);
    expect(state.cursor).toBe(null);
  });

  it('loadMorePosts failure sets pagination error', async () => {
    sectionStore.setActiveSection({
      id: 'section-1',
      name: 'Music',
      type: 'music',
      icon: '🎵',
      slug: 'music',
    });
    postStore.setPosts([mapPost('post-0')], 'cursor-1', true);

    apiGet.mockRejectedValue(new Error('boom'));

    await loadMorePosts();
    const state = get(postStore);
    expect(state.paginationError).toBe('boom');
    expect(state.isLoading).toBe(false);
  });
});
