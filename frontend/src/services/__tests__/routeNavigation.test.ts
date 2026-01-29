import { describe, expect, it } from 'vitest';
import {
  buildAdminHref,
  buildFeedHref,
  buildSectionHref,
  buildSettingsHref,
  buildThreadHref,
  isAdminPath,
  isSettingsPath,
  parseSectionSlug,
  parseThreadPostId,
} from '../routeNavigation';

describe('routeNavigation', () => {
  it('builds section hrefs', () => {
    expect(buildSectionHref('section-1')).toBe('/sections/section-1');
  });

  it('parses section slugs from section paths', () => {
    expect(parseSectionSlug('/sections/music')).toBe('music');
    expect(parseSectionSlug('/sections/music/extra')).toBe('music');
    expect(parseSectionSlug('/sections/music/posts/post-1')).toBe('music');
    expect(parseSectionSlug('/sections/')).toBeNull();
    expect(parseSectionSlug('/admin')).toBeNull();
  });

  it('builds and parses thread hrefs', () => {
    expect(buildThreadHref('section-1', 'post-1')).toBe('/sections/section-1/posts/post-1');
    expect(parseThreadPostId('/sections/section-1/posts/post-1')).toBe('post-1');
    expect(parseThreadPostId('/sections/section-1')).toBeNull();
    expect(parseThreadPostId('/sections/section-1/posts/')).toBeNull();
    expect(parseThreadPostId('/admin')).toBeNull();
  });

  it('builds admin hrefs and recognizes admin paths', () => {
    expect(buildAdminHref()).toBe('/admin');
    expect(isAdminPath('/admin')).toBe(true);
    expect(isAdminPath('/admin/tools')).toBe(true);
    expect(isAdminPath('/sections/section-1')).toBe(false);
  });

  it('builds settings hrefs and recognizes settings paths', () => {
    expect(buildSettingsHref()).toBe('/settings');
    expect(isSettingsPath('/settings')).toBe(true);
    expect(isSettingsPath('/settings/profile')).toBe(true);
    expect(isSettingsPath('/admin')).toBe(false);
  });

  it('builds feed hrefs from section slugs', () => {
    expect(buildFeedHref('music')).toBe('/sections/music');
    expect(buildFeedHref(null)).toBe('/');
    expect(buildFeedHref(undefined)).toBe('/');
  });
});
