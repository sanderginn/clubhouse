import { beforeEach, describe, expect, it, vi } from 'vitest';
import { get } from 'svelte/store';
import type { Post } from '../postStore';
import type { RecentPodcastItem } from '../../services/api';

const apiSavePodcast = vi.hoisted(() => vi.fn());
const apiUnsavePodcast = vi.hoisted(() => vi.fn());
const apiGetPostPodcastSaveInfo = vi.hoisted(() => vi.fn());
const apiGetSectionSavedPodcasts = vi.hoisted(() => vi.fn());
const apiGetSectionRecentPodcasts = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    savePodcast: apiSavePodcast,
    unsavePodcast: apiUnsavePodcast,
    getPostPodcastSaveInfo: apiGetPostPodcastSaveInfo,
    getSectionSavedPodcasts: apiGetSectionSavedPodcasts,
    getSectionRecentPodcasts: apiGetSectionRecentPodcasts,
  },
}));

const { podcastStore } = await import('../podcastStore');

function buildPost(id: string, sectionId = 'section-podcast'): Post {
  return {
    id,
    userId: 'user-1',
    sectionId,
    content: `Podcast ${id}`,
    createdAt: '2026-02-10T10:00:00Z',
  };
}

function buildRecentItem(id: string, kind: 'show' | 'episode' = 'show'): RecentPodcastItem {
  return {
    postId: `post-${id}`,
    linkId: `link-${id}`,
    url: `https://example.com/podcast/${id}`,
    podcast: {
      kind,
    },
    userId: 'user-1',
    username: 'sander',
    postCreatedAt: '2026-02-10T10:00:00Z',
    linkCreatedAt: `2026-02-10T10:00:0${id}Z`,
  };
}

beforeEach(() => {
  podcastStore.reset();
  apiSavePodcast.mockReset();
  apiUnsavePodcast.mockReset();
  apiGetPostPodcastSaveInfo.mockReset();
  apiGetSectionSavedPodcasts.mockReset();
  apiGetSectionRecentPodcasts.mockReset();
});

