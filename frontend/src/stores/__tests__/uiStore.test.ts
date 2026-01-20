import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { uiStore } from '../uiStore';

beforeEach(() => {
  uiStore.setSidebarOpen(true);
  uiStore.setIsMobile(false);
  uiStore.setActiveView('feed');
});

describe('uiStore', () => {
  it('toggleSidebar flips value', () => {
    const before = get(uiStore).sidebarOpen;
    uiStore.toggleSidebar();
    expect(get(uiStore).sidebarOpen).toBe(!before);
  });

  it('setSidebarOpen sets explicit value', () => {
    uiStore.setSidebarOpen(false);
    expect(get(uiStore).sidebarOpen).toBe(false);
  });

  it('setIsMobile sets isMobile and closes sidebar when true', () => {
    uiStore.setSidebarOpen(true);
    uiStore.setIsMobile(true);
    const state = get(uiStore);
    expect(state.isMobile).toBe(true);
    expect(state.sidebarOpen).toBe(false);
  });

  it('setActiveView updates active view', () => {
    uiStore.setActiveView('admin');
    expect(get(uiStore).activeView).toBe('admin');
  });
});
