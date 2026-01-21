import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@testing-library/svelte';
import { createRequire } from 'module';
import type { Writable } from 'svelte/store';

const require = createRequire(import.meta.url);
const { writable } = require('svelte/store') as typeof import('svelte/store');

interface SearchState {
  query: string;
  scope: string;
}

const storeRefs: {
  activeSection: Writable<{ id: string; name: string } | null>;
  state: Writable<SearchState>;
  searchStore: {
    subscribe: Writable<SearchState>['subscribe'];
    setQuery: Mock;
    setScope: Mock;
    search: Mock;
    clear: Mock;
  };
} = {} as typeof storeRefs;

vi.mock('../../../stores', () => {
  storeRefs.activeSection = writable<{ id: string; name: string } | null>({
    id: 'section-1',
    name: 'Music',
  });
  storeRefs.state = writable<SearchState>({
    query: '',
    scope: 'section',
  });
  storeRefs.searchStore = {
    subscribe: storeRefs.state.subscribe,
    setQuery: vi.fn((query: string) => storeRefs.state.update((s) => ({ ...s, query }))),
    setScope: vi.fn((scope: string) => storeRefs.state.update((s) => ({ ...s, scope }))),
    search: vi.fn(),
    clear: vi.fn(() => storeRefs.state.set({ query: '', scope: 'section' })),
  };

  return {
    activeSection: storeRefs.activeSection,
    searchStore: storeRefs.searchStore,
  };
});

const { default: SearchBar } = await import('../SearchBar.svelte');

beforeEach(() => {
  storeRefs.searchStore.setQuery.mockClear();
  storeRefs.searchStore.setScope.mockClear();
  storeRefs.searchStore.search.mockClear();
  storeRefs.searchStore.clear.mockClear();
  storeRefs.state.set({ query: '', scope: 'section' });
});

afterEach(() => {
  cleanup();
});

describe('SearchBar', () => {
  it('updates query on input', async () => {
    render(SearchBar);

    const input = screen.getByPlaceholderText('Search posts and comments...');
    await fireEvent.input(input, { target: { value: 'hello' } });

    expect(storeRefs.searchStore.setQuery).toHaveBeenCalledWith('hello');
  });

  it('changes scope', async () => {
    render(SearchBar);

    const select = screen.getByRole('combobox');
    await fireEvent.change(select, { target: { value: 'global' } });

    expect(storeRefs.searchStore.setScope).toHaveBeenCalledWith('global');
  });

  it('submits search', async () => {
    const { container } = render(SearchBar);

    const form = container.querySelector('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);

    expect(storeRefs.searchStore.search).toHaveBeenCalled();
  });

  it('clears query', async () => {
    storeRefs.state.set({ query: 'hello', scope: 'section' });
    render(SearchBar);

    const clearButton = screen.getByText('Clear');
    await fireEvent.click(clearButton);

    expect(storeRefs.searchStore.clear).toHaveBeenCalled();
  });
});
