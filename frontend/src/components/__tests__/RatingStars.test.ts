import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import { describe, it, expect, afterEach, vi } from 'vitest';

const { default: RatingStars } = await import('../recipes/RatingStars.svelte');

describe('RatingStars', () => {
  afterEach(() => {
    cleanup();
  });

  it('renders half stars in readonly mode', () => {
    render(RatingStars, { value: 4.3, readonly: true });

    const halfStar = screen.getByTestId('rating-star-fill-5');
    expect(halfStar).toHaveStyle('width: 50%');
  });

  it('toggles selection when clicking the same rating', async () => {
    const onChange = vi.fn();
    render(RatingStars, { value: 3, onChange });

    const star3 = screen.getByTestId('rating-star-3');
    await fireEvent.click(star3);

    expect(onChange).toHaveBeenCalledWith(0);

    const star4 = screen.getByTestId('rating-star-4');
    await fireEvent.click(star4);

    expect(onChange).toHaveBeenCalledWith(4);
  });

  it('supports keyboard interaction with arrow keys and enter', async () => {
    const onChange = vi.fn();
    render(RatingStars, { value: 1, onChange });

    const slider = screen.getByRole('slider');
    slider.focus();

    await fireEvent.keyDown(slider, { key: 'ArrowRight' });
    await fireEvent.keyDown(slider, { key: 'Enter' });

    expect(onChange).toHaveBeenCalledWith(2);
  });
});
