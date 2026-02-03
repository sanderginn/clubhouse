import { get } from 'svelte/store';
import { api } from '../services/api';
import { sectionLinksStore } from './sectionLinksStore';

const LINKS_PAGE_SIZE = 20;

export async function loadSectionLinks(sectionId: string): Promise<void> {
  sectionLinksStore.setLoading(true);

  try {
    const response = await api.getSectionLinks(sectionId, LINKS_PAGE_SIZE);
    sectionLinksStore.setLinks(response.links, response.nextCursor, response.hasMore, sectionId);
  } catch (err) {
    sectionLinksStore.setError(
      err instanceof Error ? err.message : 'Failed to load section links'
    );
  }
}

export async function loadMoreSectionLinks(): Promise<void> {
  const state = get(sectionLinksStore);
  if (state.isLoading || !state.hasMore || !state.cursor || !state.sectionId) {
    return;
  }

  sectionLinksStore.setLoading(true);

  try {
    const response = await api.getSectionLinks(state.sectionId, LINKS_PAGE_SIZE, state.cursor);
    sectionLinksStore.appendLinks(response.links, response.nextCursor, response.hasMore);
  } catch (err) {
    sectionLinksStore.setError(
      err instanceof Error ? err.message : 'Failed to load more links'
    );
  }
}
