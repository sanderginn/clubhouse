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
  sections: ReturnType<typeof writable>;
  sectionStore: { setActiveSection: ReturnType<typeof vi.fn> };
  searchStore: { setQuery: ReturnType<typeof vi.fn> };
  postStore: { upsertPost: ReturnType<typeof vi.fn> };
  uiStore: { setActiveView: ReturnType<typeof vi.fn> };
  threadRouteStore: { setTarget: ReturnType<typeof vi.fn> };
} = {} as any;

vi.mock('../../../stores', () => {
  storeRefs.searchResults = writable([]);
  storeRefs.isSearching = writable(false);
  storeRefs.searchError = writable<string | null>(null);
  storeRefs.searchQuery = writable('');
  storeRefs.lastSearchQuery = writable('');
  storeRefs.searchScope = writable<'section' | 'global'>('section');
  storeRefs.activeSection = writable<{ id: string; name: string } | null>(null);
  storeRefs.sections = writable([]);
  storeRefs.sectionStore = { setActiveSection: vi.fn() };
  storeRefs.searchStore = { setQuery: vi.fn() };
  storeRefs.postStore = { upsertPost: vi.fn() };
  storeRefs.uiStore = { setActiveView: vi.fn() };
  storeRefs.threadRouteStore = { setTarget: vi.fn() };

  return storeRefs;
});

vi.mock('../../PostCard.svelte', async () => ({
  default: (await import('./PostCardMock.svelte')).default,
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
  storeRefs.sections.set([]);
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
    storeRefs.searchQuery.set('nice');
    storeRefs.lastSearchQuery.set('nice');
    storeRefs.sections.set([
      { id: 'section-1', name: 'Music', type: 'music', icon: '🎵', slug: 'music' },
    ]);
    storeRefs.activeSection.set({ id: 'section-1', name: 'Music', slug: 'music' });
    storeRefs.searchResults.set([
      {
        type: 'comment',
        score: 1,
        comment: {
          id: 'comment-1',
          postId: 'post-1',
          sectionId: 'section-1',
          content: 'Nice post',
          createdAt: '2025-01-01T00:00:00Z',
          user: { id: 'user-1', username: 'Sander' },
        },
      },
    ]);

    render(SearchResults);
    const highlight = screen.getByText('Nice');
    expect(highlight).toBeInTheDocument();
    expect(highlight.tagName).toBe('MARK');
    expect(screen.getByText(/post/i)).toBeInTheDocument();
    const sectionLabels = screen.getAllByText('Music');
    expect(sectionLabels.length).toBeGreaterThan(0);
  });

  it('renders section label for post results', () => {
    storeRefs.searchQuery.set('hello');
    storeRefs.lastSearchQuery.set('hello');
    storeRefs.sections.set([
      { id: 'section-2', name: 'Movies', type: 'movie', icon: '🎬', slug: 'movies' },
    ]);
    storeRefs.activeSection.set({ id: 'section-2', name: 'Movies', slug: 'movies' });
    storeRefs.searchResults.set([
      {
        type: 'post',
        score: 1,
        post: {
          id: 'post-1',
          sectionId: 'section-2',
          content: 'New post',
          createdAt: '2025-01-02T00:00:00Z',
          userId: 'user-1',
        },
      },
    ]);

    render(SearchResults);
    expect(screen.getAllByText('Movies').length).toBeGreaterThan(0);
  });

  it('groups global search results by section', () => {
    storeRefs.searchQuery.set('mix');
    storeRefs.lastSearchQuery.set('mix');
    storeRefs.searchScope.set('global');
    storeRefs.sections.set([
      { id: 'section-1', name: 'Music', type: 'music', icon: '🎵', slug: 'music' },
      { id: 'section-2', name: 'Movies', type: 'movie', icon: '🎬', slug: 'movies' },
    ]);
    storeRefs.searchResults.set([
      {
        type: 'post',
        score: 2,
        post: {
          id: 'post-1',
          sectionId: 'section-1',
          content: 'Music post',
          createdAt: '2025-01-02T00:00:00Z',
          userId: 'user-1',
        },
      },
      {
        type: 'comment',
        score: 1,
        comment: {
          id: 'comment-1',
          postId: 'post-1',
          sectionId: 'section-1',
          content: 'Great tune',
          createdAt: '2025-01-03T00:00:00Z',
          user: { id: 'user-2', username: 'Alex' },
        },
        post: {
          id: 'post-1',
          sectionId: 'section-1',
          content: 'Music post',
          createdAt: '2025-01-02T00:00:00Z',
          userId: 'user-1',
        },
      },
      {
        type: 'post',
        score: 1,
        post: {
          id: 'post-2',
          sectionId: 'section-2',
          content: 'Movie post',
          createdAt: '2025-01-04T00:00:00Z',
          userId: 'user-3',
        },
      },
    ]);

    render(SearchResults);
    expect(screen.getByText('Music')).toBeInTheDocument();
    expect(screen.getByText('Movies')).toBeInTheDocument();
    expect(screen.getByText('post-1')).toBeInTheDocument();
    expect(screen.getByText('post-2')).toBeInTheDocument();
    expect(screen.getByText('Parent post')).toBeInTheDocument();
  });
});
