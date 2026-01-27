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

  it('openProfile sets profile view and active user', () => {
    uiStore.openProfile('user-42');
    const state = get(uiStore);
    expect(state.activeView).toBe('profile');
    expect(state.activeProfileUserId).toBe('user-42');
  });

  it('setActiveView clears active profile when leaving profile view', () => {
    uiStore.openProfile('user-42');
    uiStore.setActiveView('feed');
    const state = get(uiStore);
    expect(state.activeView).toBe('feed');
    expect(state.activeProfileUserId).toBeNull();
  });
});
