import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import {
  extractDurationSeconds,
  matchesMusicLengthFilter,
  musicLengthFilter,
  filteredPosts,
  TRACK_MAX_DURATION_SECONDS,
} from '../musicFilterStore';
import { postStore } from '../postStore';
import { sectionStore } from '../sectionStore';

const basePost = {
  id: 'post-1',
  userId: 'user-1',
  sectionId: 'section-1',
  content: 'hello',
  createdAt: '2026-01-29T00:00:00Z',
};

beforeEach(() => {
  window.sessionStorage?.clear();
  musicLengthFilter.set('all');
  postStore.reset();
  sectionStore.setActiveSection(null);
});

describe('musicFilterStore', () => {
  it('extracts valid duration seconds', () => {
    expect(extractDurationSeconds({ duration: 0, url: 'a' })).toBeNull();
    expect(extractDurationSeconds({ duration: -10, url: 'a' })).toBeNull();
    expect(extractDurationSeconds({ duration: Number.NaN, url: 'a' })).toBeNull();
    expect(extractDurationSeconds({ duration: 120, url: 'a' })).toBe(120);
  });

  it('matches length filters', () => {
    const threshold = TRACK_MAX_DURATION_SECONDS;
    expect(matchesMusicLengthFilter(null, 'tracks')).toBe(false);
    expect(matchesMusicLengthFilter(null, 'sets')).toBe(false);
    expect(matchesMusicLengthFilter(threshold - 1, 'tracks')).toBe(true);
    expect(matchesMusicLengthFilter(threshold - 1, 'sets')).toBe(false);
    expect(matchesMusicLengthFilter(threshold, 'tracks')).toBe(false);
    expect(matchesMusicLengthFilter(threshold, 'sets')).toBe(true);
  });

  it('filters posts by duration when in music section', () => {
    sectionStore.setActiveSection({
      id: 'section-1',
      name: 'Music',
      type: 'music',
      icon: '🎵',
      slug: 'music',
    });

    postStore.setPosts(
      [
        {
          ...basePost,
          id: 'track-1',
          links: [{ url: 'https://example.com/track', metadata: { url: 'https://example.com/track', duration: 120 } }],
        },
        {
          ...basePost,
          id: 'set-1',
          links: [{ url: 'https://example.com/set', metadata: { url: 'https://example.com/set', duration: TRACK_MAX_DURATION_SECONDS } }],
        },
        {
          ...basePost,
          id: 'unknown-1',
          links: [{ url: 'https://example.com/unknown', metadata: { url: 'https://example.com/unknown' } }],
        },
      ],
      null,
      false
    );

    musicLengthFilter.set('tracks');
    expect(get(filteredPosts).map((post) => post.id)).toEqual(['track-1']);

    musicLengthFilter.set('sets');
    expect(get(filteredPosts).map((post) => post.id)).toEqual(['set-1']);
  });

  it('does not filter posts outside the music section', () => {
    sectionStore.setActiveSection({
      id: 'section-1',
      name: 'General',
      type: 'general',
      icon: '💬',
      slug: 'general',
    });

    postStore.setPosts(
      [
        {
          ...basePost,
          id: 'post-1',
          links: [{ url: 'https://example.com/track', metadata: { url: 'https://example.com/track', duration: 120 } }],
        },
        {
          ...basePost,
          id: 'post-2',
          links: [{ url: 'https://example.com/set', metadata: { url: 'https://example.com/set', duration: TRACK_MAX_DURATION_SECONDS } }],
        },
      ],
      null,
      false
    );

    musicLengthFilter.set('tracks');
    expect(get(filteredPosts).map((post) => post.id)).toEqual(['post-1', 'post-2']);
  });
});
