import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Post } from '../../stores/postStore';
import { podcastStore } from '../../stores/podcastStore';
import { sectionStore } from '../../stores/sectionStore';

const apiGetSectionRecentPodcasts = vi.hoisted(() => vi.fn());
const apiGetSectionSavedPodcasts = vi.hoisted(() => vi.fn());
const apiGetPostPodcastSaveInfo = vi.hoisted(() => vi.fn());
const apiSavePodcast = vi.hoisted(() => vi.fn());
const apiUnsavePodcast = vi.hoisted(() => vi.fn());
const pushPath = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    getSectionRecentPodcasts: apiGetSectionRecentPodcasts,
    getSectionSavedPodcasts: apiGetSectionSavedPodcasts,
    getPostPodcastSaveInfo: apiGetPostPodcastSaveInfo,
    savePodcast: apiSavePodcast,
    unsavePodcast: apiUnsavePodcast,
  },
}));

vi.mock('../../services/routeNavigation', () => ({
  buildStandaloneThreadHref: (postId: string) => `/posts/${postId}`,
  pushPath,
}));

const { default: PodcastsTopContainer } = await import('../podcasts/PodcastsTopContainer.svelte');

function setActiveSection(type: 'podcast' | 'general') {
  sectionStore.setActiveSection({
    id: 'section-1',
    name: type === 'podcast' ? 'Podcasts' : 'General',
    type,
    icon: type === 'podcast' ? '🎙️' : '💬',
    slug: type === 'podcast' ? 'podcasts' : 'general',
  });
}

function buildSavedPost(id = 'post-1', title = 'Saved episode'): Post {
  return {
    id,
    userId: 'user-1',
    sectionId: 'section-1',
    content: `Saved podcast post ${id}`,
    createdAt: '2026-02-10T10:00:00Z',
    user: {
      id: 'user-1',
      username: 'sander',
    },
    links: [
      {
        url: 'https://example.com/podcast/episode',
        metadata: {
          title,
          podcast: {
            kind: 'episode',
          },
        },
      },
    ],
  };
}

beforeEach(() => {
  apiGetSectionRecentPodcasts.mockReset();
  apiGetSectionSavedPodcasts.mockReset();
  apiGetPostPodcastSaveInfo.mockReset();
  apiSavePodcast.mockReset();
  apiUnsavePodcast.mockReset();
  pushPath.mockReset();
  podcastStore.reset();
  sectionStore.setActiveSection(null);
});

afterEach(() => {
  cleanup();
  podcastStore.reset();
  sectionStore.setActiveSection(null);
});