describe('podcastStore', () => {
  it('loadPostSaveInfo updates save info and saved post IDs', async () => {
    apiGetPostPodcastSaveInfo.mockResolvedValue({
      saveCount: 2,
      users: [],
      viewerSaved: true,
    });

    await podcastStore.loadPostSaveInfo('post-1');

    const state = get(podcastStore);
    expect(state.isLoadingSaveInfo).toBe(false);
    expect(state.saveInfoByPostId['post-1']).toEqual({
      saveCount: 2,
      users: [],
      viewerSaved: true,
    });
    expect(state.savedPostIds.has('post-1')).toBe(true);
  });

  it('toggleSave saves post when viewer has not saved it yet', async () => {
    podcastStore.setPostSaveInfo('post-1', {
      saveCount: 0,
      users: [],
      viewerSaved: false,
    });
    apiSavePodcast.mockResolvedValue({
      id: 'save-1',
      userId: 'user-1',
      postId: 'post-1',
      createdAt: '2026-02-10T10:00:00Z',
    });
    apiGetPostPodcastSaveInfo.mockResolvedValue({
      saveCount: 1,
      users: [],
      viewerSaved: true,
    });

    await podcastStore.toggleSave('post-1');

    const state = get(podcastStore);
    expect(apiSavePodcast).toHaveBeenCalledWith('post-1');
    expect(apiUnsavePodcast).not.toHaveBeenCalled();
    expect(state.isTogglingSave).toBe(false);
    expect(state.saveInfoByPostId['post-1']?.viewerSaved).toBe(true);
    expect(state.savedPostIds.has('post-1')).toBe(true);
  });

  it('toggleSave unsaves post and removes it from saved list', async () => {
    podcastStore.setPostSaveInfo('post-1', {
      saveCount: 3,
      users: [],
      viewerSaved: true,
    });
    podcastStore.setSavedPosts([buildPost('post-1')], 'cursor-1', true, 'section-podcast');
    apiUnsavePodcast.mockResolvedValue(undefined);
    apiGetPostPodcastSaveInfo.mockResolvedValue({
      saveCount: 2,
      users: [],
      viewerSaved: false,
    });

    await podcastStore.toggleSave('post-1');

    const state = get(podcastStore);
    expect(apiUnsavePodcast).toHaveBeenCalledWith('post-1');
    expect(apiSavePodcast).not.toHaveBeenCalled();
    expect(state.savedPostIds.has('post-1')).toBe(false);
    expect(state.savedPosts).toHaveLength(0);
  });

  it('loadSavedPodcasts and loadMoreSavedPodcasts handle cursor pagination', async () => {
    apiGetSectionSavedPodcasts
      .mockResolvedValueOnce({
        posts: [buildPost('post-1')],
        hasMore: true,
        nextCursor: 'cursor-next',
      })
      .mockResolvedValueOnce({
        posts: [buildPost('post-1'), buildPost('post-2')],
        hasMore: false,
        nextCursor: undefined,
      });

    await podcastStore.loadSavedPodcasts('section-podcast', 1);
    await podcastStore.loadMoreSavedPodcasts(2);

    const state = get(podcastStore);
    expect(apiGetSectionSavedPodcasts).toHaveBeenNthCalledWith(1, 'section-podcast', 1);
    expect(apiGetSectionSavedPodcasts).toHaveBeenNthCalledWith(
      2,
      'section-podcast',
      2,
      'cursor-next'
    );
    expect(state.savedPosts.map((post) => post.id)).toEqual(['post-1', 'post-2']);
    expect(state.hasMore).toBe(false);
    expect(state.cursor).toBeNull();
  });

  it('loadRecentPodcasts and loadMoreRecentPodcasts handle cursor pagination', async () => {
    apiGetSectionRecentPodcasts
      .mockResolvedValueOnce({
        items: [buildRecentItem('1', 'show')],
        hasMore: true,
        nextCursor: 'cursor-recent-next',
      })
      .mockResolvedValueOnce({
        items: [buildRecentItem('1', 'show'), buildRecentItem('2', 'episode')],
        hasMore: false,
        nextCursor: undefined,
      });

    await podcastStore.loadRecentPodcasts('section-podcast', 1);
    await podcastStore.loadMoreRecentPodcasts(2);

    const state = get(podcastStore);
    expect(apiGetSectionRecentPodcasts).toHaveBeenNthCalledWith(1, 'section-podcast', 1);
    expect(apiGetSectionRecentPodcasts).toHaveBeenNthCalledWith(
      2,
      'section-podcast',
      2,
      'cursor-recent-next'
    );
    expect(state.recentItems.map((item) => item.linkId)).toEqual(['link-1', 'link-2']);
    expect(state.recentHasMore).toBe(false);
    expect(state.recentCursor).toBeNull();
  });

  it('ignores stale recent load responses when a newer request already completed', async () => {
    let resolveFirstRecent: ((value: unknown) => void) | null = null;
    const firstRecentPromise = new Promise((resolve) => {
      resolveFirstRecent = resolve;
    });

    apiGetSectionRecentPodcasts
      .mockReturnValueOnce(firstRecentPromise)
      .mockResolvedValueOnce({
        items: [buildRecentItem('2', 'episode')],
        hasMore: false,
        nextCursor: undefined,
      });

    const firstLoad = podcastStore.loadRecentPodcasts('section-podcast');
    await podcastStore.loadRecentPodcasts('section-podcast');

    resolveFirstRecent?.({
      items: [buildRecentItem('1', 'show')],
      hasMore: false,
      nextCursor: undefined,
    });
    await firstLoad;

    const state = get(podcastStore);
    expect(state.recentItems.map((item) => item.linkId)).toEqual(['link-2']);
  });

  it('sets error on load failure and reset clears store state', async () => {
    apiGetSectionSavedPodcasts.mockRejectedValue(new Error('Failed to load saved podcasts'));
    apiGetSectionRecentPodcasts.mockRejectedValue(new Error('Failed to load recent podcasts'));

    await podcastStore.loadSavedPodcasts('section-podcast');
    await podcastStore.loadRecentPodcasts('section-podcast');
    let state = get(podcastStore);
    expect(state.error).toBe('Failed to load saved podcasts');
    expect(state.recentError).toBe('Failed to load recent podcasts');
    expect(state.isLoadingSaved).toBe(false);
    expect(state.isLoadingRecent).toBe(false);

    podcastStore.reset();
    state = get(podcastStore);
    expect(state.error).toBeNull();
    expect(state.recentError).toBeNull();
    expect(state.recentItems).toHaveLength(0);
    expect(state.savedPosts).toHaveLength(0);
    expect(state.savedPostIds.size).toBe(0);
    expect(state.recentCursor).toBeNull();
    expect(state.recentHasMore).toBe(false);
    expect(state.recentSectionId).toBeNull();
    expect(state.cursor).toBeNull();
    expect(state.hasMore).toBe(false);
    expect(state.sectionId).toBeNull();
  });
});
