const SECTION_PATH_PREFIX = '/sections/';
const THREAD_PATH_SEGMENT = '/posts/';
const ADMIN_PATH = '/admin';
const SETTINGS_PATH = '/settings';

export function buildSectionHref(sectionSlug: string): string {
  return `${SECTION_PATH_PREFIX}${encodeURIComponent(sectionSlug)}`;
}

export function buildThreadHref(sectionSlug: string, postId: string): string {
  return `${buildSectionHref(sectionSlug)}${THREAD_PATH_SEGMENT}${postId}`;
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

export function buildAdminHref(): string {
  return ADMIN_PATH;
}

export function buildSettingsHref(): string {
  return SETTINGS_PATH;
}

export function isAdminPath(pathname: string): boolean {
  return pathname === ADMIN_PATH || pathname.startsWith(`${ADMIN_PATH}/`);
}

export function isSettingsPath(pathname: string): boolean {
  return pathname === SETTINGS_PATH || pathname.startsWith(`${SETTINGS_PATH}/`);
}

export function buildFeedHref(sectionSlug?: string | null): string {
  return sectionSlug ? buildSectionHref(sectionSlug) : '/';
}

export function pushPath(path: string): void {
  if (typeof window === 'undefined') return;
  window.history.pushState(null, '', path);
}

export function replacePath(path: string): void {
  if (typeof window === 'undefined') return;
  window.history.replaceState(null, '', path);
}
