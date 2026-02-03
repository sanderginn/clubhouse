import { describe, it, expect, afterEach, vi } from 'vitest';
import { render, cleanup, fireEvent } from '@testing-library/svelte';

const { default: HighlightDisplay } = await import('../HighlightDisplay.svelte');

afterEach(() => {
  cleanup();
});

describe('HighlightDisplay', () => {
  it('renders formatted timestamps with labels', () => {
    const { getByText } = render(HighlightDisplay, {
      highlights: [{ timestamp: 75, label: 'Intro' }],
    });

    expect(getByText('01:15')).toBeInTheDocument();
    expect(getByText('Intro')).toBeInTheDocument();
  });

  it('renders timestamps without labels', () => {
    const { getByText, queryByText } = render(HighlightDisplay, {
      highlights: [{ timestamp: 5 }],
    });

    expect(getByText('00:05')).toBeInTheDocument();
    expect(queryByText('Intro')).not.toBeInTheDocument();
  });

  it('renders hour timestamps when needed', () => {
    const { getByText } = render(HighlightDisplay, {
      highlights: [{ timestamp: 3930 }],
    });

    expect(getByText('1:05:30')).toBeInTheDocument();
  });

  it('renders nothing when highlights are empty', () => {
    const { queryByLabelText } = render(HighlightDisplay, {
      highlights: [],
    });

    expect(queryByLabelText('Highlights')).not.toBeInTheDocument();
  });

  it('renders clickable highlights when seek handler is provided', () => {
    const onSeek = vi.fn().mockResolvedValue(true);
    const { getByRole } = render(HighlightDisplay, {
      highlights: [{ timestamp: 5, label: 'Intro' }],
      onSeek,
    });

    expect(getByRole('button', { name: '00:05 Intro' })).toBeInTheDocument();
  });

  it('shows feedback when seeking is unsupported', async () => {
    const onSeek = vi.fn().mockResolvedValue(false);
    const { getByRole, getByText } = render(HighlightDisplay, {
      highlights: [{ timestamp: 30 }],
      onSeek,
      unsupportedMessage: 'Seeking not supported',
    });

    await fireEvent.click(getByRole('button', { name: '00:30' }));
    expect(getByText('Seeking not supported')).toBeInTheDocument();
  });
});