describe('PodcastsTopContainer', () => {
  it('does not render outside podcast sections', () => {
    setActiveSection('general');
    render(PodcastsTopContainer);

    expect(screen.queryByTestId('podcasts-top-container')).not.toBeInTheDocument();
    expect(apiGetSectionRecentPodcasts).not.toHaveBeenCalled();
    expect(apiGetSectionSavedPodcasts).not.toHaveBeenCalled();
  });

  it('shows recent mode by default and switches to saved mode', async () => {
    apiGetSectionRecentPodcasts.mockResolvedValue({
      items: [
        {
          postId: 'post-1',
          linkId: 'link-1',
          url: 'https://example.com/podcast/show',
          podcast: { kind: 'show' },
          userId: 'user-1',
          username: 'sander',
          postCreatedAt: '2026-02-10T10:00:00Z',
          linkCreatedAt: '2026-02-10T10:01:00Z',
        },
        {
          postId: 'post-2',
          linkId: 'link-2',
          url: 'https://example.com/podcast/episode-42',
          title: 'Episode 42: Debugging',
          podcast: { kind: 'episode' },
          userId: 'user-2',
          username: 'taylor',
          postCreatedAt: '2026-02-10T10:02:00Z',
          linkCreatedAt: '2026-02-10T10:03:00Z',
        },
      ],
      hasMore: false,
      nextCursor: undefined,
    });
    apiGetSectionSavedPodcasts.mockResolvedValue({
      posts: [buildSavedPost()],
      hasMore: false,
      nextCursor: undefined,
    });

    setActiveSection('podcast');
    render(PodcastsTopContainer);

    expect(await screen.findByTestId('podcasts-recent-list')).toBeInTheDocument();
    expect(screen.getByText('Show')).toBeInTheDocument();
    expect(screen.getByText('Episode 42: Debugging')).toBeInTheDocument();
    expect(screen.getByText('Episode')).toBeInTheDocument();

    await fireEvent.click(screen.getByTestId('podcasts-mode-saved'));

    expect(await screen.findByTestId('podcasts-saved-list')).toBeInTheDocument();
    expect(screen.getByText('Saved episode')).toBeInTheDocument();
  });

  it('shows loading, error, and empty states across modes', async () => {
    let resolveRecent: ((value: unknown) => void) | null = null;
    const recentPromise = new Promise((resolve) => {
      resolveRecent = resolve;
    });

    apiGetSectionRecentPodcasts.mockReturnValueOnce(recentPromise).mockRejectedValueOnce(
      new Error('Recent failed')
    );
    apiGetSectionSavedPodcasts.mockResolvedValue({
      posts: [],
      hasMore: false,
      nextCursor: undefined,
    });

    setActiveSection('podcast');
    render(PodcastsTopContainer);

    expect(await screen.findByTestId('podcasts-recent-loading')).toBeInTheDocument();

    resolveRecent?.({
      items: [],
      hasMore: false,
      nextCursor: undefined,
    });
    expect(await screen.findByTestId('podcasts-recent-empty')).toBeInTheDocument();

    await podcastStore.loadRecentPodcasts('section-1');
    expect(await screen.findByTestId('podcasts-recent-error')).toBeInTheDocument();

    await fireEvent.click(screen.getByTestId('podcasts-mode-saved'));
    expect(await screen.findByTestId('podcasts-saved-empty')).toBeInTheDocument();
  });

  it('reloads saved podcasts across mount cycles for refresh persistence', async () => {
    apiGetSectionRecentPodcasts.mockResolvedValue({
      items: [],
      hasMore: false,
      nextCursor: undefined,
    });
    apiGetSectionSavedPodcasts.mockResolvedValue({
      posts: [buildSavedPost()],
      hasMore: false,
      nextCursor: undefined,
    });

    setActiveSection('podcast');
    const firstRender = render(PodcastsTopContainer);

    await screen.findByTestId('podcasts-recent-empty');
    await fireEvent.click(screen.getByTestId('podcasts-mode-saved'));
    expect(await screen.findByText('Saved episode')).toBeInTheDocument();
    await waitFor(() => {
      expect(apiGetSectionSavedPodcasts).toHaveBeenCalledTimes(1);
    });

    firstRender.unmount();

    render(PodcastsTopContainer);
    await screen.findByTestId('podcasts-recent-empty');
    await fireEvent.click(screen.getByTestId('podcasts-mode-saved'));
    expect(await screen.findByText('Saved episode')).toBeInTheDocument();
    await waitFor(() => {
      expect(apiGetSectionSavedPodcasts).toHaveBeenCalledTimes(2);
    });
  });

  it('reactively removes deleted posts from the saved tab without a refresh', async () => {
    apiGetSectionRecentPodcasts.mockResolvedValue({
      items: [],
      hasMore: false,
      nextCursor: undefined,
    });
    apiGetSectionSavedPodcasts.mockResolvedValue({
      posts: [buildSavedPost('post-1', 'Saved episode A'), buildSavedPost('post-2', 'Saved episode B')],
      hasMore: false,
      nextCursor: undefined,
    });

    setActiveSection('podcast');
    render(PodcastsTopContainer);

    await screen.findByTestId('podcasts-recent-empty');
    await fireEvent.click(screen.getByTestId('podcasts-mode-saved'));
    expect(await screen.findByText('Saved episode A')).toBeInTheDocument();
    expect(screen.getByText('Saved episode B')).toBeInTheDocument();

    podcastStore.handlePostDeleted('post-1');

    await waitFor(() => {
      expect(screen.queryByText('Saved episode A')).not.toBeInTheDocument();
    });
    expect(screen.getByText('Saved episode B')).toBeInTheDocument();
  });

  it('keeps latest recent podcasts after navigation when an older request resolves late', async () => {
    let resolveFirstRecent: ((value: unknown) => void) | null = null;
    const firstRecentPromise = new Promise((resolve) => {
      resolveFirstRecent = resolve;
    });

    apiGetSectionRecentPodcasts
      .mockReturnValueOnce(firstRecentPromise)
      .mockResolvedValueOnce({
        items: [
          {
            postId: 'post-2',
            linkId: 'link-2',
            url: 'https://example.com/podcast/new',
            title: 'Newest podcast',
            podcast: { kind: 'episode' },
            userId: 'user-2',
            username: 'taylor',
            postCreatedAt: '2026-02-10T11:00:00Z',
            linkCreatedAt: '2026-02-10T11:01:00Z',
          },
        ],
        hasMore: false,
        nextCursor: undefined,
      });
    apiGetSectionSavedPodcasts.mockResolvedValue({
      posts: [],
      hasMore: false,
      nextCursor: undefined,
    });

    setActiveSection('podcast');
    render(PodcastsTopContainer);
    expect(await screen.findByTestId('podcasts-recent-loading')).toBeInTheDocument();

    setActiveSection('general');
    await waitFor(() => {
      expect(screen.queryByTestId('podcasts-top-container')).not.toBeInTheDocument();
    });

    setActiveSection('podcast');
    expect(await screen.findByText('Newest podcast')).toBeInTheDocument();

    resolveFirstRecent?.({
      items: [],
      hasMore: false,
      nextCursor: undefined,
    });
    await firstRecentPromise;

    expect(screen.getByText('Newest podcast')).toBeInTheDocument();
    expect(screen.queryByTestId('podcasts-recent-empty')).not.toBeInTheDocument();
  });
});
