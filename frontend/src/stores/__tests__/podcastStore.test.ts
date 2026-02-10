import { beforeEach, describe, expect, it, vi } from 'vitest';
import { get } from 'svelte/store';
import type { Post } from '../postStore';

const apiSavePodcast = vi.hoisted(() => vi.fn());
const apiUnsavePodcast = vi.hoisted(() => vi.fn());
const apiGetPostPodcastSaveInfo = vi.hoisted(() => vi.fn());
const apiGetSectionSavedPodcasts = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    savePodcast: apiSavePodcast,
    unsavePodcast: apiUnsavePodcast,
    getPostPodcastSaveInfo: apiGetPostPodcastSaveInfo,
    getSectionSavedPodcasts: apiGetSectionSavedPodcasts,
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

beforeEach(() => {
  podcastStore.reset();
  apiSavePodcast.mockReset();
  apiUnsavePodcast.mockReset();
  apiGetPostPodcastSaveInfo.mockReset();
  apiGetSectionSavedPodcasts.mockReset();
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

  it('sets error on load failure and reset clears store state', async () => {
    apiGetSectionSavedPodcasts.mockRejectedValue(new Error('Failed to load saved podcasts'));

    await podcastStore.loadSavedPodcasts('section-podcast');
    let state = get(podcastStore);
    expect(state.error).toBe('Failed to load saved podcasts');
    expect(state.isLoadingSaved).toBe(false);

    podcastStore.reset();
    state = get(podcastStore);
    expect(state.error).toBeNull();
    expect(state.savedPosts).toHaveLength(0);
    expect(state.savedPostIds.size).toBe(0);
    expect(state.cursor).toBeNull();
    expect(state.hasMore).toBe(false);
    expect(state.sectionId).toBeNull();
  });
});
