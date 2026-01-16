import { writable } from 'svelte/store';

interface UIState {
  sidebarOpen: boolean;
  isMobile: boolean;
}

function createUIStore() {
  const { subscribe, update } = writable<UIState>({
    sidebarOpen: true,
    isMobile: false,
  });

  return {
    subscribe,
    toggleSidebar: () =>
      update((state) => ({ ...state, sidebarOpen: !state.sidebarOpen })),
    setSidebarOpen: (open: boolean) =>
      update((state) => ({ ...state, sidebarOpen: open })),
    setIsMobile: (isMobile: boolean) =>
      update((state) => ({
        ...state,
        isMobile,
        sidebarOpen: isMobile ? false : state.sidebarOpen,
      })),
  };
}

export const uiStore = createUIStore();
