import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';

import YouTubeEmbed from '../YouTubeEmbed.svelte';

describe('YouTubeEmbed', () => {
  it('renders a responsive player container', () => {
    const embedUrl = 'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ';
    render(YouTubeEmbed, { embedUrl, title: 'Video' });

    const container = screen.getByTestId('youtube-embed-frame');
    expect(container).toBeInTheDocument();
  });
});
