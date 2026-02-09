import { describe, expect, it } from 'vitest';
import {
  buildAdminHref,
  buildFeedHref,
  buildSectionHref,
  buildStandaloneThreadHref,
  buildSettingsHref,
  buildWatchlistHref,
  buildThreadHref,
  isAdminPath,
  isSettingsPath,
  isWatchlistPath,
  parseStandaloneThreadPostId,
  parseSectionSlug,
  parseThreadCommentId,
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

  it('builds and parses standalone thread hrefs', () => {
    expect(buildStandaloneThreadHref('post-99')).toBe('/posts/post-99');
    expect(parseStandaloneThreadPostId('/posts/post-99')).toBe('post-99');
    expect(parseStandaloneThreadPostId('/posts/post-99/comments')).toBe('post-99');
    expect(parseStandaloneThreadPostId('/sections/section-1/posts/post-1')).toBeNull();
  });

  it('parses comment highlight params', () => {
    expect(parseThreadCommentId('?comment=comment-1')).toBe('comment-1');
    expect(parseThreadCommentId('?commentId=comment-2')).toBe('comment-2');
    expect(parseThreadCommentId('?foo=bar')).toBeNull();
    expect(parseThreadCommentId('')).toBeNull();
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

  it('builds watchlist hrefs and recognizes watchlist paths', () => {
    expect(buildWatchlistHref()).toBe('/watchlist');
    expect(isWatchlistPath('/watchlist')).toBe(true);
    expect(isWatchlistPath('/watchlist/favorites')).toBe(true);
    expect(isWatchlistPath('/sections/movies')).toBe(false);
  });

  it('builds feed hrefs from section slugs', () => {
    expect(buildFeedHref('music')).toBe('/sections/music');
    expect(buildFeedHref(null)).toBe('/');
    expect(buildFeedHref(undefined)).toBe('/');
  });
});
