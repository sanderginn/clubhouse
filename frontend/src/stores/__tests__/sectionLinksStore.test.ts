import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import {
  sectionLinksStore,
  sectionLinks,
  isLoadingSectionLinks,
  hasMoreSectionLinks,
} from '../sectionLinksStore';

const makeLink = (id: string) => ({
  id,
  url: `https://example.com/${id}`,
  postId: `post-${id}`,
  userId: `user-${id}`,
  username: `user-${id}`,
  createdAt: '2026-02-02T00:00:00Z',
});

beforeEach(() => {
  sectionLinksStore.reset();
});

describe('sectionLinksStore', () => {
  it('setLinks sets links, cursor, section, and clears loading/error', () => {
    sectionLinksStore.setLoading(true);
    sectionLinksStore.setLinks([makeLink('1')], 'cursor-1', true, 'section-1');

    const state = get(sectionLinksStore);
    expect(state.links).toHaveLength(1);
    expect(state.cursor).toBe('cursor-1');
    expect(state.hasMore).toBe(true);
    expect(state.sectionId).toBe('section-1');
    expect(state.isLoading).toBe(false);
    expect(state.error).toBeNull();
  });

  it('appendLinks adds new links without duplicating ids', () => {
    sectionLinksStore.setLinks([makeLink('1')], 'cursor-1', true, 'section-1');
    sectionLinksStore.appendLinks([makeLink('1'), makeLink('2')], 'cursor-2', false);

    const state = get(sectionLinksStore);
    expect(state.links).toHaveLength(2);
    expect(state.links.map((link) => link.id)).toEqual(['1', '2']);
    expect(state.cursor).toBe('cursor-2');
    expect(state.hasMore).toBe(false);
  });

  it('setLoading clears error when loading', () => {
    sectionLinksStore.setError('boom');
    sectionLinksStore.setLoading(true);

    const state = get(sectionLinksStore);
    expect(state.isLoading).toBe(true);
    expect(state.error).toBeNull();
  });

  it('setError stops loading', () => {
    sectionLinksStore.setLoading(true);
    sectionLinksStore.setError('failed');

    const state = get(sectionLinksStore);
    expect(state.isLoading).toBe(false);
    expect(state.error).toBe('failed');
  });

  it('reset returns to initial state', () => {
    sectionLinksStore.setLinks([makeLink('1')], 'cursor-1', false, 'section-1');
    sectionLinksStore.reset();

    const state = get(sectionLinksStore);
    expect(state.links).toEqual([]);
    expect(state.cursor).toBeNull();
    expect(state.hasMore).toBe(true);
    expect(state.sectionId).toBeNull();
  });

  it('derived stores expose state slices', () => {
    sectionLinksStore.setLinks([makeLink('1')], null, true, 'section-1');
    sectionLinksStore.setLoading(true);

    expect(get(sectionLinks)).toHaveLength(1);
    expect(get(isLoadingSectionLinks)).toBe(true);
    expect(get(hasMoreSectionLinks)).toBe(true);
  });
});
