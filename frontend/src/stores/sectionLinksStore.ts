import { writable, derived } from 'svelte/store';
import type { LinkMetadata } from './postStore';

export interface SectionLink {
  id: string;
  url: string;
  metadata?: LinkMetadata;
  postId: string;
  userId: string;
  username: string;
  createdAt: string;
}

export interface SectionLinksState {
  links: SectionLink[];
  isLoading: boolean;
  error: string | null;
  cursor: string | null;
  hasMore: boolean;
  sectionId: string | null;
}

const initialState: SectionLinksState = {
  links: [],
  isLoading: false,
  error: null,
  cursor: null,
  hasMore: true,
  sectionId: null,
};

function createSectionLinksStore() {
  const { subscribe, set, update } = writable<SectionLinksState>({ ...initialState });

  return {
    subscribe,
    setLinks: (
      links: SectionLink[],
      cursor: string | null,
      hasMore: boolean,
      sectionId: string | null
    ) =>
      update((state) => ({
        ...state,
        links,
        cursor,
        hasMore,
        sectionId,
        isLoading: false,
        error: null,
      })),
    appendLinks: (links: SectionLink[], cursor: string | null, hasMore: boolean) =>
      update((state) => {
        const seen = new Set(state.links.map((link) => link.id));
        const unique = links.filter((link) => {
          if (!link.id) return true;
          if (seen.has(link.id)) return false;
          seen.add(link.id);
          return true;
        });
        return {
          ...state,
          links: [...state.links, ...unique],
          cursor,
          hasMore,
          isLoading: false,
          error: null,
        };
      }),
    setLoading: (isLoading: boolean) =>
      update((state) => ({
        ...state,
        isLoading,
        error: isLoading ? null : state.error,
      })),
    setError: (error: string | null) =>
      update((state) => ({
        ...state,
        error,
        isLoading: false,
      })),
    reset: () => set({ ...initialState }),
  };
}

export const sectionLinksStore = createSectionLinksStore();

export const sectionLinks = derived(sectionLinksStore, ($store) => $store.links);
export const isLoadingSectionLinks = derived(sectionLinksStore, ($store) => $store.isLoading);
export const hasMoreSectionLinks = derived(sectionLinksStore, ($store) => $store.hasMore);
