import { describe, it, expect } from 'vitest';

import {
  isYouTubeUrl,
  isSpotifyUrl,
  isSoundCloudUrl,
  isBandcampUrl,
  parseYouTubeUrl
} from '../urlParsers';

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
    expect(isYouTubeUrl('https://spotify.com/track/abc')).toBe(false);
  });

  it('returns false for invalid URLs', () => {
    expect(isYouTubeUrl('not a url')).toBe(false);
    expect(isYouTubeUrl('')).toBe(false);
  });
});

describe('isSpotifyUrl', () => {
  it('returns true for open.spotify.com URLs', () => {
    expect(isSpotifyUrl('https://open.spotify.com/track/abc')).toBe(true);
  });

  it('returns false for non-Spotify URLs', () => {
    expect(isSpotifyUrl('https://youtube.com/watch?v=abc')).toBe(false);
  });

  it('returns false for invalid URLs', () => {
    expect(isSpotifyUrl('not a url')).toBe(false);
  });
});

describe('isSoundCloudUrl', () => {
  it('returns true for soundcloud.com URLs', () => {
    expect(isSoundCloudUrl('https://soundcloud.com/artist/track')).toBe(true);
  });

  it('returns false for non-SoundCloud URLs', () => {
    expect(isSoundCloudUrl('https://youtube.com/watch?v=abc')).toBe(false);
  });

  it('returns false for invalid URLs', () => {
    expect(isSoundCloudUrl('not a url')).toBe(false);
  });
});

describe('isBandcampUrl', () => {
  it('returns true for bandcamp.com URLs', () => {
    expect(isBandcampUrl('https://artist.bandcamp.com/album/title')).toBe(true);
  });

  it('returns false for non-Bandcamp URLs', () => {
    expect(isBandcampUrl('https://youtube.com/watch?v=abc')).toBe(false);
  });

  it('returns false for invalid URLs', () => {
    expect(isBandcampUrl('not a url')).toBe(false);
  });
});

describe('parseYouTubeUrl', () => {
  it('parses standard watch URLs', () => {
    expect(parseYouTubeUrl('https://www.youtube.com/watch?v=dQw4w9WgXcQ')).toBe(
      'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ'
    );
  });

  it('parses short URLs', () => {
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

  it('parses legacy v/ URLs', () => {
    expect(parseYouTubeUrl('https://www.youtube.com/v/dQw4w9WgXcQ')).toBe(
      'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ'
    );
  });

  it('handles URLs with extra parameters', () => {
    expect(parseYouTubeUrl('https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=120&list=PLxyz')).toBe(
      'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ'
    );
  });

  it('handles URLs without www', () => {
    expect(parseYouTubeUrl('https://youtube.com/watch?v=dQw4w9WgXcQ')).toBe(
      'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ'
    );
  });

  it('returns null for invalid YouTube URLs', () => {
    expect(parseYouTubeUrl('https://youtube.com/channel/abc')).toBeNull();
    expect(parseYouTubeUrl('https://youtube.com/')).toBeNull();
  });

  it('returns null for non-YouTube URLs', () => {
    expect(parseYouTubeUrl('https://vimeo.com/123456')).toBeNull();
    expect(parseYouTubeUrl('not a url')).toBeNull();
  });
});
