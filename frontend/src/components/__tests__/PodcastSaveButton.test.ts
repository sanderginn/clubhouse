import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { get } from 'svelte/store';
import type { PodcastSave } from '../../services/api';
import type { Post } from '../../stores/postStore';

const apiSavePodcast = vi.hoisted(() => vi.fn());
const apiUnsavePodcast = vi.hoisted(() => vi.fn());
const apiGetPostPodcastSaveInfo = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    savePodcast: apiSavePodcast,
    unsavePodcast: apiUnsavePodcast,
    getPostPodcastSaveInfo: apiGetPostPodcastSaveInfo,
  },
}));

const { podcastStore } = await import('../../stores/podcastStore');
const { default: PodcastSaveButton } = await import('../podcasts/PodcastSaveButton.svelte');

function buildPost(id: string): Post {
  return {
    id,
    userId: 'user-1',
    sectionId: 'section-podcast',
    content: `Podcast ${id}`,
    createdAt: '2026-02-10T10:00:00Z',
  };
}

const createDeferred = <T>() => {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
};

beforeEach(() => {
  podcastStore.reset();
  apiSavePodcast.mockReset();
  apiUnsavePodcast.mockReset();
  apiGetPostPodcastSaveInfo.mockReset();

  apiSavePodcast.mockResolvedValue({
    id: 'save-1',
    userId: 'user-1',
    postId: 'post-1',
    createdAt: '2026-02-10T10:00:00Z',
  });
  apiUnsavePodcast.mockResolvedValue(undefined);
  apiGetPostPodcastSaveInfo.mockResolvedValue({
    saveCount: 1,
    users: [],
    viewerSaved: true,
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('PodcastSaveButton', () => {
  it('renders saved state from store-backed saved post IDs on initial render', () => {
    podcastStore.setSavedPosts([buildPost('post-1')], null, false, 'section-podcast');

    render(PodcastSaveButton, {
      postId: 'post-1',
      initialSaved: false,
      initialSaveCount: 0,
    });

    expect(screen.getByText('Saved')).toBeInTheDocument();
    expect(screen.getByTestId('podcast-save-count')).toHaveTextContent('1');
  });

  it('renders unsaved state', () => {
    render(PodcastSaveButton, { postId: 'post-1' });

    expect(screen.getByText('Save for later')).toBeInTheDocument();
    expect(screen.queryByText('Saved')).not.toBeInTheDocument();
  });

  it('applies optimistic save state immediately and syncs after server response', async () => {
    const deferredSave = createDeferred<PodcastSave>();
    apiSavePodcast.mockReturnValueOnce(deferredSave.promise);
    apiGetPostPodcastSaveInfo.mockResolvedValueOnce({
      saveCount: 3,
      users: [],
      viewerSaved: true,
    });

    render(PodcastSaveButton, {
      postId: 'post-1',
      post: buildPost('post-1'),
      initialSaved: false,
      initialSaveCount: 2,
    });

    await fireEvent.click(screen.getByRole('button', { name: 'Save podcast for later' }));

    expect(screen.getByText('Saved')).toBeInTheDocument();
    expect(screen.getByTestId('podcast-save-count')).toHaveTextContent('3');
    expect(screen.getByTestId('podcast-save-spinner')).toBeInTheDocument();
    expect(get(podcastStore).savedPosts.map((post) => post.id)).toEqual(['post-1']);

    deferredSave.resolve({
      id: 'save-1',
      userId: 'user-1',
      postId: 'post-1',
      createdAt: '2026-02-10T10:00:00Z',
    });

    await waitFor(() => {
      expect(apiSavePodcast).toHaveBeenCalledWith('post-1');
      expect(apiGetPostPodcastSaveInfo).toHaveBeenCalledWith('post-1');
    });
    await waitFor(() => {
      expect(screen.queryByTestId('podcast-save-spinner')).not.toBeInTheDocument();
    });
  });

  it('supports unsaving previously saved podcast posts', async () => {
    const deferredUnsave = createDeferred<void>();
    apiUnsavePodcast.mockReturnValueOnce(deferredUnsave.promise);
    apiGetPostPodcastSaveInfo.mockResolvedValueOnce({
      saveCount: 1,
      users: [],
      viewerSaved: false,
    });
    podcastStore.addSavedPost(buildPost('post-1'));

    render(PodcastSaveButton, {
      postId: 'post-1',
      post: buildPost('post-1'),
      initialSaved: true,
      initialSaveCount: 2,
    });

    await fireEvent.click(screen.getByRole('button', { name: 'Remove podcast from saved for later' }));

    expect(screen.getByText('Save for later')).toBeInTheDocument();
    expect(screen.getByTestId('podcast-save-count')).toHaveTextContent('1');
    expect(get(podcastStore).savedPosts).toHaveLength(0);

    deferredUnsave.resolve();

    await waitFor(() => {
      expect(apiUnsavePodcast).toHaveBeenCalledWith('post-1');
      expect(apiGetPostPodcastSaveInfo).toHaveBeenCalledWith('post-1');
    });
  });

  it('rolls back optimistic state and shows an error when save fails', async () => {
    const deferredSave = createDeferred<PodcastSave>();
    apiSavePodcast.mockReturnValueOnce(deferredSave.promise);

    render(PodcastSaveButton, {
      postId: 'post-1',
      post: buildPost('post-1'),
      initialSaved: false,
      initialSaveCount: 0,
    });

    await fireEvent.click(screen.getByRole('button', { name: 'Save podcast for later' }));

    expect(screen.getByText('Saved')).toBeInTheDocument();
    deferredSave.reject(new Error('Network broke'));

    await waitFor(() => {
      expect(screen.getByTestId('podcast-save-error')).toHaveTextContent('Network broke');
    });

    expect(screen.getByText('Save for later')).toBeInTheDocument();
    expect(screen.queryByText('Saved')).not.toBeInTheDocument();
    expect(get(podcastStore).savedPosts).toHaveLength(0);
    expect(apiGetPostPodcastSaveInfo).not.toHaveBeenCalled();
  });

  it('rolls back optimistic removal when unsave fails', async () => {
    const deferredUnsave = createDeferred<void>();
    apiUnsavePodcast.mockReturnValueOnce(deferredUnsave.promise);
    podcastStore.addSavedPost(buildPost('post-1'));

    render(PodcastSaveButton, {
      postId: 'post-1',
      post: buildPost('post-1'),
      initialSaved: true,
      initialSaveCount: 1,
    });

    await fireEvent.click(screen.getByRole('button', { name: 'Remove podcast from saved for later' }));
    expect(get(podcastStore).savedPosts).toHaveLength(0);

    deferredUnsave.reject(new Error('Unsave failed'));

    await waitFor(() => {
      expect(screen.getByTestId('podcast-save-error')).toHaveTextContent('Unsave failed');
    });

    expect(screen.getByText('Saved')).toBeInTheDocument();
    expect(get(podcastStore).savedPosts.map((post) => post.id)).toEqual(['post-1']);
    expect(apiGetPostPodcastSaveInfo).not.toHaveBeenCalled();
  });
});
