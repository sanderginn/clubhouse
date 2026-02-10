const SECTION_PATH_PREFIX = '/sections/';
const THREAD_ROOT_PATH = '/posts/';
const THREAD_PATH_SEGMENT = '/posts/';
const ADMIN_PATH = '/admin';
const SETTINGS_PATH = '/settings';
const WATCHLIST_PATH = '/watchlist';
const BOOKSHELF_PATH = '/bookshelf';
const WATCHLIST_SEGMENT = WATCHLIST_PATH.slice(1);

export type SearchHistoryState = {
  query: string;
  scope: 'section' | 'global';
};

export type AppHistoryState = {
  search?: SearchHistoryState;
  fromSearch?: boolean;
};

export function buildSectionHref(sectionSlug: string): string {
  return `${SECTION_PATH_PREFIX}${encodeURIComponent(sectionSlug)}`;
}

export function buildThreadHref(sectionSlug: string, postId: string): string {
  return `${buildSectionHref(sectionSlug)}${THREAD_PATH_SEGMENT}${postId}`;
}

export function buildStandaloneThreadHref(postId: string): string {
  return `${THREAD_ROOT_PATH}${postId}`;
}

export function buildSectionWatchlistHref(sectionSlug: string): string {
  return `${buildSectionHref(sectionSlug)}/${WATCHLIST_SEGMENT}`;
}

export function parseSectionSlug(pathname: string): string | null {
  if (!pathname.startsWith(SECTION_PATH_PREFIX)) {
    return null;
  }
  const slug = pathname.slice(SECTION_PATH_PREFIX.length).split('/')[0]?.trim();
  if (!slug) {
    return null;
  }
  try {
    return decodeURIComponent(slug);
  } catch {
    return slug;
  }
}

export function parseThreadPostId(pathname: string): string | null {
  if (!pathname.startsWith(SECTION_PATH_PREFIX)) {
    return null;
  }
  const remainder = pathname.slice(SECTION_PATH_PREFIX.length);
  const [sectionSlug, segment, postId] = remainder.split('/');
  if (!sectionSlug || segment !== THREAD_PATH_SEGMENT.slice(1, -1)) {
    return null;
  }
  const trimmed = postId?.trim();
  return trimmed ? trimmed : null;
}

export function parseStandaloneThreadPostId(pathname: string): string | null {
  if (!pathname.startsWith(THREAD_ROOT_PATH)) {
    return null;
  }
  const remainder = pathname.slice(THREAD_ROOT_PATH.length);
  const [postId] = remainder.split('/');
  const trimmed = postId?.trim();
  return trimmed ? trimmed : null;
}

export function parseSectionWatchlistSlug(pathname: string): string | null {
  if (!pathname.startsWith(SECTION_PATH_PREFIX)) {
    return null;
  }
  const remainder = pathname.slice(SECTION_PATH_PREFIX.length);
  const [sectionSlug, segment] = remainder.split('/');
  if (!sectionSlug || segment !== WATCHLIST_SEGMENT) {
    return null;
  }
  try {
    return decodeURIComponent(sectionSlug);
  } catch {
    return sectionSlug;
  }
}

export function parseThreadCommentId(search: string): string | null {
  if (!search) return null;
  const params = new URLSearchParams(search);
  return params.get('comment') ?? params.get('commentId') ?? null;
}

export function buildAdminHref(): string {
  return ADMIN_PATH;
}

export function buildSettingsHref(): string {
  return SETTINGS_PATH;
}

export function buildWatchlistHref(): string {
  return WATCHLIST_PATH;
}

export function buildBookshelfHref(): string {
  return BOOKSHELF_PATH;
}

export function isAdminPath(pathname: string): boolean {
  return pathname === ADMIN_PATH || pathname.startsWith(`${ADMIN_PATH}/`);
}

export function isSettingsPath(pathname: string): boolean {
  return pathname === SETTINGS_PATH || pathname.startsWith(`${SETTINGS_PATH}/`);
}

export function isWatchlistPath(pathname: string): boolean {
  return pathname === WATCHLIST_PATH || pathname.startsWith(`${WATCHLIST_PATH}/`);
}

export function isBookshelfPath(pathname: string): boolean {
  return pathname === BOOKSHELF_PATH || pathname.startsWith(`${BOOKSHELF_PATH}/`);
}

export function buildFeedHref(sectionSlug?: string | null): string {
  return sectionSlug ? buildSectionHref(sectionSlug) : '/';
}

export function pushPath(path: string, state?: AppHistoryState | null): void {
  if (typeof window === 'undefined') return;
  window.history.pushState(state ?? null, '', path);
}

export function replacePath(path: string, state?: AppHistoryState | null): void {
  if (typeof window === 'undefined') return;
  window.history.replaceState(state ?? null, '', path);
}

export function getHistoryState(): AppHistoryState | null {
  if (typeof window === 'undefined') return null;
  return (window.history.state as AppHistoryState | null) ?? null;
}

export function updateHistoryState(nextState: Partial<AppHistoryState>): void {
  if (typeof window === 'undefined') return;
  const current = (window.history.state ?? {}) as AppHistoryState;
  const merged: AppHistoryState = { ...current, ...nextState };
  if ('search' in nextState && nextState.search === undefined) {
    delete merged.search;
  }
  if ('fromSearch' in nextState && nextState.fromSearch === undefined) {
    delete merged.fromSearch;
  }
  window.history.replaceState(
    merged,
    '',
    window.location.pathname + window.location.search
  );
}
