import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';

const apiGet = vi.hoisted(() => vi.fn());
const apiPost = vi.hoisted(() => vi.fn());
const apiPatch = vi.hoisted(() => vi.fn());
const apiDelete = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    get: apiGet,
    post: apiPost,
    patch: apiPatch,
    delete: apiDelete,
  },
}));

const { default: AdminPanel } = await import('../AdminPanel.svelte');

beforeEach(() => {
  apiGet.mockReset();
  apiPost.mockReset();
  apiPatch.mockReset();
  apiDelete.mockReset();
});

afterEach(() => {
  cleanup();
});

describe('AdminPanel', () => {
  it('switches between pending users and audit logs', async () => {
    apiGet.mockImplementation((endpoint: string) => {
      if (endpoint === '/admin/users') {
        return Promise.resolve([]);
      }
      if (endpoint === '/admin/users/approved') {
        return Promise.resolve([]);
      }
      if (endpoint.startsWith('/admin/audit-logs')) {
        return Promise.resolve({ logs: [], has_more: false });
      }
      return Promise.resolve([]);
    });

    render(AdminPanel);
    await tick();

    expect(screen.getByText('Pending approvals')).toBeInTheDocument();

    const membersTab = screen.getByText('Members');
    await fireEvent.click(membersTab);
    const refreshButtons = screen.getAllByText('Refresh');
    await fireEvent.click(refreshButtons[refreshButtons.length - 1]);
    await tick();
    await Promise.resolve();
    await tick();

    expect(await screen.findByText('No approved members yet.')).toBeInTheDocument();

    const auditTab = screen.getByText('Audit Logs');
    await fireEvent.click(auditTab);
    await tick();

    expect(screen.getByText('Audit log')).toBeInTheDocument();
  });
});
