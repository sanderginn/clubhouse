import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';

const apiGet = vi.hoisted(() => vi.fn());
const apiPost = vi.hoisted(() => vi.fn());

vi.mock('../../../services/api', () => ({
  api: {
    get: apiGet,
    post: apiPost,
  },
}));

const { default: UserResetLinks } = await import('../UserResetLinks.svelte');

const flushUsersLoad = async () => {
  await tick();
  await Promise.resolve();
  await tick();
};

beforeEach(() => {
  apiGet.mockReset();
  apiPost.mockReset();
});

afterEach(() => {
  cleanup();
});

describe('UserResetLinks', () => {
  it('renders approved users from the API', async () => {
    apiGet.mockResolvedValue([
      {
        id: 'user-1',
        username: 'lena',
        email: 'lena@example.com',
        is_admin: false,
        approved_at: '2024-01-02T00:00:00Z',
        created_at: '2024-01-01T00:00:00Z',
      },
    ]);

    render(UserResetLinks);
    await fireEvent.click(screen.getByText('Refresh'));
    await flushUsersLoad();

    const profileLink = await screen.findByRole('link', { name: "View lena's profile" });
    expect(profileLink).toHaveAttribute('href', '/users/user-1');
    expect(screen.getByText('lena@example.com')).toBeInTheDocument();
    if (typeof AbortController === 'undefined') {
      expect(apiGet).toHaveBeenCalledWith('/admin/users/approved');
    } else {
      expect(apiGet).toHaveBeenCalledWith(
        '/admin/users/approved',
        expect.objectContaining({ signal: expect.any(AbortSignal) })
      );
    }
  });

  it('generates a reset link and displays copy actions', async () => {
    apiGet.mockResolvedValue([
      {
        id: 'user-2',
        username: 'marco',
        email: 'marco@example.com',
        is_admin: true,
        approved_at: '2024-02-01T00:00:00Z',
        created_at: '2024-01-20T00:00:00Z',
      },
    ]);
    apiPost.mockResolvedValue({
      token: 'reset-token',
      user_id: 'user-2',
      expires_at: '2024-02-02T00:00:00Z',
    });

    render(UserResetLinks);
    await fireEvent.click(screen.getByText('Refresh'));
    await flushUsersLoad();
    await screen.findByText('marco');

    const button = screen.getByText('Generate reset link');
    await fireEvent.click(button);
    await tick();

    expect(apiPost).toHaveBeenCalledWith('/admin/password-reset/generate', {
      user_id: 'user-2',
    });
    expect(await screen.findByText('One-time link')).toBeInTheDocument();
    expect(screen.getByText('Copy link')).toBeInTheDocument();
    expect(screen.getByText(/Single-use link/)).toBeInTheDocument();
  });

  it('treats a null response as no approved users', async () => {
    apiGet.mockResolvedValue(null);

    render(UserResetLinks);
    await fireEvent.click(screen.getByText('Refresh'));
    await flushUsersLoad();

    expect(await screen.findByText('No approved members yet.')).toBeInTheDocument();
  });
});
