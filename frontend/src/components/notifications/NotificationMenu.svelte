<script lang="ts">
  import {
    isAuthenticated,
    notificationStore,
    loadNotifications,
    loadMoreNotifications,
    markNotificationRead,
    markAllNotificationsRead,
  } from '../../stores';
  import type { Notification } from '../../stores';
  import { buildStandaloneThreadHref, pushPath } from '../../services/routeNavigation';
  import RelativeTime from '../RelativeTime.svelte';

  let menuOpen = false;

  const typeLabels: Record<string, string> = {
    new_post: 'New post',
    new_comment: 'New comment',
    mention: 'Mention',
    reaction: 'Reaction',
  };

  function toggleMenu() {
    menuOpen = !menuOpen;
    if (menuOpen) {
      if ($notificationStore.notifications.length === 0 && !$notificationStore.isLoading) {
        loadNotifications();
      }
    }
  }

  function closeMenu() {
    menuOpen = false;
  }


  function notificationTitle(notification: Notification): string {
    const actor = notification.relatedUser?.username
      ? `@${notification.relatedUser.username}`
      : 'Someone';

    switch (notification.type) {
      case 'new_post':
        return `${actor} shared a new post`;
      case 'new_comment':
        return `${actor} commented on your post`;
      case 'mention':
        return `${actor} mentioned you`;
      case 'reaction':
        return `${actor} reacted to your ${notification.relatedCommentId ? 'comment' : 'post'}`;
      default:
        return typeLabels[notification.type] ?? 'Notification';
    }
  }

  function notificationIcon(notification: Notification): string {
    switch (notification.type) {
      case 'new_post':
        return '📝';
      case 'new_comment':
        return '💬';
      case 'mention':
        return '🔔';
      case 'reaction':
        return '❤️';
      default:
        return '🔔';
    }
  }

  function buildNotificationHref(notification: Notification): string | null {
    if (!notification.relatedPostId) {
      return null;
    }
    const base = buildStandaloneThreadHref(notification.relatedPostId);
    if (notification.relatedCommentId) {
      return `${base}?comment=${encodeURIComponent(notification.relatedCommentId)}`;
    }
    return base;
  }

  function handleNotificationClick(notification: Notification) {
    if (!notification.readAt) {
      markNotificationRead(notification.id);
    }
    const href = buildNotificationHref(notification);
    if (href) {
      pushPath(href);
      if (typeof window !== 'undefined') {
        window.dispatchEvent(new PopStateEvent('popstate', { state: window.history.state }));
      }
    }
    closeMenu();
  }

  function handleMarkAll() {
    markAllNotificationsRead();
  }

</script>

<svelte:window on:click={closeMenu} />

{#if $isAuthenticated}
  <div class="relative">
    <button
      class="relative flex items-center justify-center p-2 rounded-lg text-gray-600 hover:bg-gray-100"
      aria-label="Toggle notifications"
      aria-haspopup="true"
      aria-expanded={menuOpen}
      on:click|stopPropagation={toggleMenu}
      type="button"
    >
      <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V4a2 2 0 10-4 0v1.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0a3 3 0 11-6 0h6z"
        />
      </svg>
      {#if $notificationStore.unreadCount > 0}
        <span
          class="absolute -top-1 -right-1 min-w-[1.25rem] px-1 rounded-full bg-red-500 text-white text-xs font-semibold leading-5 text-center"
        >
          {$notificationStore.unreadCount > 99 ? '99+' : $notificationStore.unreadCount}
        </span>
      {/if}
    </button>

    {#if menuOpen}
      <div class="absolute right-0 mt-2 w-80 max-w-[90vw] rounded-lg border border-gray-200 bg-white shadow-lg z-50">
        <div class="flex items-center justify-between px-4 py-3 border-b border-gray-100">
          <div>
            <p class="text-sm font-semibold text-gray-900">Notifications</p>
            <p class="text-xs text-gray-500">
              {$notificationStore.unreadCount} unread
            </p>
          </div>
          <button
            class="text-xs font-medium text-primary disabled:text-gray-400"
            on:click|stopPropagation={handleMarkAll}
            type="button"
            disabled={$notificationStore.unreadCount === 0}
          >
            Mark all as read
          </button>
        </div>

        <div class="max-h-96 overflow-y-auto">
          {#if $notificationStore.isLoading}
            <div class="px-4 py-6 text-sm text-gray-500">Loading notifications...</div>
          {:else if $notificationStore.notifications.length === 0}
            <div class="px-4 py-6 text-sm text-gray-500">You're all caught up.</div>
          {:else}
            {#each $notificationStore.notifications as notification (notification.id)}
              <button
                class={`w-full text-left px-4 py-3 border-b border-gray-100 hover:bg-gray-50 ${
                  notification.readAt ? '' : 'bg-blue-50'
                }`}
                on:click={() => handleNotificationClick(notification)}
                type="button"
              >
                <div class="flex items-start gap-3">
                  <span class="mt-1 text-lg" aria-hidden="true">
                    {notificationIcon(notification)}
                  </span>
                  <div class="flex-1">
                    <div class="flex items-center justify-between gap-2">
                      <div class="flex items-center gap-2">
                        {#if !notification.readAt}
                          <span class="h-2 w-2 rounded-full bg-blue-500" aria-label="Unread"></span>
                        {/if}
                        <p class="text-sm text-gray-900">{notificationTitle(notification)}</p>
                      </div>
                      <RelativeTime
                        dateString={notification.createdAt}
                        className="text-xs text-gray-400"
                      />
                    </div>
                    {#if notification.contentExcerpt}
                      <p class="mt-1 text-xs text-gray-500">{notification.contentExcerpt}</p>
                    {/if}
                  </div>
                </div>
              </button>
            {/each}
          {/if}
        </div>

        {#if $notificationStore.paginationError}
          <div class="px-4 py-2 text-xs text-red-600">{$notificationStore.paginationError}</div>
        {/if}

        {#if $notificationStore.hasMore}
          <button
            class="w-full px-4 py-2 text-sm font-medium text-primary hover:bg-gray-50"
            on:click|stopPropagation={loadMoreNotifications}
            type="button"
          >
            Load more
          </button>
        {/if}
      </div>
    {/if}
  </div>
{/if}
