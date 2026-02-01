import { derived, get, writable } from 'svelte/store';
import { api } from '../services/api';
import { logWarn } from '../lib/observability/logger';
import { isAuthenticated } from './authStore';
import { mapApiNotification, mapNotificationPayload, type ApiNotification } from './notificationMapper';

export interface Notification {
  id: string;
  userId?: string;
  type: string;
  relatedPostId?: string | null;
  relatedCommentId?: string | null;
  relatedUserId?: string | null;
  relatedUser?: {
    id: string;
    username: string;
    profilePictureUrl?: string | null;
  };
  contentExcerpt?: string | null;
  readAt?: string | null;
  createdAt: string;
}

interface NotificationState {
  notifications: Notification[];
  isLoading: boolean;
  error: string | null;
  paginationError: string | null;
  cursor: string | null;
  hasMore: boolean;
  unreadCount: number;
}

interface NotificationsResponse {
  notifications?: ApiNotification[];
  data?: {
    notifications?: ApiNotification[];
  };
  meta?: {
    cursor?: string | null;
    has_more?: boolean;
    hasMore?: boolean;
    unread_count?: number;
    unreadCount?: number;
  };
}

const NOTIFICATION_PAGE_SIZE = 20;

function extractNotifications(response: NotificationsResponse): ApiNotification[] {
  return response.notifications ?? response.data?.notifications ?? [];
}

function extractMeta(response: NotificationsResponse): {
  cursor: string | null;
  hasMore: boolean;
  unreadCount: number | null;
} {
  const meta = response.meta ?? {};
  const cursor = meta.cursor ?? null;
  const hasMore = meta.has_more ?? meta.hasMore ?? false;
  const unreadCount = meta.unread_count ?? meta.unreadCount ?? null;
  return { cursor, hasMore, unreadCount };
}

function createNotificationStore() {
  const { subscribe, set, update } = writable<NotificationState>({
    notifications: [],
    isLoading: false,
    error: null,
    paginationError: null,
    cursor: null,
    hasMore: true,
    unreadCount: 0,
  });

  return {
    subscribe,
    setNotifications: (
      notifications: Notification[],
      cursor: string | null,
      hasMore: boolean,
      unreadCount: number | null
    ) =>
      update((state) => ({
        ...state,
        notifications,
        cursor,
        hasMore,
        unreadCount: unreadCount ?? state.unreadCount,
        isLoading: false,
        error: null,
        paginationError: null,
      })),
    appendNotifications: (
      notifications: Notification[],
      cursor: string | null,
      hasMore: boolean,
      unreadCount: number | null
    ) =>
      update((state) => {
        const seen = new Set(state.notifications.map((notification) => notification.id));
        const unique = notifications.filter((notification) => {
          if (seen.has(notification.id)) {
            return false;
          }
          seen.add(notification.id);
          return true;
        });
        return {
          ...state,
          notifications: [...state.notifications, ...unique],
          cursor,
          hasMore,
          unreadCount: unreadCount ?? state.unreadCount,
          isLoading: false,
          error: null,
          paginationError: null,
        };
      }),
    addNotification: (notification: Notification) =>
      update((state) => {
        const existing = state.notifications.find((item) => item.id === notification.id);
        if (existing) {
          const updated = state.notifications.map((item) =>
            item.id === notification.id ? { ...item, ...notification } : item
          );
          return {
            ...state,
            notifications: updated,
          };
        }
        const unreadIncrement = notification.readAt ? 0 : 1;
        return {
          ...state,
          notifications: [notification, ...state.notifications],
          unreadCount: state.unreadCount + unreadIncrement,
        };
      }),
    markRead: (notificationId: string, readAt?: string | null) =>
      update((state) => {
        let unreadCount = state.unreadCount;
        const notifications = state.notifications.map((notification) => {
          if (notification.id !== notificationId) {
            return notification;
          }
          const wasUnread = !notification.readAt;
          const hasReadAt = readAt !== undefined;
          const nextReadAt = hasReadAt ? readAt : notification.readAt ?? new Date().toISOString();
          const willBeUnread = !nextReadAt;

          if (wasUnread && !willBeUnread) {
            unreadCount = Math.max(0, unreadCount - 1);
          } else if (!wasUnread && willBeUnread) {
            unreadCount += 1;
          }

          return {
            ...notification,
            readAt: nextReadAt ?? null,
          };
        });
        return {
          ...state,
          notifications,
          unreadCount,
        };
      }),
    markAllReadLocal: () =>
      update((state) => ({
        ...state,
        notifications: state.notifications.map((notification) => ({
          ...notification,
          readAt: notification.readAt ?? new Date().toISOString(),
        })),
        unreadCount: 0,
      })),
    incrementUnreadCount: () =>
      update((state) => ({
        ...state,
        unreadCount: state.unreadCount + 1,
      })),
    setUnreadCount: (unreadCount: number) =>
      update((state) => ({
        ...state,
        unreadCount,
      })),
    setLoading: (isLoading: boolean) =>
      update((state) => ({
        ...state,
        isLoading,
        error: isLoading ? null : state.error,
        paginationError: isLoading ? null : state.paginationError,
      })),
    setError: (error: string | null) =>
      update((state) => ({
        ...state,
        error,
        isLoading: false,
        paginationError: null,
      })),
    setPaginationError: (error: string | null) =>
      update((state) => ({
        ...state,
        paginationError: error,
        isLoading: false,
      })),
    reset: () =>
      set({
        notifications: [],
        isLoading: false,
        error: null,
        paginationError: null,
        cursor: null,
        hasMore: true,
        unreadCount: 0,
      }),
  };
}

