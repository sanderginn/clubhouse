import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { createRequire } from 'module';

const require = createRequire(import.meta.url);
const { writable } = require('svelte/store') as typeof import('svelte/store');

const apiGet = vi.hoisted(() => vi.fn());
const apiPatch = vi.hoisted(() => vi.fn());
const logWarn = vi.hoisted(() => vi.fn());

const storeRefs: {
  isAuthenticated: ReturnType<typeof writable>;
} = {} as any;

vi.mock('../../services/api', () => ({
  api: {
    get: apiGet,
    patch: apiPatch,
  },
}));

vi.mock('../../lib/observability/logger', () => ({
  logWarn: (...args: any[]) => logWarn(...args),
}));

vi.mock('../authStore', () => {
  storeRefs.isAuthenticated = writable(false);
  return {
    isAuthenticated: storeRefs.isAuthenticated,
  };
});

const { notificationStore, markNotificationRead, markVisibleNotificationsRead, handleRealtimeNotification } =
  await import(
  '../notificationStore'
);

beforeEach(() => {
  apiGet.mockReset();
  apiPatch.mockReset();
  logWarn.mockReset();
  notificationStore.reset();
});

describe('notificationStore', () => {
  it('marks notification read and applies server readAt', async () => {
    notificationStore.setNotifications(
      [
        {
          id: 'notif-1',
          type: 'new_post',
          createdAt: '2026-01-01T00:00:00Z',
          readAt: null,
        },
      ],
      null,
      false,
      1
    );

    apiPatch.mockResolvedValue({
      notification: {
        id: 'notif-1',
        type: 'new_post',
        created_at: '2026-01-01T00:00:00Z',
        read_at: '2026-01-02T00:00:00Z',
      },
    });

    await markNotificationRead('notif-1');

    const state = get(notificationStore);
    expect(state.unreadCount).toBe(0);
    expect(state.notifications[0]?.readAt).toBe('2026-01-02T00:00:00Z');
    expect(apiPatch).toHaveBeenCalledTimes(1);
  });

  it('reverts optimistic read when patch fails', async () => {
    notificationStore.setNotifications(
      [
        {
          id: 'notif-2',
          type: 'new_comment',
          createdAt: '2026-01-01T00:00:00Z',
          readAt: null,
        },
      ],
      null,
      false,
      1
    );

    apiPatch.mockRejectedValue(new Error('nope'));
    apiGet.mockResolvedValue({
      notifications: [
        {
          id: 'notif-2',
          type: 'new_comment',
          created_at: '2026-01-01T00:00:00Z',
          read_at: null,
        },
      ],
      meta: {
        cursor: null,
        has_more: false,
        unread_count: 1,
      },
    });

    await markNotificationRead('notif-2');

    const state = get(notificationStore);
    expect(state.unreadCount).toBe(1);
    expect(state.notifications[0]?.readAt).toBeNull();
    expect(apiGet).toHaveBeenCalledTimes(1);
    expect(logWarn).toHaveBeenCalled();
  });

  it('skips API call when notification already read', async () => {
    notificationStore.setNotifications(
      [
        {
          id: 'notif-3',
          type: 'mention',
          createdAt: '2026-01-01T00:00:00Z',
          readAt: '2026-01-01T01:00:00Z',
        },
      ],
      null,
      false,
      0
    );

    await markNotificationRead('notif-3');

    expect(apiPatch).not.toHaveBeenCalled();
  });

  it('reloads notifications when marking visible ones fails', async () => {
    notificationStore.setNotifications(
      [
        {
          id: 'notif-4',
          type: 'new_post',
          createdAt: '2026-01-01T00:00:00Z',
          readAt: null,
        },
        {
          id: 'notif-5',
          type: 'mention',
          createdAt: '2026-01-01T01:00:00Z',
          readAt: null,
        },
      ],
      'cursor-1',
      true,
      2
    );

    apiPatch
      .mockResolvedValueOnce({
        notification: {
          id: 'notif-4',
          type: 'new_post',
          created_at: '2026-01-01T00:00:00Z',
          read_at: '2026-01-02T00:00:00Z',
        },
      })
      .mockRejectedValueOnce(new Error('nope'));

    apiGet.mockResolvedValue({
      notifications: [
        {
          id: 'notif-4',
          type: 'new_post',
          created_at: '2026-01-01T00:00:00Z',
          read_at: '2026-01-02T00:00:00Z',
        },
        {
          id: 'notif-5',
          type: 'mention',
          created_at: '2026-01-01T01:00:00Z',
          read_at: null,
        },
      ],
      meta: {
        cursor: null,
        has_more: false,
        unread_count: 1,
      },
    });

    await markVisibleNotificationsRead();

    expect(apiPatch).toHaveBeenCalledTimes(2);
    expect(apiGet).toHaveBeenCalledTimes(1);
    expect(logWarn).toHaveBeenCalled();
  });

  it('adds realtime notifications to the store', () => {
    handleRealtimeNotification({
      id: 'notif-10',
      type: 'mention',
      created_at: '2026-01-05T00:00:00Z',
      read_at: null,
    });

    const state = get(notificationStore);
    expect(state.notifications).toHaveLength(1);
    expect(state.notifications[0]?.id).toBe('notif-10');
    expect(state.unreadCount).toBe(1);
  });

  it('reloads notifications when realtime payload is incomplete', async () => {
    storeRefs.isAuthenticated.set(true);
    apiGet.mockResolvedValue({
      notifications: [
        {
          id: 'notif-11',
          type: 'new_comment',
          created_at: '2026-01-06T00:00:00Z',
          read_at: null,
        },
      ],
      meta: {
        cursor: null,
        has_more: false,
        unread_count: 1,
      },
    });

    handleRealtimeNotification({});

    expect(apiGet).toHaveBeenCalledTimes(1);
    await new Promise((resolve) => setTimeout(resolve, 0));
    const state = get(notificationStore);
    expect(state.notifications[0]?.id).toBe('notif-11');
  });
});
