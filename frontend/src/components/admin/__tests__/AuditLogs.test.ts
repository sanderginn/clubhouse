import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';

const apiGet = vi.hoisted(() => vi.fn());

vi.mock('../../../services/api', () => ({
  api: {
    get: apiGet,
  },
}));

const { default: AuditLogs } = await import('../AuditLogs.svelte');

beforeEach(() => {
  apiGet.mockReset();
});

afterEach(() => {
  cleanup();
});

describe('AuditLogs', () => {
  it('renders logs from API', async () => {
    apiGet.mockResolvedValue({
      logs: [
        {
          id: 'log-1',
          admin_user_id: 'admin-1',
          admin_username: 'sander',
          action: 'approve_user',
          target_user_id: 'user-1',
          target_username: 'johndoe',
          created_at: '2024-01-01T00:00:00Z',
        },
      ],
      has_more: false,
    });

    render(AuditLogs);
    await fireEvent.click(screen.getByText('Refresh'));
    await tick();

    expect(apiGet).toHaveBeenCalledWith('/admin/audit-logs');
    expect(screen.getByText('Approved user johndoe')).toBeInTheDocument();
    expect(screen.getByText(/sander/i)).toBeInTheDocument();
  });

  it('loads more logs when requested', async () => {
    const initialResponse = {
      logs: [
        {
          id: 'log-1',
          admin_user_id: 'admin-1',
          admin_username: 'sander',
          action: 'approve_user',
          target_user_id: 'user-1',
          target_username: 'johndoe',
          created_at: '2024-01-01T00:00:00Z',
        },
      ],
      has_more: true,
      next_cursor: 'cursor-1',
    };

    const loadMoreResponse = {
      logs: [
        {
          id: 'log-2',
          admin_user_id: 'admin-1',
          admin_username: 'sander',
          action: 'reject_user',
          target_user_id: 'user-2',
          target_username: 'janedoe',
          created_at: '2024-01-02T00:00:00Z',
        },
      ],
      has_more: false,
    };

    apiGet.mockImplementation((endpoint: string) => {
      if (endpoint.includes('cursor=cursor-1')) {
        return Promise.resolve(loadMoreResponse);
      }
      return Promise.resolve(initialResponse);
    });

    render(AuditLogs);
    await fireEvent.click(screen.getByText('Refresh'));
    await tick();

    const loadMore = screen.getByText('Load more');
    await fireEvent.click(loadMore);
    await tick();

    expect(apiGet).toHaveBeenCalledWith('/admin/audit-logs?cursor=cursor-1');
    expect(screen.getByText('Rejected user janedoe')).toBeInTheDocument();
  });
});
