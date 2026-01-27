import { uiStore } from '../stores/uiStore';

const PROFILE_PATH_PREFIX = '/users/';

export function buildProfileHref(userId: string): string {
  return `${PROFILE_PATH_PREFIX}${userId}`;
}

export function openUserProfile(userId: string): void {
  if (!userId) return;
  uiStore.openProfile(userId);
  if (typeof window !== 'undefined') {
    window.history.pushState(null, '', buildProfileHref(userId));
  }
}

export function returnToFeed(): void {
  uiStore.setActiveView('feed');
  if (typeof window !== 'undefined') {
    window.history.pushState(null, '', '/');
  }
}

export function handleProfileNavigation(event: MouseEvent, userId?: string | null): void {
  if (!userId) return;
  if (
    event.defaultPrevented ||
    event.button !== 0 ||
    event.metaKey ||
    event.ctrlKey ||
    event.shiftKey ||
    event.altKey
  ) {
    return;
  }
  event.preventDefault();
  openUserProfile(userId);
}

export function parseProfileUserId(pathname: string): string | null {
  if (!pathname.startsWith(PROFILE_PATH_PREFIX)) {
    return null;
  }
  const id = pathname.slice(PROFILE_PATH_PREFIX.length).split('/')[0]?.trim();
  return id ? id : null;
}
