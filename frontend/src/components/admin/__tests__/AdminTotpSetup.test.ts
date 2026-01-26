import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';

const apiPost = vi.hoisted(() => vi.fn());

vi.mock('../../../services/api', () => ({
  api: {
    post: apiPost,
  },
}));

const { default: AdminTotpSetup } = await import('../AdminTotpSetup.svelte');

beforeEach(() => {
  apiPost.mockReset();
});

afterEach(() => {
  cleanup();
});

describe('AdminTotpSetup', () => {
  it('starts enrollment and verifies code', async () => {
    apiPost
      .mockResolvedValueOnce({
        qr_code: 'data:image/png;base64,abc123',
        manual_entry_key: 'MANUAL-KEY',
      })
      .mockResolvedValueOnce({ message: 'Enabled' });

    render(AdminTotpSetup);

    await fireEvent.click(screen.getByText('Start enrollment'));
    await tick();

    expect(apiPost).toHaveBeenCalledWith('/admin/totp/enroll');
    expect(await screen.findByText('Manual entry key')).toBeInTheDocument();
    expect(screen.getByText('MANUAL-KEY')).toBeInTheDocument();

    await fireEvent.input(screen.getByPlaceholderText('123 456'), {
      target: { value: '123 456' },
    });
    await fireEvent.click(screen.getByText('Verify code'));
    await tick();

    expect(apiPost).toHaveBeenLastCalledWith('/admin/totp/verify', { code: '123456' });
    expect(await screen.findByText('Enabled')).toBeInTheDocument();
  });

  it('shows manual enrollment when QR code is missing', async () => {
    apiPost.mockResolvedValueOnce({
      secret: 'SECRET-ONLY',
      otpauth_url: 'otpauth://totp/Clubhouse:admin?secret=SECRET-ONLY',
    });

    render(AdminTotpSetup);

    await fireEvent.click(screen.getByText('Start enrollment'));
    await tick();

    expect(apiPost).toHaveBeenCalledWith('/admin/totp/enroll');
    expect(await screen.findByText('Manual entry key')).toBeInTheDocument();
    expect(screen.getByText('SECRET-ONLY')).toBeInTheDocument();
    expect(screen.getByText(/otpauth:\/\/totp\/Clubhouse/)).toBeInTheDocument();
    expect(screen.getByPlaceholderText('123 456')).toBeInTheDocument();
  });
});
