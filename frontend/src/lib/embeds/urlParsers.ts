/**
 * URL detection and parsing functions for embed providers.
 */

export function isYouTubeUrl(url: string): boolean {
  try {
    const parsed = new URL(url);
    const host = parsed.hostname.replace(/^www\./, '');
    return host === 'youtube.com' || host === 'youtu.be';
  } catch {
    return false;
  }
}

export function isSpotifyUrl(url: string): boolean {
  try {
    const parsed = new URL(url);
    const host = parsed.hostname.replace(/^www\./, '');
    return host === 'open.spotify.com';
  } catch {
    return false;
  }
}

export function isSoundCloudUrl(url: string): boolean {
  try {
    const parsed = new URL(url);
    const host = parsed.hostname.replace(/^www\./, '');
    return host === 'soundcloud.com';
  } catch {
    return false;
  }
}

export function isBandcampUrl(url: string): boolean {
  try {
    const parsed = new URL(url);
    return parsed.hostname.endsWith('.bandcamp.com');
  } catch {
    return false;
  }
}

type SpotifyContentType = 'track' | 'album' | 'playlist' | 'artist' | 'show' | 'episode';

const SPOTIFY_HEIGHTS: Record<SpotifyContentType, number> = {
  track: 152,
  album: 380,
  playlist: 380,
  artist: 380,
  show: 232,
  episode: 232
};

export function parseSpotifyUrl(url: string): { embedUrl: string; height: number } | null {
  if (!isSpotifyUrl(url)) {
    return null;
  }

  try {
    const parsed = new URL(url);
    const pathParts = parsed.pathname.split('/').filter(Boolean);

    if (pathParts.length < 2) {
      return null;
    }

    const type = pathParts[0] as SpotifyContentType;
    const id = pathParts[1];

    if (!(type in SPOTIFY_HEIGHTS)) {
      return null;
    }

    return {
      embedUrl: `https://open.spotify.com/embed/${type}/${id}`,
      height: SPOTIFY_HEIGHTS[type]
    };
  } catch {
    return null;
  }
}

export function parseYouTubeUrl(url: string): string | null {
  if (!isYouTubeUrl(url)) {
    return null;
  }

  try {
    const parsed = new URL(url);
    const host = parsed.hostname.replace(/^www\./, '');
    let videoId: string | null = null;

    if (host === 'youtu.be') {
      // Short URL: https://youtu.be/VIDEO_ID
      videoId = parsed.pathname.slice(1);
    } else if (host === 'youtube.com') {
      const path = parsed.pathname;

      if (path.startsWith('/watch')) {
        // Standard: /watch?v=VIDEO_ID
        videoId = parsed.searchParams.get('v');
      } else if (path.startsWith('/embed/')) {
        // Embed: /embed/VIDEO_ID
        videoId = path.replace('/embed/', '').split('/')[0];
      } else if (path.startsWith('/shorts/')) {
        // Shorts: /shorts/VIDEO_ID
        videoId = path.replace('/shorts/', '').split('/')[0];
      } else if (path.startsWith('/v/')) {
        // Legacy: /v/VIDEO_ID
        videoId = path.replace('/v/', '').split('/')[0];
      }
    }

    if (videoId && videoId.length > 0) {
      return `https://www.youtube-nocookie.com/embed/${videoId}`;
    }

    return null;
  } catch {
    return null;
  }
}
