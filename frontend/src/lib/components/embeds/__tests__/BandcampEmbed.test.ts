import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/svelte';

const { default: BandcampEmbed } = await import('../BandcampEmbed.svelte');

afterEach(() => {
  cleanup();
});

describe('BandcampEmbed', () => {
  it('renders iframe with expected height and link', () => {
    render(BandcampEmbed, {
      embed: {
        embedUrl: 'https://bandcamp.com/EmbeddedPlayer/album=123',
        height: 470,
      },
      linkUrl: 'https://artist.bandcamp.com/album/test',
      title: 'Test Album',
    });

    const iframe = screen.getByTitle('Test Album on Bandcamp');
    expect(iframe).toHaveAttribute(
      'src',
      'https://bandcamp.com/EmbeddedPlayer/album=123'
    );
    expect(iframe).toHaveStyle('height: 470px;');
    expect(screen.getByText('https://artist.bandcamp.com/album/test')).toBeInTheDocument();
  });
});
