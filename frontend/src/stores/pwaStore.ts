import { writable, derived, get } from 'svelte/store';
import { api } from '../services/api';

interface PWAState {
  isInstallable: boolean;
  isInstalled: boolean;
  isServiceWorkerReady: boolean;
  isPushSupported: boolean;
  isPushSubscribed: boolean;
  pushPermission: NotificationPermission | null;
  updateAvailable: boolean;
}

interface BeforeInstallPromptEvent extends Event {
  prompt(): Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

const initialState: PWAState = {
  isInstallable: false,
  isInstalled: false,
  isServiceWorkerReady: false,
  isPushSupported: false,
  isPushSubscribed: false,
  pushPermission: null,
  updateAvailable: false,
};

function createPWAStore() {
  const { subscribe, set, update } = writable<PWAState>(initialState);

  let deferredInstallPrompt: BeforeInstallPromptEvent | null = null;
  let serviceWorkerRegistration: ServiceWorkerRegistration | null = null;

  return {
    subscribe,

    init: async () => {
      // Check if already installed (display-mode: standalone)
      const isInstalled =
        window.matchMedia('(display-mode: standalone)').matches ||
        (window.navigator as Navigator & { standalone?: boolean }).standalone === true;

      // Check push notification support
      const isPushSupported = 'PushManager' in window && 'serviceWorker' in navigator;

      // Check notification permission
      const pushPermission = 'Notification' in window ? Notification.permission : null;

      update((state) => ({
        ...state,
        isInstalled,
        isPushSupported,
        pushPermission,
      }));

      // Listen for install prompt
      window.addEventListener('beforeinstallprompt', (e) => {
        e.preventDefault();
        deferredInstallPrompt = e as BeforeInstallPromptEvent;
        update((state) => ({ ...state, isInstallable: true }));
      });

      // Listen for app installed event
      window.addEventListener('appinstalled', () => {
        deferredInstallPrompt = null;
        update((state) => ({
          ...state,
          isInstallable: false,
          isInstalled: true,
        }));
      });

      // Register service worker
      if ('serviceWorker' in navigator) {
        try {
          const registration = await navigator.serviceWorker.register('/sw.js', {
            scope: '/',
          });

          serviceWorkerRegistration = registration;

          update((state) => ({ ...state, isServiceWorkerReady: true }));

          // Check for updates
          registration.addEventListener('updatefound', () => {
            const newWorker = registration.installing;
            if (newWorker) {
              newWorker.addEventListener('statechange', () => {
                if (newWorker.state === 'installed' && navigator.serviceWorker.controller) {
                  update((state) => ({ ...state, updateAvailable: true }));
                }
              });
            }
          });

          // Check if already subscribed to push
          if (isPushSupported) {
            const subscription = await registration.pushManager.getSubscription();
            update((state) => ({ ...state, isPushSubscribed: !!subscription }));
          }

          // Listen for messages from service worker
          navigator.serviceWorker.addEventListener('message', (event) => {
            if (event.data?.type === 'NOTIFICATION_CLICK') {
              // Handle notification click navigation
              const url = event.data.url;
              if (url && url !== window.location.pathname) {
                window.location.href = url;
              }
            }
          });
        } catch (error) {
          console.error('Service worker registration failed:', error);
        }
      }
    },

    promptInstall: async (): Promise<boolean> => {
      if (!deferredInstallPrompt) {
        return false;
      }

      deferredInstallPrompt.prompt();
      const { outcome } = await deferredInstallPrompt.userChoice;

      if (outcome === 'accepted') {
        deferredInstallPrompt = null;
        update((state) => ({
          ...state,
          isInstallable: false,
          isInstalled: true,
        }));
        return true;
      }

      return false;
    },

    subscribeToPush: async (): Promise<boolean> => {
      if (!serviceWorkerRegistration) {
        console.error('Service worker not registered');
        return false;
      }

      try {
        // Request notification permission
        const permission = await Notification.requestPermission();
        update((state) => ({ ...state, pushPermission: permission }));

        if (permission !== 'granted') {
          return false;
        }

        // Get the VAPID public key from the server
        const { publicKey } = await api.get<{ publicKey: string }>('/push/vapid-key');

        // Subscribe to push notifications
        const subscription = await serviceWorkerRegistration.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: urlBase64ToUint8Array(publicKey),
        });

        // Send subscription to server
        await api.post('/push/subscribe', subscription.toJSON());

        update((state) => ({ ...state, isPushSubscribed: true }));
        return true;
      } catch (error) {
        console.error('Failed to subscribe to push notifications:', error);
        return false;
      }
    },

    unsubscribeFromPush: async (): Promise<boolean> => {
      if (!serviceWorkerRegistration) {
        return false;
      }

      try {
        const subscription = await serviceWorkerRegistration.pushManager.getSubscription();
        if (subscription) {
          // Unsubscribe locally
          await subscription.unsubscribe();

          // Notify server
          await api.delete('/push/subscribe');
        }

        update((state) => ({ ...state, isPushSubscribed: false }));
        return true;
      } catch (error) {
        console.error('Failed to unsubscribe from push notifications:', error);
        return false;
      }
    },

    applyUpdate: () => {
      if (serviceWorkerRegistration?.waiting) {
        serviceWorkerRegistration.waiting.postMessage({ type: 'SKIP_WAITING' });
        window.location.reload();
      }
    },

    dismissUpdate: () => {
      update((state) => ({ ...state, updateAvailable: false }));
    },
  };
}

// Helper function to convert VAPID key to ArrayBuffer
function urlBase64ToUint8Array(base64String: string): ArrayBuffer {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');

  const rawData = window.atob(base64);
  const outputArray = new Uint8Array(rawData.length);

  for (let i = 0; i < rawData.length; ++i) {
    outputArray[i] = rawData.charCodeAt(i);
  }
  return outputArray.buffer as ArrayBuffer;
}

export const pwaStore = createPWAStore();

// Derived stores for easier access
export const isInstallable = derived(pwaStore, ($pwa) => $pwa.isInstallable);
export const isInstalled = derived(pwaStore, ($pwa) => $pwa.isInstalled);
export const isPushSubscribed = derived(pwaStore, ($pwa) => $pwa.isPushSubscribed);
export const updateAvailable = derived(pwaStore, ($pwa) => $pwa.updateAvailable);
