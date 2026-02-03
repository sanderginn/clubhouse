import { describe, it, expect, afterEach } from 'vitest';
import { render, cleanup } from '@testing-library/svelte';

const { default: SpotifyEmbed } = await import('../SpotifyEmbed.svelte');

describe('SpotifyEmbed', () => {
  afterEach(() => {
    cleanup();
  });

  it('uses provided height when available', () => {
    const { container } = render(SpotifyEmbed, {
      embedUrl: 'https://open.spotify.com/embed/track/xyz',
      height: 232,
    });

    const iframe = container.querySelector('iframe');
    expect(iframe).not.toBeNull();
    expect(iframe?.getAttribute('style')).toContain('232px');
  });

  it('infers height from embed url', () => {
    const { container, getByTitle } = render(SpotifyEmbed, {
      embedUrl: 'https://open.spotify.com/embed/track/xyz',
    });

    expect(getByTitle('Spotify track')).toBeInTheDocument();
    const iframe = container.querySelector('iframe');
    expect(iframe?.getAttribute('style')).toContain('152px');
  });
});
