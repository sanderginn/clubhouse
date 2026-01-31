import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';
import { createRequire } from 'module';

const require = createRequire(import.meta.url);
const { writable } = require('svelte/store') as typeof import('svelte/store');

const storeRefs: {
  isAuthenticated: ReturnType<typeof writable>;
  notificationStore: ReturnType<typeof writable>;
  displayTimezone: ReturnType<typeof writable>;
  loadNotifications: ReturnType<typeof vi.fn>;
  loadMoreNotifications: ReturnType<typeof vi.fn>;
  markNotificationRead: ReturnType<typeof vi.fn>;
  markVisibleNotificationsRead: ReturnType<typeof vi.fn>;
} = {} as any;

const routeRefs = {
  buildStandaloneThreadHref: vi.fn(),
  pushPath: vi.fn(),
};

vi.mock('../../../stores', () => {
  storeRefs.isAuthenticated = writable(true);
  storeRefs.notificationStore = writable({
    notifications: [],
    isLoading: false,
    error: null,
    paginationError: null,
    cursor: null,
    hasMore: false,
    unreadCount: 0,
  });
  storeRefs.displayTimezone = writable(null);
  storeRefs.loadNotifications = vi.fn();
  storeRefs.loadMoreNotifications = vi.fn();
  storeRefs.markNotificationRead = vi.fn();
  storeRefs.markVisibleNotificationsRead = vi.fn();

  return storeRefs;
});

vi.mock('../../../services/routeNavigation', () => ({
  buildStandaloneThreadHref: (...args: any[]) => routeRefs.buildStandaloneThreadHref(...args),
  pushPath: (...args: any[]) => routeRefs.pushPath(...args),
}));

const { default: NotificationMenu } = await import('../NotificationMenu.svelte');

const baseState = {
  notifications: [],
  isLoading: false,
  error: null,
  paginationError: null,
  cursor: null,
  hasMore: false,
  unreadCount: 0,
};

beforeEach(() => {
  storeRefs.isAuthenticated.set(true);
  storeRefs.notificationStore.set({ ...baseState });
  storeRefs.displayTimezone.set(null);
  storeRefs.loadNotifications.mockReset();
  storeRefs.loadMoreNotifications.mockReset();
  storeRefs.markNotificationRead.mockReset();
  storeRefs.markVisibleNotificationsRead.mockReset();
  routeRefs.buildStandaloneThreadHref.mockReset();
  routeRefs.pushPath.mockReset();
});

afterEach(() => {
  cleanup();
});

describe('NotificationMenu', () => {
  it('loads notifications when opening with empty state', async () => {
    render(NotificationMenu);

    const toggle = screen.getByLabelText('Toggle notifications');
    await fireEvent.click(toggle);

    expect(storeRefs.loadNotifications).toHaveBeenCalledTimes(1);
    expect(storeRefs.markVisibleNotificationsRead).not.toHaveBeenCalled();
  });

  it('marks visible notifications when opening with items', async () => {
    storeRefs.notificationStore.set({
      ...baseState,
      notifications: [
        {
          id: 'notif-1',
          type: 'new_post',
          createdAt: '2026-01-01T00:00:00Z',
          readAt: null,
        },
      ],
      unreadCount: 1,
    });

    render(NotificationMenu);

    const toggle = screen.getByLabelText('Toggle notifications');
    await fireEvent.click(toggle);
    await tick();

    expect(storeRefs.markVisibleNotificationsRead).toHaveBeenCalledTimes(1);
  });

  it('marks a notification read and navigates on click', async () => {
    storeRefs.notificationStore.set({
      ...baseState,
      notifications: [
        {
          id: 'notif-2',
          type: 'new_comment',
          relatedPostId: 'post-1',
          relatedCommentId: 'comment-1',
          createdAt: '2026-01-01T00:00:00Z',
          readAt: null,
        },
      ],
      unreadCount: 1,
    });

    routeRefs.buildStandaloneThreadHref.mockReturnValue('/posts/post-1');

    render(NotificationMenu);

    const toggle = screen.getByLabelText('Toggle notifications');
    await fireEvent.click(toggle);
    await tick();

    storeRefs.markVisibleNotificationsRead.mockClear();

    const notificationButton = screen.getByText('Someone commented on your post');
    await fireEvent.click(notificationButton);

    expect(storeRefs.markNotificationRead).toHaveBeenCalledWith('notif-2');
    expect(routeRefs.pushPath).toHaveBeenCalledWith('/posts/post-1?comment=comment-1');
  });

  it('calls mark all when action clicked', async () => {
    storeRefs.notificationStore.set({
      ...baseState,
      notifications: [
        {
          id: 'notif-3',
          type: 'mention',
          createdAt: '2026-01-01T00:00:00Z',
          readAt: null,
        },
      ],
      unreadCount: 1,
    });

    render(NotificationMenu);

    const toggle = screen.getByLabelText('Toggle notifications');
    await fireEvent.click(toggle);
    await tick();

    storeRefs.markVisibleNotificationsRead.mockClear();

    const markAll = screen.getByText('Mark all as read');
    await fireEvent.click(markAll);

    expect(storeRefs.markVisibleNotificationsRead).toHaveBeenCalledTimes(1);
  });
});
