import { uiStore } from '../stores/uiStore';
import { threadRouteStore } from '../stores/threadRouteStore';
import { buildSettingsHref, pushPath } from './routeNavigation';

export function openSettings(): void {
  uiStore.setActiveView('settings');
  threadRouteStore.clearTarget();
  pushPath(buildSettingsHref());
}

export function handleSettingsNavigation(event: MouseEvent): void {
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
  openSettings();
}
