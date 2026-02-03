import { writable, derived } from 'svelte/store';
import { activeSection } from './sectionStore';
import { posts, type Link, type LinkMetadata } from './postStore';

export type MusicLengthFilter = 'all' | 'tracks' | 'sets';

const STORAGE_KEY = 'music-length-filter';
export const TRACK_MAX_DURATION_SECONDS = 15 * 60;

function isValidFilter(value: string | null): value is MusicLengthFilter {
  return value === 'all' || value === 'tracks' || value === 'sets';
}

function getInitialFilter(): MusicLengthFilter {
  if (typeof window === 'undefined') {
    return 'all';
  }

  const stored = window.sessionStorage?.getItem(STORAGE_KEY) ?? null;
  return isValidFilter(stored) ? stored : 'all';
}

export function extractDurationSeconds(metadata?: LinkMetadata | null): number | null {
  const duration = metadata?.duration;
  if (typeof duration !== 'number' || !Number.isFinite(duration) || duration <= 0) {
    return null;
  }
  return duration;
}

export function matchesMusicLengthFilter(
  durationSeconds: number | null,
  filter: MusicLengthFilter
): boolean {
  if (filter === 'all') {
    return true;
  }
  if (durationSeconds === null) {
    return false;
  }
  if (filter === 'tracks') {
    return durationSeconds < TRACK_MAX_DURATION_SECONDS;
  }
  return durationSeconds >= TRACK_MAX_DURATION_SECONDS;
}

function getDurationFromLinks(links?: Link[]): number | null {
  if (!links || links.length === 0) {
    return null;
  }

  for (const link of links) {
    const duration = extractDurationSeconds(link.metadata);
    if (duration !== null) {
      return duration;
    }
  }

  return null;
}

export const musicLengthFilter = writable<MusicLengthFilter>(getInitialFilter());

if (typeof window !== 'undefined') {
  musicLengthFilter.subscribe((value) => {
    window.sessionStorage?.setItem(STORAGE_KEY, value);
  });
}

export const filteredPosts = derived(
  [posts, activeSection, musicLengthFilter],
  ([$posts, $activeSection, $musicLengthFilter]) => {
    if ($activeSection?.type !== 'music' || $musicLengthFilter === 'all') {
      return $posts;
    }

    return $posts.filter((post) =>
      matchesMusicLengthFilter(getDurationFromLinks(post.links), $musicLengthFilter)
    );
  }
);
