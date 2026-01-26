import { writable, derived } from 'svelte/store';
import { api } from '../services/api';

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

interface MeResponse {
  id: string;
  username: string;
  email?: string | null;
  profile_picture_url?: string;
  bio?: string;
  is_admin: boolean;
}

function createAuthStore() {
  const { subscribe, set, update } = writable<AuthState>({
    user: null,
    isLoading: true,
  });

  return {
    subscribe,
    setUser: (user: User | null) => update((state) => ({ ...state, user, isLoading: false })),
    setLoading: (isLoading: boolean) => update((state) => ({ ...state, isLoading })),
    logout: async () => {
      try {
        await api.post('/auth/logout');
      } catch {
        // Ignore errors - we're logging out anyway
      }
      api.clearCsrfToken();
      set({ user: null, isLoading: false });
    },
    checkSession: async () => {
      update((state) => ({ ...state, isLoading: true }));
      try {
        const response = await api.get<MeResponse>('/auth/me');
        const user: User = {
          id: response.id,
          username: response.username,
          email: response.email ?? '',
          profilePictureUrl: response.profile_picture_url,
          bio: response.bio,
          isAdmin: response.is_admin,
        };
        set({ user, isLoading: false });
        void api.prefetchCsrfToken();
        return true;
      } catch {
        api.clearCsrfToken();
        set({ user: null, isLoading: false });
        return false;
      }
    },
  };
}

export const authStore = createAuthStore();

export const isAuthenticated = derived(authStore, ($authStore) => $authStore.user !== null);

export const currentUser = derived(authStore, ($authStore) => $authStore.user);

export const isAdmin = derived(authStore, ($authStore) => $authStore.user?.isAdmin ?? false);
