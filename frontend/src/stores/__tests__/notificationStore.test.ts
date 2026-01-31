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

const { notificationStore, markNotificationRead } = await import('../notificationStore');

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

    await markNotificationRead('notif-2');

    const state = get(notificationStore);
    expect(state.unreadCount).toBe(1);
    expect(state.notifications[0]?.readAt).toBeNull();
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
});
