import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  isYouTubeUrl,
  isSpotifyUrl,
  isSoundCloudUrl,
  isBandcampUrl,
  parseYouTubeUrl,
  parseSpotifyUrl,
  fetchSoundCloudEmbed
} from './urlParsers';

describe('URL detection functions', () => {
  describe('isYouTubeUrl', () => {
    it('returns true for youtube.com URLs', () => {
      expect(isYouTubeUrl('https://www.youtube.com/watch?v=abc123')).toBe(true);
      expect(isYouTubeUrl('https://youtube.com/watch?v=abc123')).toBe(true);
    });

    it('returns true for youtu.be URLs', () => {
      expect(isYouTubeUrl('https://youtu.be/abc123')).toBe(true);
    });

    it('returns false for non-YouTube URLs', () => {
      expect(isYouTubeUrl('https://vimeo.com/123')).toBe(false);
      expect(isYouTubeUrl('https://open.spotify.com/track/abc')).toBe(false);
    });

    it('returns false for invalid URLs', () => {
      expect(isYouTubeUrl('not a url')).toBe(false);
    });
  });

  describe('isSpotifyUrl', () => {
    it('returns true for Spotify URLs', () => {
      expect(isSpotifyUrl('https://open.spotify.com/track/abc123')).toBe(true);
      expect(isSpotifyUrl('https://open.spotify.com/album/abc123')).toBe(true);
    });

    it('returns false for non-Spotify URLs', () => {
      expect(isSpotifyUrl('https://youtube.com/watch?v=abc')).toBe(false);
    });

    it('returns false for invalid URLs', () => {
      expect(isSpotifyUrl('not a url')).toBe(false);
    });
  });

  describe('isSoundCloudUrl', () => {
    it('returns true for SoundCloud URLs', () => {
      expect(isSoundCloudUrl('https://soundcloud.com/artist/track')).toBe(true);
    });

    it('returns false for non-SoundCloud URLs', () => {
      expect(isSoundCloudUrl('https://youtube.com/watch?v=abc')).toBe(false);
    });
  });

  describe('isBandcampUrl', () => {
    it('returns true for Bandcamp URLs', () => {
      expect(isBandcampUrl('https://artist.bandcamp.com/album/name')).toBe(true);
    });

    it('returns false for non-Bandcamp URLs', () => {
      expect(isBandcampUrl('https://youtube.com/watch?v=abc')).toBe(false);
    });
  });
});

describe('parseYouTubeUrl', () => {
  it('parses standard watch URLs', () => {
    expect(parseYouTubeUrl('https://www.youtube.com/watch?v=dQw4w9WgXcQ')).toBe(
      'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ'
    );
  });

  it('parses short youtu.be URLs', () => {
    expect(parseYouTubeUrl('https://youtu.be/dQw4w9WgXcQ')).toBe(
      'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ'
    );
  });

  it('parses embed URLs', () => {
    expect(parseYouTubeUrl('https://www.youtube.com/embed/dQw4w9WgXcQ')).toBe(
      'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ'
    );
  });

  it('parses shorts URLs', () => {
    expect(parseYouTubeUrl('https://www.youtube.com/shorts/dQw4w9WgXcQ')).toBe(
      'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ'
    );
  });

  it('returns null for non-YouTube URLs', () => {
    expect(parseYouTubeUrl('https://vimeo.com/123')).toBeNull();
  });

  it('returns null for invalid URLs', () => {
    expect(parseYouTubeUrl('not a url')).toBeNull();
  });
});

