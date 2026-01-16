import { writable, derived } from 'svelte/store';

export interface User {
  id: string;
  username: string;
  email: string;
  profilePictureUrl?: string;
  bio?: string;
  isAdmin: boolean;
}

interface AuthState {
  user: User | null;
  isLoading: boolean;
}

function createAuthStore() {
  const { subscribe, set, update } = writable<AuthState>({
    user: null,
    isLoading: true,
  });

  return {
    subscribe,
    setUser: (user: User | null) =>
      update((state) => ({ ...state, user, isLoading: false })),
    setLoading: (isLoading: boolean) =>
      update((state) => ({ ...state, isLoading })),
    logout: () => set({ user: null, isLoading: false }),
  };
}

export const authStore = createAuthStore();

export const isAuthenticated = derived(
  authStore,
  ($authStore) => $authStore.user !== null
);

export const currentUser = derived(authStore, ($authStore) => $authStore.user);

export const isAdmin = derived(
  authStore,
  ($authStore) => $authStore.user?.isAdmin ?? false
);
