import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup, waitFor } from '@testing-library/svelte';

const apiPost = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    post: apiPost,
  },
}));

const { default: Register } = await import('../Register.svelte');

afterEach(() => {
  cleanup();
});

beforeEach(() => {
  apiPost.mockReset();
});

describe('Register', () => {
  it('submits username and password only', async () => {
    apiPost.mockResolvedValue({ message: 'ok' });

    render(Register, { onNavigate: vi.fn() });

    expect(screen.queryByLabelText('Email address')).toBeNull();

    await fireEvent.input(screen.getByLabelText('Username'), {
      target: { value: 'newuser' },
    });
    await fireEvent.input(screen.getByLabelText('Password'), {
      target: { value: 'secret' },
    });
    await fireEvent.input(screen.getByLabelText('Confirm Password'), {
      target: { value: 'secret' },
    });
    await fireEvent.click(screen.getByRole('button', { name: /create account/i }));

    await waitFor(() =>
      expect(apiPost).toHaveBeenCalledWith('/auth/register', {
        username: 'newuser',
        password: 'secret',
      })
    );
  });

  it('shows a validation message when passwords do not match', async () => {
    render(Register, { onNavigate: vi.fn() });

    await fireEvent.input(screen.getByLabelText('Username'), {
      target: { value: 'newuser' },
    });
    await fireEvent.input(screen.getByLabelText('Password'), {
      target: { value: 'secret' },
    });
    await fireEvent.input(screen.getByLabelText('Confirm Password'), {
      target: { value: 'not-secret' },
    });
    await fireEvent.click(screen.getByRole('button', { name: /create account/i }));

    expect(screen.getByText('Passwords do not match')).toBeInTheDocument();
    expect(apiPost).not.toHaveBeenCalled();
  });
});
