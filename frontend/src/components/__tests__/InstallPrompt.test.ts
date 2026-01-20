import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@testing-library/svelte';
import { writable } from 'svelte/store';

// Mock the pwaStore module with inline mock implementations
vi.mock('../../stores/pwaStore', () => {
  const mockState = writable({
    isInstallable: false,
    isInstalled: false,
    isServiceWorkerReady: false,
    isPushSupported: false,
    isPushSubscribed: false,
    pushPermission: null,
    updateAvailable: false,
  });

  return {
    pwaStore: {
      subscribe: mockState.subscribe,
      promptInstall: vi.fn().mockResolvedValue(true),
      applyUpdate: vi.fn(),
      dismissUpdate: vi.fn(),
    },
    isInstallable: {
      subscribe: (fn: (value: boolean) => void) => mockState.subscribe((s) => fn(s.isInstallable)),
    },
    isInstalled: {
      subscribe: (fn: (value: boolean) => void) => mockState.subscribe((s) => fn(s.isInstalled)),
    },
    updateAvailable: {
      subscribe: (fn: (value: boolean) => void) => mockState.subscribe((s) => fn(s.updateAvailable)),
    },
    _setMockState: (newState: Partial<typeof mockState extends { subscribe: (run: infer Run) => void } ? Run extends (value: infer V) => void ? V : never : never>) => {
      mockState.update((s) => ({ ...s, ...newState }));
    },
  };
});

// Import component after mocking
import InstallPrompt from '../InstallPrompt.svelte';
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore - we're using a test-only export
import { pwaStore, _setMockState } from '../../stores/pwaStore';

describe('InstallPrompt', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    // Reset mock state
    _setMockState({
      isInstallable: false,
      isInstalled: false,
      updateAvailable: false,
    });
    // Mock sessionStorage
    Object.defineProperty(window, 'sessionStorage', {
      value: {
        getItem: vi.fn().mockReturnValue(null),
        setItem: vi.fn(),
      },
      writable: true,
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.useRealTimers();
  });

  it('should not show install prompt by default', () => {
    render(InstallPrompt);
    expect(screen.queryByText('Install Clubhouse')).not.toBeInTheDocument();
  });

  it('should show update banner when update is available', () => {
    _setMockState({ updateAvailable: true });
    render(InstallPrompt);

    expect(screen.getByText('Update available')).toBeInTheDocument();
    expect(screen.getByText('A new version of Clubhouse is available.')).toBeInTheDocument();
    expect(screen.getByText('Update now')).toBeInTheDocument();
  });

  it('should call applyUpdate when Update now is clicked', async () => {
    _setMockState({ updateAvailable: true });
    render(InstallPrompt);

    const updateButton = screen.getByText('Update now');
    await fireEvent.click(updateButton);

    expect(pwaStore.applyUpdate).toHaveBeenCalled();
  });

  it('should call dismissUpdate when Later is clicked', async () => {
    _setMockState({ updateAvailable: true });
    render(InstallPrompt);

    const laterButton = screen.getByText('Later');
    await fireEvent.click(laterButton);

    expect(pwaStore.dismissUpdate).toHaveBeenCalled();
  });
});
