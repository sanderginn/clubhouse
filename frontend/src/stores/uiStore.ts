import { writable, derived } from 'svelte/store';

interface UIState {
  sidebarOpen: boolean;
  isMobile: boolean;
  activeView: 'feed' | 'admin' | 'profile' | 'settings';
  activeProfileUserId: string | null;
}

function createUIStore() {
  const { subscribe, update } = writable<UIState>({
    sidebarOpen: true,
    isMobile: false,
    activeView: 'feed',
    activeProfileUserId: null,
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
    setActiveView: (activeView: 'feed' | 'admin' | 'profile' | 'settings') =>
      update((state) => ({
        ...state,
        activeView,
        activeProfileUserId: activeView === 'profile' ? state.activeProfileUserId : null,
      })),
    openProfile: (userId: string) =>
      update((state) => ({
        ...state,
        activeView: 'profile',
        activeProfileUserId: userId,
      })),
  };
}

export const uiStore = createUIStore();

export const activeView = derived(uiStore, ($uiStore) => $uiStore.activeView);
export const activeProfileUserId = derived(
  uiStore,
  ($uiStore) => $uiStore.activeProfileUserId
);
