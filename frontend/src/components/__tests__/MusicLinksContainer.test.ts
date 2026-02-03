import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { sectionStore } from '../../stores/sectionStore';
import { sectionLinksStore, type SectionLink } from '../../stores/sectionLinksStore';

const loadSectionLinks = vi.hoisted(() => vi.fn());
const loadMoreSectionLinks = vi.hoisted(() => vi.fn());

vi.mock('../../stores/sectionLinksFeedStore', () => ({
  loadSectionLinks,
  loadMoreSectionLinks,
}));

const { default: MusicLinksContainer } = await import('../MusicLinksContainer.svelte');

const baseLink: SectionLink = {
  id: 'link-1',
  url: 'https://example.com',
  metadata: {
    url: 'https://example.com',
    title: 'Example Song',
    provider: 'Example',
  },
  postId: 'post-1',
  userId: 'user-1',
  username: 'sander',
  createdAt: '2026-01-29T08:00:00Z',
};

function setActiveSection(type: 'music' | 'general') {
  sectionStore.setActiveSection({
    id: 'section-1',
    name: type === 'music' ? 'Music' : 'General',
    type,
    icon: type === 'music' ? '🎵' : '💬',
    slug: type === 'music' ? 'music' : 'general',
  });
}

beforeEach(() => {
  loadSectionLinks.mockReset();
  loadMoreSectionLinks.mockReset();
  sectionLinksStore.reset();
  sectionStore.setActiveSection(null);
});

afterEach(() => {
  cleanup();
});

describe('MusicLinksContainer', () => {
  it('does not render outside the music section', () => {
    setActiveSection('general');
    render(MusicLinksContainer);

    expect(screen.queryByText('Recent Music Links')).not.toBeInTheDocument();
    expect(loadSectionLinks).not.toHaveBeenCalled();
  });

  it('shows music links with a count badge', () => {
    setActiveSection('music');
    sectionLinksStore.setLinks([baseLink, { ...baseLink, id: 'link-2', url: 'https://song.two' }], null, false, 'section-1');

    render(MusicLinksContainer);

    expect(screen.getByText('Recent Music Links')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getAllByText('Example Song')).toHaveLength(2);
    expect(screen.getAllByText('@sander')).toHaveLength(2);
  });

  it('collapses the list when the header is clicked', async () => {
    setActiveSection('music');
    sectionLinksStore.setLinks([baseLink], null, false, 'section-1');

    render(MusicLinksContainer);

    const toggle = screen.getByRole('button', { name: /Recent Music Links/i });
    await fireEvent.click(toggle);

    expect(screen.queryByText('Example Song')).not.toBeInTheDocument();
  });

  it('calls loadMoreSectionLinks when load more is clicked', async () => {
    setActiveSection('music');
    sectionLinksStore.setLinks([baseLink], 'cursor-1', true, 'section-1');

    render(MusicLinksContainer);

    const button = screen.getByRole('button', { name: 'Load more' });
    await fireEvent.click(button);

    expect(loadMoreSectionLinks).toHaveBeenCalledTimes(1);
  });
});
