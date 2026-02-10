import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it } from 'vitest';

const { default: SpoilerWrapper } = await import('./SpoilerWrapper.svelte');

afterEach(() => {
  cleanup();
});

describe('SpoilerWrapper', () => {
  it('blurs content and shows reveal overlay when spoiler is hidden', () => {
    render(SpoilerWrapper, { isSpoiler: true });

    expect(screen.getByTestId('spoiler-content')).toHaveStyle('filter: blur(8px)');
    expect(screen.getByTestId('spoiler-overlay')).toBeInTheDocument();
    expect(screen.getByText('Spoiler — Click to reveal')).toBeInTheDocument();
  });

  it('reveals spoiler content on click', async () => {
    render(SpoilerWrapper, { isSpoiler: true });

    await fireEvent.click(screen.getByTestId('spoiler-overlay'));

    expect(screen.queryByTestId('spoiler-overlay')).not.toBeInTheDocument();
    expect(screen.getByTestId('spoiler-content')).toHaveStyle('filter: none');
    expect(screen.getByTestId('spoiler-badge')).toBeInTheDocument();
  });

  it('shows content normally when not marked as spoiler', () => {
    render(SpoilerWrapper, { isSpoiler: false });

    expect(screen.queryByTestId('spoiler-overlay')).not.toBeInTheDocument();
    expect(screen.getByTestId('spoiler-content')).toHaveStyle('filter: none');
    expect(screen.queryByTestId('spoiler-badge')).not.toBeInTheDocument();
  });
});
