import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
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

function buildSavedPost(): Post {
  return {
    id: 'post-1',
    userId: 'user-1',
    sectionId: 'section-1',
    content: 'Saved podcast post',
    createdAt: '2026-02-10T10:00:00Z',
    user: {
      id: 'user-1',
      username: 'sander',
    },
    links: [
      {
        url: 'https://example.com/podcast/episode',
        metadata: {
          title: 'Saved episode',
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
});
