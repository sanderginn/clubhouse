import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup, waitFor } from '@testing-library/svelte';

const apiPost = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    post: apiPost,
  },
}));

const { default: PasswordReset } = await import('../PasswordReset.svelte');

afterEach(() => {
  cleanup();
});

beforeEach(() => {
  apiPost.mockReset();
});

describe('PasswordReset', () => {
  it('submits token and new password', async () => {
    apiPost.mockResolvedValue({ message: 'ok' });
    const onNavigate = vi.fn();

    render(PasswordReset, { token: 'reset-token', onNavigate });

    await fireEvent.input(screen.getByLabelText('New password'), {
      target: { value: 'longpassword12' },
    });
    await fireEvent.input(screen.getByLabelText('Confirm new password'), {
      target: { value: 'longpassword12' },
    });
    await fireEvent.click(screen.getByRole('button', { name: /reset password/i }));

    await waitFor(() =>
      expect(apiPost).toHaveBeenCalledWith('/auth/password-reset/redeem', {
        token: 'reset-token',
        new_password: 'longpassword12',
      })
    );
  });

  it('shows a validation message when token is missing', async () => {
    render(PasswordReset, { token: '', onNavigate: vi.fn() });

    await fireEvent.input(screen.getByLabelText('New password'), {
      target: { value: 'longpassword12' },
    });
    await fireEvent.input(screen.getByLabelText('Confirm new password'), {
      target: { value: 'longpassword12' },
    });
    await fireEvent.click(screen.getByRole('button', { name: /reset password/i }));

    expect(screen.getByText('Reset token is required')).toBeInTheDocument();
    expect(apiPost).not.toHaveBeenCalled();
  });
});