export const notificationStore = createNotificationStore();

export const unreadRegistrationCount = derived(notificationStore, ($store) =>
  $store.notifications.filter(
    (notification) => notification.type === 'user_registration_pending' && !notification.readAt
  ).length
);

let initialized = false;
let authUnsub: (() => void) | null = null;

export async function loadNotifications(): Promise<void> {
  notificationStore.setLoading(true);
  notificationStore.setPaginationError(null);

  try {
    const response = await api.get<NotificationsResponse>(
      `/notifications?limit=${NOTIFICATION_PAGE_SIZE}`
    );
    const notifications = extractNotifications(response).map(mapApiNotification);
    const meta = extractMeta(response);
    notificationStore.setNotifications(notifications, meta.cursor, meta.hasMore, meta.unreadCount);
  } catch (error) {
    notificationStore.setError(
      error instanceof Error ? error.message : 'Failed to load notifications'
    );
  }
}

export async function loadMoreNotifications(): Promise<void> {
  const state = get(notificationStore);
  if (state.isLoading || !state.hasMore || !state.cursor) {
    return;
  }

  notificationStore.setLoading(true);
  notificationStore.setPaginationError(null);

  try {
    const response = await api.get<NotificationsResponse>(
      `/notifications?limit=${NOTIFICATION_PAGE_SIZE}&cursor=${encodeURIComponent(state.cursor)}`
    );
    const notifications = extractNotifications(response).map(mapApiNotification);
    const meta = extractMeta(response);
    notificationStore.appendNotifications(notifications, meta.cursor, meta.hasMore, meta.unreadCount);
  } catch (error) {
    notificationStore.setPaginationError(
      error instanceof Error ? error.message : 'Failed to load more notifications'
    );
  }
}

export async function markNotificationRead(notificationId: string): Promise<void> {
  const state = get(notificationStore);
  const target = state.notifications.find((notification) => notification.id === notificationId);
  if (!target || target.readAt) {
    return;
  }

  const previousReadAt = target.readAt ?? null;
  notificationStore.markRead(notificationId);

  try {
    const response = await api.patch<{ notification: ApiNotification }>(
      `/notifications/${notificationId}`
    );
    const updated = mapApiNotification(response.notification);
    notificationStore.markRead(updated.id, updated.readAt ?? undefined);
  } catch (error) {
    notificationStore.markRead(notificationId, previousReadAt);
    logWarn('Failed to mark notification as read', { notificationId, error });
    await loadNotifications();
  }
}

export async function markAllNotificationsRead(): Promise<void> {
  const state = get(notificationStore);
  const unreadIds = state.notifications.filter((notification) => !notification.readAt).map((n) => n.id);
  if (unreadIds.length === 0 && state.unreadCount === 0) {
    return;
  }
  const previousUnreadCount = state.unreadCount;

  notificationStore.markAllReadLocal();

  try {
    const response = await api.patch<{ unread_count?: number; unreadCount?: number }>(
      '/notifications/read'
    );
    const unreadCount = response.unread_count ?? response.unreadCount;
    if (typeof unreadCount === 'number') {
      notificationStore.setUnreadCount(unreadCount);
    }
  } catch (error) {
    notificationStore.setUnreadCount(previousUnreadCount);
    logWarn('Failed to mark notifications as read', { error });
    await loadNotifications();
  }
}

export function handleRealtimeNotification(payload: unknown): void {
  const notification = mapNotificationPayload(payload);
  if (notification) {
    notificationStore.addNotification(notification);
    return;
  }
  if (get(isAuthenticated)) {
    void loadNotifications();
  }
}

export function initNotifications(): void {
  if (initialized) {
    return;
  }
  initialized = true;

  authUnsub = isAuthenticated.subscribe((authed) => {
    if (authed) {
      loadNotifications();
    } else {
      notificationStore.reset();
    }
  });

  if (get(isAuthenticated)) {
    loadNotifications();
  }
}

export function cleanupNotifications(): void {
  authUnsub?.();
  authUnsub = null;
  initialized = false;
}
