import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/svelte';
import { createRequire } from 'module';

const require = createRequire(import.meta.url);
const { writable } = require('svelte/store') as typeof import('svelte/store');

const storeRefs: {
  searchResults: ReturnType<typeof writable>;
  isSearching: ReturnType<typeof writable>;
  searchError: ReturnType<typeof writable>;
  searchQuery: ReturnType<typeof writable>;
  lastSearchQuery: ReturnType<typeof writable>;
  searchScope: ReturnType<typeof writable>;
  activeSection: ReturnType<typeof writable>;
} = {} as any;

vi.mock('../../../stores', () => {
  storeRefs.searchResults = writable([]);
  storeRefs.isSearching = writable(false);
  storeRefs.searchError = writable<string | null>(null);
  storeRefs.searchQuery = writable('');
  storeRefs.lastSearchQuery = writable('');
  storeRefs.searchScope = writable<'section' | 'global'>('section');
  storeRefs.activeSection = writable<{ id: string; name: string } | null>(null);

  return storeRefs;
});

vi.mock('../../PostCard.svelte', () => ({
  default: { $$render: () => '<div data-testid="post-card"></div>' },
}));

const { default: SearchResults } = await import('../SearchResults.svelte');

beforeEach(() => {
  storeRefs.searchResults.set([]);
  storeRefs.isSearching.set(false);
  storeRefs.searchError.set(null);
  storeRefs.searchQuery.set('');
  storeRefs.lastSearchQuery.set('');
  storeRefs.searchScope.set('section');
  storeRefs.activeSection.set(null);
});

afterEach(() => {
  cleanup();
});

describe('SearchResults', () => {
  it('shows empty state when no query', () => {
    render(SearchResults);
    expect(screen.getByText('Start typing to search posts and comments.')).toBeInTheDocument();
  });

  it('shows loading state', () => {
    storeRefs.searchQuery.set('hello');
    storeRefs.lastSearchQuery.set('hello');
    storeRefs.isSearching.set(true);
    render(SearchResults);
    expect(screen.getByText('Searching...')).toBeInTheDocument();
  });

  it('shows error state', () => {
    storeRefs.searchQuery.set('hello');
    storeRefs.lastSearchQuery.set('hello');
    storeRefs.searchError.set('boom');
    render(SearchResults);
    expect(screen.getByText('boom')).toBeInTheDocument();
  });

  it('shows no results state', () => {
    storeRefs.searchQuery.set('hello');
    storeRefs.lastSearchQuery.set('hello');
    storeRefs.searchResults.set([]);
    render(SearchResults);
    expect(screen.getByText('No results for "hello".')).toBeInTheDocument();
  });

  it('hides results when query changes', () => {
    storeRefs.searchQuery.set('new');
    storeRefs.lastSearchQuery.set('old');
    render(SearchResults);
    expect(screen.getByText('Press Search to see results.')).toBeInTheDocument();
  });

  it('renders comment results', () => {
    storeRefs.searchQuery.set('hello');
    storeRefs.lastSearchQuery.set('hello');
    storeRefs.searchResults.set([
      {
        type: 'comment',
        score: 1,
        comment: {
          id: 'comment-1',
          postId: 'post-1',
          content: 'Nice post',
          createdAt: '2025-01-01T00:00:00Z',
          user: { id: 'user-1', username: 'Sander' },
        },
      },
    ]);

    render(SearchResults);
    expect(screen.getByText('Nice post')).toBeInTheDocument();
  });
});
