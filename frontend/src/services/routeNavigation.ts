const SECTION_PATH_PREFIX = '/sections/';
const THREAD_PATH_SEGMENT = '/posts/';
const ADMIN_PATH = '/admin';

export function buildSectionHref(sectionId: string): string {
  return `${SECTION_PATH_PREFIX}${sectionId}`;
}

export function buildThreadHref(sectionId: string, postId: string): string {
  return `${buildSectionHref(sectionId)}${THREAD_PATH_SEGMENT}${postId}`;
}

export function parseSectionId(pathname: string): string | null {
  if (!pathname.startsWith(SECTION_PATH_PREFIX)) {
    return null;
  }
  const id = pathname.slice(SECTION_PATH_PREFIX.length).split('/')[0]?.trim();
  return id ? id : null;
}

export function parseThreadPostId(pathname: string): string | null {
  if (!pathname.startsWith(SECTION_PATH_PREFIX)) {
    return null;
  }
  const remainder = pathname.slice(SECTION_PATH_PREFIX.length);
  const [sectionId, segment, postId] = remainder.split('/');
  if (!sectionId || segment !== THREAD_PATH_SEGMENT.slice(1, -1)) {
    return null;
  }
  const trimmed = postId?.trim();
  return trimmed ? trimmed : null;
}

export function buildAdminHref(): string {
  return ADMIN_PATH;
}

export function isAdminPath(pathname: string): boolean {
  return pathname === ADMIN_PATH || pathname.startsWith(`${ADMIN_PATH}/`);
}

export function buildFeedHref(sectionId?: string | null): string {
  return sectionId ? buildSectionHref(sectionId) : '/';
}

export function pushPath(path: string): void {
  if (typeof window === 'undefined') return;
  window.history.pushState(null, '', path);
}

export function replacePath(path: string): void {
  if (typeof window === 'undefined') return;
  window.history.replaceState(null, '', path);
}
