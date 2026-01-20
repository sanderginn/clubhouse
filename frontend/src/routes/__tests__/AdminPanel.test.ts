import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';

const apiGet = vi.hoisted(() => vi.fn());
const apiPatch = vi.hoisted(() => vi.fn());
const apiDelete = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    get: apiGet,
    patch: apiPatch,
    delete: apiDelete,
  },
}));

const { default: AdminPanel } = await import('../AdminPanel.svelte');

beforeEach(() => {
  apiGet.mockReset();
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
      if (endpoint.startsWith('/admin/audit-logs')) {
        return Promise.resolve({ logs: [], has_more: false });
      }
      return Promise.resolve([]);
    });

    render(AdminPanel);
    await tick();

    expect(screen.getByText('Pending approvals')).toBeInTheDocument();

    const auditTab = screen.getByText('Audit Logs');
    await fireEvent.click(auditTab);
    await tick();

    expect(screen.getByText('Audit log')).toBeInTheDocument();
  });
});
