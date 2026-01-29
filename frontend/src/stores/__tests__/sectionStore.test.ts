import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';

const apiGet = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    get: apiGet,
  },
}));

const { sectionStore } = await import('../sectionStore');

beforeEach(() => {
  apiGet.mockReset();
  sectionStore.setSections([]);
});

describe('sectionStore', () => {
  it('loadSections success sets sections and active section', async () => {
    apiGet.mockResolvedValue({
      sections: [
        { id: 'section-1', name: 'Music', type: 'music' },
        { id: 'section-2', name: 'General', type: 'general' },
        { id: 'section-3', name: 'Books', type: 'book' },
      ],
    });

    await sectionStore.loadSections();
    const state = get(sectionStore);

    expect(state.sections).toHaveLength(3);
    expect(state.sections[0].icon).toBe('💬');
    expect(state.sections[1].icon).toBe('🎵');
    expect(state.sections[2].icon).toBe('📚');
    expect(state.activeSection?.id).toBe('section-2');
    expect(state.isLoading).toBe(false);
  });

  it('loadSections failure keeps existing state', async () => {
    sectionStore.setSections([
      { id: 'section-1', name: 'Music', type: 'music', icon: '🎵', slug: 'music' },
    ]);

    apiGet.mockRejectedValue(new Error('fail'));

    await sectionStore.loadSections();
    const state = get(sectionStore);

    expect(state.sections).toHaveLength(1);
    expect(state.sections[0].id).toBe('section-1');
    expect(state.isLoading).toBe(false);
  });

  it('setSections preserves active section when present', () => {
    sectionStore.setSections([
      { id: 'section-1', name: 'Music', type: 'music', icon: '🎵', slug: 'music' },
      { id: 'section-2', name: 'Books', type: 'book', icon: '📚', slug: 'books' },
      { id: 'section-3', name: 'General', type: 'general', icon: '💬', slug: 'general' },
    ]);
    sectionStore.setActiveSection({
      id: 'section-2',
      name: 'Books',
      type: 'book',
      icon: '📚',
      slug: 'books',
    });

    sectionStore.setSections([
      { id: 'section-2', name: 'Books', type: 'book', icon: '📚', slug: 'books' },
      { id: 'section-3', name: 'General', type: 'general', icon: '💬', slug: 'general' },
      { id: 'section-4', name: 'Series', type: 'series', icon: '📺', slug: 'series' },
    ]);

    const state = get(sectionStore);
    expect(state.activeSection?.id).toBe('section-2');
  });

  it('setSections selects first or null when active missing', () => {
    sectionStore.setActiveSection({
      id: 'section-99',
      name: 'Old',
      type: 'general',
      icon: '💬',
      slug: 'old',
    });

    sectionStore.setSections([
      { id: 'section-1', name: 'Music', type: 'music', icon: '🎵', slug: 'music' },
      { id: 'section-2', name: 'General', type: 'general', icon: '💬', slug: 'general' },
    ]);
    let state = get(sectionStore);
    expect(state.activeSection?.id).toBe('section-2');

    sectionStore.setSections([]);
    state = get(sectionStore);
    expect(state.activeSection).toBeNull();
  });

  it('uses fallback icon for unknown type', () => {
    sectionStore.setSections([
      {
        id: 'section-x',
        name: 'Unknown',
        type: 'unknown' as unknown as never,
        icon: '📁',
        slug: 'unknown',
      },
    ]);

    const state = get(sectionStore);
    expect(state.sections[0].icon).toBe('📁');
  });
});
