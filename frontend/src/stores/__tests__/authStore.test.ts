import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';

const apiGet = vi.hoisted(() => vi.fn());
const apiPost = vi.hoisted(() => vi.fn());
const apiPrefetchCsrfToken = vi.hoisted(() => vi.fn());
const apiClearCsrfToken = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    get: apiGet,
    post: apiPost,
    prefetchCsrfToken: apiPrefetchCsrfToken,
    clearCsrfToken: apiClearCsrfToken,
  },
}));

const { authStore, isAuthenticated, isAdmin, currentUser } = await import('../authStore');

beforeEach(() => {
  apiGet.mockReset();
  apiPost.mockReset();
  apiPrefetchCsrfToken.mockReset();
  apiClearCsrfToken.mockReset();
  authStore.setUser(null);
});

describe('authStore', () => {
  it('checkSession success populates user and returns true', async () => {
    apiGet.mockResolvedValue({
      id: 'user-1',
      username: 'sander',
      email: 'sander@example.com',
      profile_picture_url: 'https://example.com/avatar.png',
      bio: 'hello',
      is_admin: true,
    });

    const result = await authStore.checkSession();
    const state = get(authStore);

    expect(result).toBe(true);
    expect(state.isLoading).toBe(false);
    expect(state.user).toEqual({
      id: 'user-1',
      username: 'sander',
      email: 'sander@example.com',
      profilePictureUrl: 'https://example.com/avatar.png',
      bio: 'hello',
      isAdmin: true,
    });
    expect(apiPrefetchCsrfToken).toHaveBeenCalledTimes(1);
  });

  it('checkSession failure clears user and returns false', async () => {
    authStore.setUser({
      id: 'user-1',
      username: 'old',
      email: 'old@example.com',
      isAdmin: false,
    });
    apiGet.mockRejectedValue(new Error('nope'));

    const result = await authStore.checkSession();
    const state = get(authStore);

    expect(result).toBe(false);
    expect(state.user).toBeNull();
    expect(state.isLoading).toBe(false);
    expect(apiClearCsrfToken).toHaveBeenCalledTimes(1);
  });

  it('logout success clears user', async () => {
    authStore.setUser({
      id: 'user-1',
      username: 'sander',
      email: 'sander@example.com',
      isAdmin: false,
    });
    apiPost.mockResolvedValue({});

    await authStore.logout();

    const state = get(authStore);
    expect(state.user).toBeNull();
    expect(state.isLoading).toBe(false);
    expect(apiClearCsrfToken).toHaveBeenCalledTimes(1);
  });

  it('logout failure still clears user', async () => {
    authStore.setUser({
      id: 'user-2',
      username: 'alex',
      email: 'alex@example.com',
      isAdmin: false,
    });
    apiPost.mockRejectedValue(new Error('fail'));

    await authStore.logout();

    const state = get(authStore);
    expect(state.user).toBeNull();
    expect(state.isLoading).toBe(false);
    expect(apiClearCsrfToken).toHaveBeenCalledTimes(1);
  });

  it('derived stores reflect auth state', () => {
    authStore.setUser(null);
    expect(get(isAuthenticated)).toBe(false);
    expect(get(isAdmin)).toBe(false);
    expect(get(currentUser)).toBeNull();

    authStore.setUser({
      id: 'user-3',
      username: 'admin',
      email: 'admin@example.com',
      isAdmin: true,
    });

    expect(get(isAuthenticated)).toBe(true);
    expect(get(isAdmin)).toBe(true);
    expect(get(currentUser)?.username).toBe('admin');
  });
});
