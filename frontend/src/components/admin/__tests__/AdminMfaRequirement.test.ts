import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup, waitFor } from '@testing-library/svelte';
import { tick } from 'svelte';

const apiGet = vi.hoisted(() => vi.fn());
const apiPatch = vi.hoisted(() => vi.fn());

vi.mock('../../../services/api', () => ({
  api: {
    get: apiGet,
    patch: apiPatch,
  },
}));

const { default: AdminMfaRequirement } = await import('../AdminMfaRequirement.svelte');

const flush = async () => {
  await tick();
  await Promise.resolve();
  await tick();
};

beforeEach(() => {
  apiGet.mockReset();
  apiPatch.mockReset();
});

afterEach(() => {
  cleanup();
});

describe('AdminMfaRequirement', () => {
  it('shows the current MFA requirement and roster counts', async () => {
    apiGet.mockImplementation((endpoint: string) => {
      if (endpoint === '/admin/config') {
        return Promise.resolve({ config: { mfaRequired: false } });
      }
      if (endpoint === '/admin/users/approved') {
        return Promise.resolve([
          {
            id: 'user-1',
            username: 'lena',
            email: 'lena@example.com',
            is_admin: false,
            approved_at: '2024-01-02T00:00:00Z',
            created_at: '2024-01-01T00:00:00Z',
            totp_enabled: false,
          },
          {
            id: 'user-2',
            username: 'marco',
            email: 'marco@example.com',
            is_admin: false,
            approved_at: '2024-01-02T00:00:00Z',
            created_at: '2024-01-01T00:00:00Z',
            totp_enabled: true,
          },
          {
            id: 'user-3',
            username: 'sasha',
            email: 'sasha@example.com',
            is_admin: true,
            approved_at: '2024-01-02T00:00:00Z',
            created_at: '2024-01-01T00:00:00Z',
            totp_enabled: false,
          },
        ]);
      }
      return Promise.resolve(null);
    });

    render(AdminMfaRequirement);
    await flush();

    await waitFor(() => {
      expect(screen.queryByText('Loading roster…')).not.toBeInTheDocument();
    });

    expect(screen.getByText('MFA optional')).toBeInTheDocument();
    expect(screen.getByText('2 without MFA · 3 total')).toBeInTheDocument();
  });

  it('updates MFA requirement after confirmation', async () => {
    apiGet.mockImplementation((endpoint: string) => {
      if (endpoint === '/admin/config') {
        return Promise.resolve({ config: { mfaRequired: false } });
      }
      if (endpoint === '/admin/users/approved') {
        return Promise.resolve([]);
      }
      return Promise.resolve(null);
    });
    apiPatch.mockResolvedValue({ config: { mfaRequired: true } });

    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);

    render(AdminMfaRequirement);
    await flush();

    const toggle = screen.getByRole('switch', { name: /require mfa for all users/i });
    await fireEvent.click(toggle);

    expect(apiPatch).toHaveBeenCalledWith('/admin/config', { mfa_required: true });

    confirmSpy.mockRestore();
  });
});
