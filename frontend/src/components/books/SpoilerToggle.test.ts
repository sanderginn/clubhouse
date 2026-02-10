import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

const { default: SpoilerToggle } = await import('./SpoilerToggle.svelte');

afterEach(() => {
  cleanup();
});

describe('SpoilerToggle', () => {
  it('renders unchecked state', () => {
    render(SpoilerToggle, { checked: false });

    const input = screen.getByLabelText('Contains spoiler') as HTMLInputElement;
    expect(input.checked).toBe(false);
    expect(screen.queryByTestId('spoiler-toggle-eye-slash')).not.toBeInTheDocument();
  });

  it('renders checked state with eye-slash icon', () => {
    render(SpoilerToggle, { checked: true });

    const input = screen.getByLabelText('Contains spoiler') as HTMLInputElement;
    expect(input.checked).toBe(true);
    expect(screen.getByTestId('spoiler-toggle-eye-slash')).toBeInTheDocument();
  });

  it('emits change events with boolean value', async () => {
    const { component } = render(SpoilerToggle, { checked: false });
    const onChange = vi.fn();
    component.$on('change', onChange);

    const input = screen.getByLabelText('Contains spoiler');
    await fireEvent.click(input);
    await fireEvent.click(input);

    expect(onChange).toHaveBeenCalledTimes(2);
    expect(onChange.mock.calls[0]?.[0]?.detail).toBe(true);
    expect(onChange.mock.calls[1]?.[0]?.detail).toBe(false);
  });
});
