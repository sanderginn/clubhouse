import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';

import YouTubeEmbed from '../YouTubeEmbed.svelte';

describe('YouTubeEmbed', () => {
  it('renders a responsive iframe with the embed URL', () => {
    const embedUrl = 'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ';
    render(YouTubeEmbed, { embedUrl, title: 'Video' });

    const iframe = screen.getByTestId('youtube-embed-frame');
    expect(iframe).toBeInTheDocument();
    expect(iframe).toHaveAttribute('src', embedUrl);
    expect(iframe).toHaveAttribute('allowfullscreen');
  });
});
