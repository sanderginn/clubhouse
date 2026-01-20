import { writable, derived } from 'svelte/store';

interface UIState {
  sidebarOpen: boolean;
  isMobile: boolean;
  activeView: 'feed' | 'admin';
}

function createUIStore() {
  const { subscribe, update } = writable<UIState>({
    sidebarOpen: true,
    isMobile: false,
    activeView: 'feed',
  });

  return {
    subscribe,
    toggleSidebar: () => update((state) => ({ ...state, sidebarOpen: !state.sidebarOpen })),
    setSidebarOpen: (open: boolean) => update((state) => ({ ...state, sidebarOpen: open })),
    setIsMobile: (isMobile: boolean) =>
      update((state) => ({
        ...state,
        isMobile,
        sidebarOpen: isMobile ? false : state.sidebarOpen,
      })),
    setActiveView: (activeView: 'feed' | 'admin') =>
      update((state) => ({
        ...state,
        activeView,
      })),
  };
}

export const uiStore = createUIStore();

export const activeView = derived(uiStore, ($uiStore) => $uiStore.activeView);
