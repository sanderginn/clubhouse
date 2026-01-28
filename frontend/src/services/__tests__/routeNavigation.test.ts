import { describe, expect, it } from 'vitest';
import {
  buildAdminHref,
  buildFeedHref,
  buildSectionHref,
  isAdminPath,
  parseSectionId,
} from '../routeNavigation';

describe('routeNavigation', () => {
  it('builds section hrefs', () => {
    expect(buildSectionHref('section-1')).toBe('/sections/section-1');
  });

  it('parses section ids from section paths', () => {
    expect(parseSectionId('/sections/section-1')).toBe('section-1');
    expect(parseSectionId('/sections/section-1/extra')).toBe('section-1');
    expect(parseSectionId('/sections/')).toBeNull();
    expect(parseSectionId('/admin')).toBeNull();
  });

  it('builds admin hrefs and recognizes admin paths', () => {
    expect(buildAdminHref()).toBe('/admin');
    expect(isAdminPath('/admin')).toBe(true);
    expect(isAdminPath('/admin/tools')).toBe(true);
    expect(isAdminPath('/sections/section-1')).toBe(false);
  });

  it('builds feed hrefs from section ids', () => {
    expect(buildFeedHref('section-1')).toBe('/sections/section-1');
    expect(buildFeedHref(null)).toBe('/');
    expect(buildFeedHref(undefined)).toBe('/');
  });
});
