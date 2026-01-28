import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { postStore, sectionStore } from '../../stores';
import { afterEach } from 'vitest';
import { tick } from 'svelte';

const loadFeed = vi.hoisted(() => vi.fn());
const loadMorePosts = vi.hoisted(() => vi.fn());

vi.mock('../../stores/feedStore', () => ({
  loadFeed,
  loadMorePosts,
}));

const { default: SectionFeed } = await import('../SectionFeed.svelte');

beforeEach(() => {
  loadFeed.mockReset();
  loadMorePosts.mockReset();
  postStore.reset();
  sectionStore.setActiveSection(null);
});

afterEach(() => {
  cleanup();
});

describe('SectionFeed', () => {
  it('loads feed when active section changes', () => {
    sectionStore.setActiveSection({ id: 'section-1', name: 'Music', type: 'music', icon: '🎵' });
    render(SectionFeed);

    expect(loadFeed).toHaveBeenCalledWith('section-1');
  });

  it('retry button calls loadFeed', async () => {
    sectionStore.setActiveSection({ id: 'section-1', name: 'Music', type: 'music', icon: '🎵' });
    postStore.setError('boom');

    render(SectionFeed);

    const button = screen.getByText('Try again');
    await fireEvent.click(button);

    expect(loadFeed).toHaveBeenCalledWith('section-1');
  });

  it('intersection observer triggers loadMorePosts', async () => {
    sectionStore.setActiveSection({ id: 'section-1', name: 'Music', type: 'music', icon: '🎵' });
    render(SectionFeed);

    postStore.setPosts(
      [
        { id: 'post-1', userId: 'user-1', sectionId: 'section-1', content: 'hello', createdAt: 'now' },
      ],
      'cursor-1',
      true
    );
    await tick();
    await tick();

    const observer = (globalThis as { __lastObserver?: { trigger: (value: boolean) => void } }).__lastObserver;
    observer?.trigger(true);

    expect(loadMorePosts).toHaveBeenCalled();
  });

  it('shows pagination error and allows retry when posts exist', async () => {
    sectionStore.setActiveSection({ id: 'section-1', name: 'Music', type: 'music', icon: '🎵' });
    render(SectionFeed);

    postStore.setPosts(
      [
        { id: 'post-1', userId: 'user-1', sectionId: 'section-1', content: 'hello', createdAt: 'now' },
      ],
      'cursor-1',
      true
    );
    postStore.setPaginationError('Rate limit exceeded');
    await tick();

    expect(screen.getByText(/Could not load more posts/i)).toBeInTheDocument();
    const button = screen.getByText('Try again');
    await fireEvent.click(button);

    expect(loadMorePosts).toHaveBeenCalled();
  });

  it('cleanup resets posts on destroy', () => {
    const resetSpy = vi.spyOn(postStore, 'reset');
    sectionStore.setActiveSection({ id: 'section-1', name: 'Music', type: 'music', icon: '🎵' });

    const { unmount } = render(SectionFeed);
    const observer = (globalThis as { __lastObserver?: { disconnect: () => void } }).__lastObserver;
    const disconnectSpy = observer ? vi.spyOn(observer, 'disconnect') : null;
    unmount();

    expect(resetSpy).toHaveBeenCalled();
    if (disconnectSpy) {
      expect(disconnectSpy).toHaveBeenCalled();
    }
  });
});
