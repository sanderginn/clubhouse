import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/svelte';
import { tick } from 'svelte';
import { uiStore } from '../../stores';

const { default: Layout } = await import('../Layout.svelte');

describe('Layout', () => {
  it('sets isMobile based on matchMedia changes', async () => {
    const listeners: Array<(event: MediaQueryListEvent) => void> = [];
    const media = {
      matches: true,
      media: '(max-width: 1023px)',
      onchange: null,
      addEventListener: (_event: string, handler: (event: MediaQueryListEvent) => void) => {
        listeners.push(handler);
      },
      removeEventListener: (_event: string, handler: (event: MediaQueryListEvent) => void) => {
        const index = listeners.indexOf(handler);
        if (index >= 0) {
          listeners.splice(index, 1);
        }
      },
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => true,
    } as MediaQueryList;
    const matchMediaSpy = vi.spyOn(window, 'matchMedia').mockReturnValue(media);
    let isMobile = false;
    const unsubscribe = uiStore.subscribe((state) => {
      isMobile = state.isMobile;
    });

    render(Layout);
    await tick();

    expect(isMobile).toBe(true);

    media.matches = false;
    listeners.forEach((handler) => handler(media as MediaQueryListEvent));
    expect(isMobile).toBe(false);
    matchMediaSpy.mockRestore();
    unsubscribe();
  });
});
