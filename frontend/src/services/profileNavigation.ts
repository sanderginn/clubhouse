import { get } from 'svelte/store';
import { activeSection } from '../stores/sectionStore';
import { uiStore } from '../stores/uiStore';
import { buildFeedHref, pushPath } from './routeNavigation';

const PROFILE_PATH_PREFIX = '/users/';

export function buildProfileHref(userId: string): string {
  return `${PROFILE_PATH_PREFIX}${userId}`;
}

export function openUserProfile(userId: string): void {
  if (!userId) return;
  uiStore.openProfile(userId);
  pushPath(buildProfileHref(userId));
}

export function returnToFeed(): void {
  uiStore.setActiveView('feed');
  const sectionId = get(activeSection)?.id ?? null;
  pushPath(buildFeedHref(sectionId));
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
