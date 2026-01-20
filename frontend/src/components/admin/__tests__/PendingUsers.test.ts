import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';

const apiGet = vi.hoisted(() => vi.fn());
const apiPatch = vi.hoisted(() => vi.fn());
const apiDelete = vi.hoisted(() => vi.fn());

vi.mock('../../../services/api', () => ({
  api: {
    get: apiGet,
    patch: apiPatch,
    delete: apiDelete,
  },
}));

const { default: PendingUsers } = await import('../PendingUsers.svelte');

beforeEach(() => {
  apiGet.mockReset();
  apiPatch.mockReset();
  apiDelete.mockReset();
});

afterEach(() => {
  cleanup();
});

describe('PendingUsers', () => {
  it('renders pending users from API', async () => {
    apiGet.mockResolvedValue([
      {
        id: 'user-1',
        username: 'lena',
        email: 'lena@example.com',
        created_at: '2024-01-01T00:00:00Z',
      },
    ]);

    render(PendingUsers);
    await fireEvent.click(screen.getByText('Refresh'));
    await tick();

    expect(apiGet).toHaveBeenCalledWith('/admin/users');
    expect(screen.getByText('lena')).toBeInTheDocument();
    expect(screen.getByText('lena@example.com')).toBeInTheDocument();
  });

  it('approves a user and removes them from the list', async () => {
    apiGet.mockResolvedValue([
      {
        id: 'user-2',
        username: 'marco',
        email: 'marco@example.com',
        created_at: '2024-02-01T00:00:00Z',
      },
    ]);
    apiPatch.mockResolvedValue({});

    render(PendingUsers);
    await fireEvent.click(screen.getByText('Refresh'));
    await tick();

    const approveButton = screen.getByText('Approve');
    await fireEvent.click(approveButton);
    await tick();

    expect(apiPatch).toHaveBeenCalledWith('/admin/users/user-2/approve');
    expect(screen.queryByText('marco')).not.toBeInTheDocument();
  });

  it('rejects a user and removes them from the list', async () => {
    apiGet.mockResolvedValue([
      {
        id: 'user-3',
        username: 'sol',
        email: 'sol@example.com',
        created_at: '2024-03-01T00:00:00Z',
      },
    ]);
    apiDelete.mockResolvedValue({});

    render(PendingUsers);
    await fireEvent.click(screen.getByText('Refresh'));
    await tick();

    const rejectButton = screen.getByText('Reject');
    await fireEvent.click(rejectButton);
    await tick();

    expect(apiDelete).toHaveBeenCalledWith('/admin/users/user-3');
    expect(screen.queryByText('sol')).not.toBeInTheDocument();
  });
});