describe('parseSpotifyUrl', () => {
  describe('track URLs', () => {
    it('parses track URLs', () => {
      const result = parseSpotifyUrl('https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh');
      expect(result).toEqual({
        embedUrl: 'https://open.spotify.com/embed/track/4iV5W9uYEdYUVa79Axb7Rh',
        height: 152
      });
    });

    it('handles track URLs with query params', () => {
      const result = parseSpotifyUrl(
        'https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh?si=abc123'
      );
      expect(result).toEqual({
        embedUrl: 'https://open.spotify.com/embed/track/4iV5W9uYEdYUVa79Axb7Rh',
        height: 152
      });
    });
  });

  describe('album URLs', () => {
    it('parses album URLs', () => {
      const result = parseSpotifyUrl('https://open.spotify.com/album/1DFixLWuPkv3KT3TnV35m3');
      expect(result).toEqual({
        embedUrl: 'https://open.spotify.com/embed/album/1DFixLWuPkv3KT3TnV35m3',
        height: 380
      });
    });
  });

  describe('playlist URLs', () => {
    it('parses playlist URLs', () => {
      const result = parseSpotifyUrl('https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M');
      expect(result).toEqual({
        embedUrl: 'https://open.spotify.com/embed/playlist/37i9dQZF1DXcBWIGoYBM5M',
        height: 380
      });
    });
  });

  describe('artist URLs', () => {
    it('parses artist URLs', () => {
      const result = parseSpotifyUrl('https://open.spotify.com/artist/0OdUWJ0sBjDrqHygGUXeCF');
      expect(result).toEqual({
        embedUrl: 'https://open.spotify.com/embed/artist/0OdUWJ0sBjDrqHygGUXeCF',
        height: 380
      });
    });
  });

  describe('podcast URLs', () => {
    it('parses show URLs', () => {
      const result = parseSpotifyUrl('https://open.spotify.com/show/4rOoJ6Egrf8K2IrywzwOMk');
      expect(result).toEqual({
        embedUrl: 'https://open.spotify.com/embed/show/4rOoJ6Egrf8K2IrywzwOMk',
        height: 232
      });
    });

    it('parses episode URLs', () => {
      const result = parseSpotifyUrl('https://open.spotify.com/episode/512ojhOuo1ktJprKbVcKyQ');
      expect(result).toEqual({
        embedUrl: 'https://open.spotify.com/embed/episode/512ojhOuo1ktJprKbVcKyQ',
        height: 232
      });
    });
  });

  describe('invalid URLs', () => {
    it('returns null for non-Spotify URLs', () => {
      expect(parseSpotifyUrl('https://youtube.com/watch?v=abc')).toBeNull();
    });

    it('returns null for Spotify URLs without type/id', () => {
      expect(parseSpotifyUrl('https://open.spotify.com/')).toBeNull();
      expect(parseSpotifyUrl('https://open.spotify.com/search')).toBeNull();
    });

    it('returns null for unsupported Spotify URL types', () => {
      expect(parseSpotifyUrl('https://open.spotify.com/user/abc123')).toBeNull();
      expect(parseSpotifyUrl('https://open.spotify.com/genre/rock')).toBeNull();
    });

    it('returns null for malformed URLs', () => {
      expect(parseSpotifyUrl('not a url')).toBeNull();
    });
  });
});

describe('fetchSoundCloudEmbed', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches and parses oEmbed response for track URL', async () => {
    const mockResponse = {
      height: 166,
      html: '<iframe width="100%" height="166" scrolling="no" frameborder="no" src="https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/tracks/123456&color=%23ff5500"></iframe>'
    };

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse)
    });

    const result = await fetchSoundCloudEmbed('https://soundcloud.com/artist/track-name');

    expect(fetch).toHaveBeenCalledWith(
      expect.stringContaining('soundcloud.com/oembed'),
      expect.any(Object)
    );
    expect(result).toEqual({
      embedUrl:
        'https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/tracks/123456&color=%23ff5500',
      height: 166
    });
  });

  it('fetches and parses oEmbed response for playlist URL', async () => {
    const mockResponse = {
      height: 450,
      html: '<iframe width="100%" height="450" scrolling="no" frameborder="no" src="https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/playlists/789"></iframe>'
    };

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse)
    });

    const result = await fetchSoundCloudEmbed('https://soundcloud.com/artist/sets/playlist-name');

    expect(result).toEqual({
      embedUrl: 'https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/playlists/789',
      height: 450
    });
  });

  it('returns null for non-200 response', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 404
    });

    const result = await fetchSoundCloudEmbed('https://soundcloud.com/nonexistent/track');

    expect(result).toBeNull();
  });

  it('returns null when HTML is missing from response', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ height: 166 })
    });

    const result = await fetchSoundCloudEmbed('https://soundcloud.com/artist/track');

    expect(result).toBeNull();
  });

  it('returns null when iframe src cannot be extracted', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          height: 166,
          html: '<div>No iframe here</div>'
        })
    });

    const result = await fetchSoundCloudEmbed('https://soundcloud.com/artist/track');

    expect(result).toBeNull();
  });

  it('returns null on network error', async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error('Network error'));

    const result = await fetchSoundCloudEmbed('https://soundcloud.com/artist/track');

    expect(result).toBeNull();
  });
});
