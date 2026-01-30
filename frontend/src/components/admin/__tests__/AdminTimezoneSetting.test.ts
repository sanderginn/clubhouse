import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, cleanup, waitFor } from '@testing-library/svelte';
import { tick } from 'svelte';
import { get } from 'svelte/store';
import { configStore, displayTimezone } from '../../../stores';

const apiGet = vi.hoisted(() => vi.fn());
const apiPatch = vi.hoisted(() => vi.fn());

vi.mock('../../../services/api', () => ({
  api: {
    get: apiGet,
    patch: apiPatch,
  },
}));

const { default: AdminTimezoneSetting } = await import('../AdminTimezoneSetting.svelte');

const flush = async () => {
  await tick();
  await Promise.resolve();
  await tick();
};

beforeEach(() => {
  apiGet.mockReset();
  apiPatch.mockReset();
  configStore.setDisplayTimezone(null);
});

afterEach(() => {
  cleanup();
});

describe('AdminTimezoneSetting', () => {
  it('loads the configured timezone into the dropdown', async () => {
    apiGet.mockResolvedValue({ config: { displayTimezone: 'America/New_York' } });

    render(AdminTimezoneSetting);
    await flush();

    const select = screen.getByLabelText(/display timezone/i) as HTMLSelectElement;
    await waitFor(() => {
      expect(select.value).toBe('America/New_York');
    });

    expect(get(displayTimezone)).toBe('America/New_York');
  });

  it('saves timezone changes', async () => {
    apiGet.mockResolvedValue({ config: { displayTimezone: 'America/Los_Angeles' } });
    apiPatch.mockResolvedValue({ config: { displayTimezone: 'UTC' } });

    render(AdminTimezoneSetting);
    await flush();

    const select = screen.getByLabelText(/display timezone/i) as HTMLSelectElement;
    await fireEvent.change(select, { target: { value: 'UTC' } });

    const saveButton = screen.getByRole('button', { name: /save timezone/i });
    await fireEvent.click(saveButton);

    expect(apiPatch).toHaveBeenCalledWith('/admin/config', { display_timezone: 'UTC' });
    expect(get(displayTimezone)).toBe('UTC');
  });
});
