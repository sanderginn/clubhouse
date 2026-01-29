import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get, writable } from 'svelte/store';

const activeSection = writable<{ id: string; name: string; type: string; icon: string } | null>(null);

vi.mock('./sectionStore', () => ({
  activeSection,
}));

const apiGet = vi.fn();

vi.mock('../services/api', () => ({
  api: {
    get: apiGet,
  },
}));

const { searchStore } = await import('./searchStore');

beforeEach(() => {
  searchStore.clear();
  activeSection.set(null);
  apiGet.mockReset();
});

describe('searchStore', () => {
  it('skips search when query is empty', async () => {
    await searchStore.search();

    expect(apiGet).not.toHaveBeenCalled();
    const state = get(searchStore);
    expect(state.results).toHaveLength(0);
    expect(state.lastSearched).toBe('');
    expect(state.lastSearchedScope).toBeNull();
  });

  it('searches within the active section by default', async () => {
    activeSection.set({ id: 'section-1', name: 'Music', type: 'music', icon: '🎵' });
    searchStore.setQuery('hello');
    apiGet.mockResolvedValue({
      results: [
        {
          type: 'post',
          score: 0.9,
          post: {
            id: 'post-1',
            user_id: 'user-1',
            section_id: 'section-1',
            content: 'hello world',
            created_at: '2024-01-01T00:00:00Z',
            comment_count: 0,
          },
        },
        {
          type: 'comment',
          score: 0.7,
          comment: {
            id: 'comment-1',
            user_id: 'user-2',
            post_id: 'post-1',
            content: 'nice post',
            created_at: '2024-01-01T01:00:00Z',
          },
          post: {
            id: 'post-1',
            user_id: 'user-1',
            section_id: 'section-1',
            content: 'hello world',
            created_at: '2024-01-01T00:00:00Z',
            comment_count: 0,
          },
        },
      ],
    });

    await searchStore.search();

    expect(apiGet).toHaveBeenCalledTimes(1);
    const request = apiGet.mock.calls[0][0] as string;
    expect(request).toContain('/search?');
    expect(request).toContain('scope=section');
    expect(request).toContain('section_id=section-1');

    const state = get(searchStore);
    expect(state.results).toHaveLength(2);
    expect(state.results[0].type).toBe('post');
    expect(state.results[1].type).toBe('comment');
    expect(state.results[1].post?.id).toBe('post-1');
    expect(state.lastSearchedScope).toBe('section');
  });

  it('searches globally when scope is global', async () => {
    searchStore.setQuery('global');
    searchStore.setScope('global');
    apiGet.mockResolvedValue({ results: [] });

    await searchStore.search();

    const request = apiGet.mock.calls[0][0] as string;
    expect(request).toContain('scope=global');
    expect(request).not.toContain('section_id=');

    const state = get(searchStore);
    expect(state.lastSearchedScope).toBe('global');
  });

  it('sets error when searching section scope without active section', async () => {
    searchStore.setQuery('hello');
    searchStore.setScope('section');

    await searchStore.search();

    expect(apiGet).not.toHaveBeenCalled();
    const state = get(searchStore);
    expect(state.error).toBe('Select a section to search within.');
  });

  it('clear resets state', () => {
    searchStore.setQuery('hello');
    searchStore.setScope('global');
    searchStore.clear();

    const state = get(searchStore);
    expect(state.query).toBe('');
    expect(state.scope).toBe('section');
    expect(state.results).toHaveLength(0);
    expect(state.isLoading).toBe(false);
    expect(state.error).toBeNull();
    expect(state.lastSearched).toBe('');
    expect(state.lastSearchedScope).toBeNull();
  });

  it('clears section search when active section changes', async () => {
    activeSection.set({ id: 'section-1', name: 'General', type: 'general', icon: '💬' });
    searchStore.setQuery('hello');
    apiGet.mockResolvedValue({ results: [] });

    await searchStore.search();

    activeSection.set({ id: 'section-2', name: 'Music', type: 'music', icon: '🎵' });

    const state = get(searchStore);
    expect(state.query).toBe('');
    expect(state.results).toHaveLength(0);
    expect(state.lastSearched).toBe('');
    expect(state.lastSearchedScope).toBeNull();
    expect(state.error).toBeNull();
    expect(state.isLoading).toBe(false);
  });

  it('stores errors when the API fails', async () => {
    searchStore.setQuery('fail');
    searchStore.setScope('global');
    apiGet.mockRejectedValue(new Error('boom'));

    await searchStore.search();

    const state = get(searchStore);
    expect(state.error).toBe('boom');
    expect(state.isLoading).toBe(false);
  });
});
