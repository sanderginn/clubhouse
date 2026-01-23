import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup, waitFor } from '@testing-library/svelte';

const apiPost = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    post: apiPost,
  },
}));

const { default: Login } = await import('../Login.svelte');
const { authStore } = await import('../../stores');

afterEach(() => {
  cleanup();
});

beforeEach(() => {
  apiPost.mockReset();
  authStore.setUser(null);
});

describe('Login', () => {
  it('submits username and password', async () => {
    apiPost.mockResolvedValue({
      id: 'user-1',
      username: 'sander',
      email: 'sander@example.com',
      is_admin: false,
      message: 'ok',
    });

    const setUserSpy = vi.spyOn(authStore, 'setUser');

    render(Login, { onNavigate: vi.fn() });

    await fireEvent.input(screen.getByLabelText('Username'), {
      target: { value: 'sander' },
    });
    await fireEvent.input(screen.getByLabelText('Password'), {
      target: { value: 'secret' },
    });
    await fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() =>
      expect(apiPost).toHaveBeenCalledWith('/auth/login', {
        username: 'sander',
        password: 'secret',
      })
    );

    await waitFor(() =>
      expect(setUserSpy).toHaveBeenCalledWith({
        id: 'user-1',
        username: 'sander',
        email: 'sander@example.com',
        isAdmin: false,
      })
    );
  });

  it('shows a validation message when missing credentials', async () => {
    render(Login, { onNavigate: vi.fn() });

    await fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    expect(screen.getByText('Username and password are required')).toBeInTheDocument();
    expect(apiPost).not.toHaveBeenCalled();
  });
});
