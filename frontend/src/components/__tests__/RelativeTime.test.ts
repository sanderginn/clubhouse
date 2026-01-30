import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

const { default: RelativeTime } = await import('../RelativeTime.svelte');

const baseDate = new Date('2026-01-29T08:47:31Z');
const expectedTooltip = `${baseDate.toLocaleDateString('en-US', {
  month: 'short',
  day: 'numeric',
  year: 'numeric',
})} ${baseDate.toLocaleTimeString('en-US', {
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
})}`;

describe('RelativeTime', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-01-29T09:00:00Z'));
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it('shows exact timestamp tooltip on hover', async () => {
    render(RelativeTime, { dateString: baseDate.toISOString() });

    expect(screen.queryByText(expectedTooltip)).not.toBeInTheDocument();

    const trigger = screen.getByRole('button');
    await fireEvent.mouseEnter(trigger);

    expect(screen.getByText(expectedTooltip)).toBeInTheDocument();
  });

  it('toggles tooltip on click for touch devices', async () => {
    render(RelativeTime, { dateString: baseDate.toISOString() });

    const trigger = screen.getByRole('button');
    await fireEvent.click(trigger);

    expect(screen.getByText(expectedTooltip)).toBeInTheDocument();
  });
});
