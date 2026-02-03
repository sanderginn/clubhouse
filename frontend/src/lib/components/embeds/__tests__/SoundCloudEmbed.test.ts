import { render, screen, cleanup } from '@testing-library/svelte';
import { afterEach, describe, it, expect } from 'vitest';
import SoundCloudEmbed from '../SoundCloudEmbed.svelte';

afterEach(() => {
  cleanup();
});

describe('SoundCloudEmbed', () => {
  it('renders SoundCloud iframe with provided height', () => {
    render(SoundCloudEmbed, {
      embedUrl: 'https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/tracks/1',
      height: 240,
      title: 'Test Track',
    });

    const iframe = screen.getByTestId('soundcloud-embed');
    expect(iframe).toHaveAttribute(
      'src',
      'https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/tracks/1'
    );
    expect(iframe).toHaveAttribute('height', '240');
    expect(iframe).toHaveAttribute('title', 'SoundCloud player: Test Track');
  });

  it('falls back to default SoundCloud height', () => {
    render(SoundCloudEmbed, {
      embedUrl: 'https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/tracks/2',
    });

    const iframe = screen.getByTestId('soundcloud-embed');
    expect(iframe).toHaveAttribute('height', '166');
  });
});
