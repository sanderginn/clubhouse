import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { get } from 'svelte/store';

// Mock the api module
vi.mock('../../services/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    delete: vi.fn(),
  },
}));

// Create mock service worker registration
const mockPushSubscription = {
  unsubscribe: vi.fn().mockResolvedValue(true),
  toJSON: vi.fn().mockReturnValue({
    endpoint: 'https://push.example.com/subscription',
    keys: { auth: 'auth-key', p256dh: 'p256dh-key' },
  }),
};

const mockPushManager = {
  subscribe: vi.fn().mockResolvedValue(mockPushSubscription),
  getSubscription: vi.fn().mockResolvedValue(null),
};

const mockServiceWorkerRegistration: Partial<ServiceWorkerRegistration> = {
  pushManager: mockPushManager as unknown as PushManager,
  waiting: null,
  installing: null,
  active: null,
  addEventListener: vi.fn(),
};

// Store original navigator
const originalNavigator = global.navigator;
const originalWindow = global.window;

describe('pwaStore', () => {
  let pwaStore: typeof import('../pwaStore').pwaStore;
  let isInstallable: typeof import('../pwaStore').isInstallable;
  let isInstalled: typeof import('../pwaStore').isInstalled;
  let isPushSubscribed: typeof import('../pwaStore').isPushSubscribed;
  let updateAvailable: typeof import('../pwaStore').updateAvailable;

  beforeEach(async () => {
    vi.resetModules();

    // Mock navigator.serviceWorker
    Object.defineProperty(global, 'navigator', {
      value: {
        ...originalNavigator,
        serviceWorker: {
          register: vi.fn().mockResolvedValue(mockServiceWorkerRegistration),
          controller: null,
          addEventListener: vi.fn(),
        },
      },
      writable: true,
      configurable: true,
    });

    // Mock window
    Object.defineProperty(global, 'window', {
      value: {
        ...originalWindow,
        matchMedia: vi.fn().mockReturnValue({
          matches: false,
          addEventListener: vi.fn(),
        }),
        addEventListener: vi.fn(),
        Notification: {
          permission: 'default' as NotificationPermission,
          requestPermission: vi.fn().mockResolvedValue('granted'),
        },
        atob: (str: string) => Buffer.from(str, 'base64').toString('binary'),
        sessionStorage: {
          getItem: vi.fn().mockReturnValue(null),
          setItem: vi.fn(),
        },
      },
      writable: true,
      configurable: true,
    });

    // Mock Notification on global as well
    (global as unknown as { Notification: { permission: string } }).Notification = {
      permission: 'default',
    };

    // Re-import to get fresh store
    const module = await import('../pwaStore');
    pwaStore = module.pwaStore;
    isInstallable = module.isInstallable;
    isInstalled = module.isInstalled;
    isPushSubscribed = module.isPushSubscribed;
    updateAvailable = module.updateAvailable;
  });

  afterEach(() => {
    vi.clearAllMocks();
    Object.defineProperty(global, 'navigator', {
      value: originalNavigator,
      writable: true,
      configurable: true,
    });
    Object.defineProperty(global, 'window', {
      value: originalWindow,
      writable: true,
      configurable: true,
    });
  });

  describe('init', () => {
    it('should initialize with default state', async () => {
      const state = get(pwaStore);
      expect(state.isInstallable).toBe(false);
      expect(state.isInstalled).toBe(false);
      expect(state.isPushSubscribed).toBe(false);
      expect(state.updateAvailable).toBe(false);
    });

    it('should register service worker on init', async () => {
      await pwaStore.init();
      expect(navigator.serviceWorker.register).toHaveBeenCalledWith('/sw.js', {
        scope: '/',
      });
    });

    it('should set isServiceWorkerReady after registration', async () => {
      await pwaStore.init();
      const state = get(pwaStore);
      expect(state.isServiceWorkerReady).toBe(true);
    });

    it('should detect standalone mode as installed', async () => {
      (window.matchMedia as ReturnType<typeof vi.fn>).mockReturnValue({
        matches: true, // standalone mode
        addEventListener: vi.fn(),
      });

      await pwaStore.init();
      const state = get(pwaStore);
      expect(state.isInstalled).toBe(true);
    });
  });

  describe('derived stores', () => {
    it('isInstallable should reflect store state', async () => {
      expect(get(isInstallable)).toBe(false);
    });

    it('isInstalled should reflect store state', async () => {
      expect(get(isInstalled)).toBe(false);
    });

    it('isPushSubscribed should reflect store state', async () => {
      expect(get(isPushSubscribed)).toBe(false);
    });

    it('updateAvailable should reflect store state', async () => {
      expect(get(updateAvailable)).toBe(false);
    });
  });

  describe('promptInstall', () => {
    it('should return false if no install prompt is deferred', async () => {
      const result = await pwaStore.promptInstall();
      expect(result).toBe(false);
    });
  });

  describe('applyUpdate', () => {
    it('should not throw when no waiting worker', () => {
      expect(() => pwaStore.applyUpdate()).not.toThrow();
    });
  });

  describe('dismissUpdate', () => {
    it('should set updateAvailable to false', () => {
      pwaStore.dismissUpdate();
      const state = get(pwaStore);
      expect(state.updateAvailable).toBe(false);
    });
  });
});
