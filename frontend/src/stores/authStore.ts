import { writable, derived } from 'svelte/store';
import { api } from '../services/api';
import { logWarn } from '../lib/observability/logger';
import { setErrorUser } from '../lib/observability/errorTracker';

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
  mfaChallenge: MfaChallenge | null;
}

interface MeResponse {
  id: string;
  username: string;
  email?: string | null;
  profile_picture_url?: string;
  bio?: string;
  is_admin: boolean;
}

export interface MfaChallenge {
  username: string;
  challengeId?: string;
}

function createAuthStore() {
  const { subscribe, set, update } = writable<AuthState>({
    user: null,
    isLoading: true,
    mfaChallenge: null,
  });

  return {
    subscribe,
    setUser: (user: User | null) => {
      setErrorUser(
        user
          ? { id: user.id, username: user.username, email: user.email }
          : null
      );
      update((state) => ({ ...state, user, isLoading: false, mfaChallenge: null }));
    },
    setMfaChallenge: (challenge: MfaChallenge | null) =>
      update((state) => ({ ...state, mfaChallenge: challenge, isLoading: false })),
    setLoading: (isLoading: boolean) => update((state) => ({ ...state, isLoading })),
    logout: async () => {
      try {
        await api.post('/auth/logout');
      } catch (error) {
        logWarn('Logout request failed', { error });
      }
      api.clearCsrfToken();
      setErrorUser(null);
      set({ user: null, isLoading: false, mfaChallenge: null });
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
        set({ user, isLoading: false, mfaChallenge: null });
        setErrorUser({ id: user.id, username: user.username, email: user.email });
        void api.prefetchCsrfToken();
        return true;
      } catch (error) {
        logWarn('Session check failed', { error });
        api.clearCsrfToken();
        setErrorUser(null);
        set({ user: null, isLoading: false, mfaChallenge: null });
        return false;
      }
    },
  };
}

export const authStore = createAuthStore();

export const isAuthenticated = derived(authStore, ($authStore) => $authStore.user !== null);

export const currentUser = derived(authStore, ($authStore) => $authStore.user);

export const isAdmin = derived(authStore, ($authStore) => $authStore.user?.isAdmin ?? false);

export const mfaChallenge = derived(authStore, ($authStore) => $authStore.mfaChallenge);

export const isMfaRequired = derived(authStore, ($authStore) => $authStore.mfaChallenge !== null);
