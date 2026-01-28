import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/svelte';
import { writable } from 'svelte/store';

// Mock the pwaStore module with inline mock implementations
vi.mock('../../stores/pwaStore', () => {
  const mockState = writable({
    isInstallable: false,
    isInstalled: false,
    isServiceWorkerReady: true,
    isPushSupported: true,
    isPushSubscribed: false,
    pushPermission: 'default' as NotificationPermission | null,
    updateAvailable: false,
  });

  return {
    pwaStore: {
      subscribe: mockState.subscribe,
      subscribeToPush: vi.fn().mockResolvedValue(true),
      unsubscribeFromPush: vi.fn().mockResolvedValue(true),
    },
    isPushSubscribed: {
      subscribe: (fn: (value: boolean) => void) => mockState.subscribe((s) => fn(s.isPushSubscribed)),
    },
    _setMockState: (newState: Partial<{
      isInstallable: boolean;
      isInstalled: boolean;
      isServiceWorkerReady: boolean;
      isPushSupported: boolean;
      isPushSubscribed: boolean;
      pushPermission: NotificationPermission | null;
      updateAvailable: boolean;
    }>) => {
      mockState.update((s) => ({ ...s, ...newState }));
    },
  };
});

// Import component after mocking
import NotificationSettings from '../NotificationSettings.svelte';
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore - we're using test-only exports
import { pwaStore, _setMockState } from '../../stores/pwaStore';

describe('NotificationSettings', () => {
  beforeEach(() => {
    // Reset mock state
    _setMockState({
      isServiceWorkerReady: true,
      isPushSupported: true,
      isPushSubscribed: false,
      pushPermission: 'default',
    });
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  describe('when push is not supported', () => {
    it('should show "Notifications not supported" message', () => {
      _setMockState({ isPushSupported: false });
      render(NotificationSettings);

      expect(screen.getByText('Notifications not supported')).toBeInTheDocument();
    });
  });

  describe('when service worker is not ready', () => {
    it('should show loading state', () => {
      _setMockState({ isServiceWorkerReady: false });
      render(NotificationSettings);

      expect(screen.getByText('Loading...')).toBeInTheDocument();
    });
  });

  describe('when push is supported and ready', () => {
    it('should show "Enable notifications" when not subscribed', () => {
      _setMockState({ isPushSubscribed: false });
      render(NotificationSettings);

      expect(screen.getByText('Enable notifications')).toBeInTheDocument();
    });

    it('should show "Notifications enabled" when subscribed', () => {
      _setMockState({ isPushSubscribed: true });
      render(NotificationSettings);

      expect(screen.getByText('Notifications enabled')).toBeInTheDocument();
    });

    it('should call subscribeToPush when enabling notifications', async () => {
      _setMockState({ isPushSubscribed: false });
      render(NotificationSettings);

      const button = screen.getByRole('button');
      await fireEvent.click(button);

      expect(pwaStore.subscribeToPush).toHaveBeenCalled();
    });

    it('should call unsubscribeFromPush when disabling notifications', async () => {
      _setMockState({ isPushSubscribed: true });
      render(NotificationSettings);

      const button = screen.getByRole('button');
      await fireEvent.click(button);

      expect(pwaStore.unsubscribeFromPush).toHaveBeenCalled();
    });

    it('should show toggle switch in correct state when not subscribed', () => {
      _setMockState({ isPushSubscribed: false });
      render(NotificationSettings);

      const toggle = screen.getByRole('switch');
      expect(toggle).toHaveAttribute('aria-checked', 'false');
    });

    it('should show toggle switch in correct state when subscribed', () => {
      _setMockState({ isPushSubscribed: true });
      render(NotificationSettings);

      const toggle = screen.getByRole('switch');
      expect(toggle).toHaveAttribute('aria-checked', 'true');
    });
  });

  describe('when permission is denied', () => {
    it('should show "Notifications blocked" message', () => {
      _setMockState({ pushPermission: 'denied' });
      render(NotificationSettings);

      expect(screen.getByText('Notifications blocked')).toBeInTheDocument();
    });

    it('should show help text about browser settings', () => {
      _setMockState({ pushPermission: 'denied' });
      render(NotificationSettings);

      expect(
        screen.getByText('Notifications blocked. Enable notifications in your browser settings.')
      ).toBeInTheDocument();
    });

    it('should disable the button when permission is denied', () => {
      _setMockState({ pushPermission: 'denied' });
      render(NotificationSettings);

      const button = screen.getByRole('button');
      expect(button).toBeDisabled();
    });

    it('should not show toggle switch when permission is denied', () => {
      _setMockState({ pushPermission: 'denied' });
      render(NotificationSettings);

      expect(screen.queryByRole('switch')).not.toBeInTheDocument();
    });
  });

  describe('error handling', () => {
    it('should show error when subscribeToPush fails', async () => {
      _setMockState({ isPushSubscribed: false });
      (pwaStore.subscribeToPush as ReturnType<typeof vi.fn>).mockResolvedValueOnce(false);
      render(NotificationSettings);

      const button = screen.getByRole('button');
      await fireEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText('Failed to enable notifications. Please try again.')).toBeInTheDocument();
      });
    });

    it('should show error when unsubscribeFromPush fails', async () => {
      _setMockState({ isPushSubscribed: true });
      (pwaStore.unsubscribeFromPush as ReturnType<typeof vi.fn>).mockResolvedValueOnce(false);
      render(NotificationSettings);

      const button = screen.getByRole('button');
      await fireEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText('Failed to disable notifications. Please try again.')).toBeInTheDocument();
      });
    });

    it('should show a single denied message when permission is denied after subscribe attempt', async () => {
      _setMockState({ isPushSubscribed: false, pushPermission: 'denied' });
      (pwaStore.subscribeToPush as ReturnType<typeof vi.fn>).mockResolvedValueOnce(false);
      render(NotificationSettings);

      const button = screen.getByRole('button');
      await fireEvent.click(button);

      await waitFor(() => {
        const messages = screen.getAllByText(
          'Notifications blocked. Enable notifications in your browser settings.'
        );
        expect(messages).toHaveLength(1);
      });
    });
  });
});
