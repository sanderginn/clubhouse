import { writable, derived } from 'svelte/store';
import { api } from '../services/api';
import { logWarn } from '../lib/observability/logger';

interface ConfigResponse {
  config?: {
    displayTimezone?: string;
    display_timezone?: string;
  };
}

interface ConfigState {
  displayTimezone: string | null;
  loaded: boolean;
}

const normalizeTimezone = (response: ConfigResponse | null): string | null => {
  const config = response?.config;
  if (!config) return null;
  if (typeof config.displayTimezone === 'string' && config.displayTimezone.trim() !== '') {
    return config.displayTimezone.trim();
  }
  if (typeof config.display_timezone === 'string' && config.display_timezone.trim() !== '') {
    return config.display_timezone.trim();
  }
  return null;
};

function createConfigStore() {
  const { subscribe, update } = writable<ConfigState>({
    displayTimezone: null,
    loaded: false,
  });

  let loadPromise: Promise<void> | null = null;

  const load = async (): Promise<void> => {
    if (loadPromise) return loadPromise;

    loadPromise = (async () => {
      try {
        const response = await api.get<ConfigResponse | null>('/config');
        const displayTimezone = normalizeTimezone(response);
        update((state) => ({
          ...state,
          displayTimezone,
          loaded: true,
        }));
      } catch (error) {
        logWarn('Failed to load public config', { error });
        update((state) => ({
          ...state,
          loaded: true,
        }));
      }
    })().finally(() => {
      loadPromise = null;
    });

    return loadPromise;
  };

  const setDisplayTimezone = (displayTimezone: string | null) => {
    update((state) => ({
      ...state,
      displayTimezone,
      loaded: true,
    }));
  };

  return {
    subscribe,
    load,
    setDisplayTimezone,
  };
}

export const configStore = createConfigStore();

export const displayTimezone = derived(configStore, ($config) => $config.displayTimezone);
export const configLoaded = derived(configStore, ($config) => $config.loaded);
