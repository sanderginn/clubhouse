import { writable } from 'svelte/store';

export type FatalErrorSource = 'window' | 'unhandledrejection';

export interface FatalErrorInfo {
  message: string;
  error?: unknown;
  source: FatalErrorSource;
  timestamp: Date;
}

const fatalErrorStore = writable<FatalErrorInfo | null>(null);

export const fatalError = {
  subscribe: fatalErrorStore.subscribe,
};

export function setFatalError(error: FatalErrorInfo): void {
  fatalErrorStore.set(error);
}

export function clearFatalError(): void {
  fatalErrorStore.set(null);
}
