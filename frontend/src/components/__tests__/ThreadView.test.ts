import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { get } from 'svelte/store';
import { sectionStore, threadRouteStore, uiStore, activeView } from '../../stores';

const { default: ThreadView } = await import('../ThreadView.svelte');

beforeEach(() => {
  sectionStore.setSections([
    { id: 'section-1', name: 'Music', type: 'music', icon: '🎵', slug: 'music' },
  ]);
  sectionStore.setActiveSection(null);
  threadRouteStore.setTarget('post-1', 'section-1');
  threadRouteStore.setReady();
  uiStore.setActiveView('thread');
});

afterEach(() => {
  cleanup();
  threadRouteStore.clearTarget();
  uiStore.setActiveView('feed');
});

describe('ThreadView', () => {
  it('navigates back to section and resets view state', async () => {
    const pushStateSpy = vi.spyOn(window.history, 'pushState');
    render(ThreadView);

    const button = screen.getByText('Back to feed');
    await fireEvent.click(button);

    expect(get(activeView)).toBe('feed');
    expect(get(threadRouteStore).postId).toBeNull();
    expect(pushStateSpy).toHaveBeenCalledWith(null, '', '/sections/music');
  });
});
