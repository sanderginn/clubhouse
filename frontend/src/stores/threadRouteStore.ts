import { get, writable } from 'svelte/store';
import { api } from '../services/api';
import { postStore } from './postStore';

export type ThreadRouteStatus = 'idle' | 'loading' | 'ready' | 'not_found' | 'error';

interface ThreadRouteState {
  postId: string | null;
  sectionId: string | null;
  status: ThreadRouteStatus;
  error: string | null;
}

function createThreadRouteStore() {
  const { subscribe, set, update } = writable<ThreadRouteState>({
    postId: null,
    sectionId: null,
    status: 'idle',
    error: null,
  });

  return {
    subscribe,
    setTarget: (postId: string, sectionId: string) =>
      set({ postId, sectionId, status: 'idle', error: null }),
    clearTarget: () => set({ postId: null, sectionId: null, status: 'idle', error: null }),
    setLoading: () => update((state) => ({ ...state, status: 'loading', error: null })),
    setReady: () => update((state) => ({ ...state, status: 'ready', error: null })),
    setNotFound: () => update((state) => ({ ...state, status: 'not_found', error: null })),
    setError: (error: string) => update((state) => ({ ...state, status: 'error', error })),
  };
}

export const threadRouteStore = createThreadRouteStore();

export async function loadThreadTargetPost(postId: string, sectionId: string): Promise<void> {
  threadRouteStore.setLoading();
  try {
    const response = await api.getPost(postId);
    const current = get(threadRouteStore);
    if (current.postId !== postId || current.sectionId !== sectionId) {
      return;
    }
    if (!response.post || response.post.sectionId !== sectionId) {
      threadRouteStore.setNotFound();
      return;
    }
    postStore.upsertPost(response.post);
    threadRouteStore.setReady();
  } catch (error) {
    const typedError = error as Error & { code?: string };
    const current = get(threadRouteStore);
    if (current.postId !== postId || current.sectionId !== sectionId) {
      return;
    }
    if (typedError?.code === 'NOT_FOUND') {
      threadRouteStore.setNotFound();
      return;
    }
    threadRouteStore.setError(
      typedError instanceof Error ? typedError.message : 'Unable to load this thread.'
    );
  }
}
