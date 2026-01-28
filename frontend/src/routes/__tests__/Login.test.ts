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

  it('clears validation errors when the user edits fields', async () => {
    render(Login, { onNavigate: vi.fn() });

    await fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    expect(screen.getByText('Username and password are required')).toBeInTheDocument();

    await fireEvent.input(screen.getByLabelText('Username'), {
      target: { value: 'sander' },
    });

    expect(screen.queryByText('Username and password are required')).toBeNull();
  });

  it('prompts for TOTP when MFA is required', async () => {
    const totpError = new Error('TOTP required') as Error & { code?: string };
    totpError.code = 'TOTP_REQUIRED';

    apiPost
      .mockRejectedValueOnce(totpError)
      .mockResolvedValueOnce({
        id: 'user-1',
        username: 'sander',
        email: 'sander@example.com',
        is_admin: true,
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

    expect(await screen.findByPlaceholderText('6-digit authentication code')).toBeInTheDocument();
    expect(screen.queryByText('Authentication code is required')).toBeNull();

    await fireEvent.input(screen.getByPlaceholderText('6-digit authentication code'), {
      target: { value: '123456' },
    });
    await fireEvent.click(screen.getByRole('button', { name: /verify code/i }));

    await waitFor(() =>
      expect(apiPost).toHaveBeenLastCalledWith('/auth/login', {
        username: 'sander',
        password: 'secret',
        totp_code: '123456',
      })
    );

    await waitFor(() =>
      expect(setUserSpy).toHaveBeenCalledWith({
        id: 'user-1',
        username: 'sander',
        email: 'sander@example.com',
        isAdmin: true,
      })
    );
  });

  it('shows approval pending message when user is not approved', async () => {
    const approvalError = new Error('Your account is awaiting admin approval.') as Error & {
      code?: string;
    };
    approvalError.code = 'USER_NOT_APPROVED';

    apiPost.mockRejectedValueOnce(approvalError);

    render(Login, { onNavigate: vi.fn() });

    await fireEvent.input(screen.getByLabelText('Username'), {
      target: { value: 'sander' },
    });
    await fireEvent.input(screen.getByLabelText('Password'), {
      target: { value: 'secret' },
    });
    await fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    expect(await screen.findByText('Your account is awaiting admin approval.')).toBeInTheDocument();
  });
});
