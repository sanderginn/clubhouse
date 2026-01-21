import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, screen } from '@testing-library/svelte';
import { sectionStore } from '../../stores';

const { default: Nav } = await import('../Nav.svelte');

beforeEach(() => {
  sectionStore.setSections([
    { id: 'section-1', name: 'Music', type: 'music', icon: '🎵' },
    { id: 'section-2', name: 'Books', type: 'book', icon: '📚' },
  ]);
});

describe('Nav', () => {
  it('clicking section sets active section', async () => {
    const setActiveSpy = vi.spyOn(sectionStore, 'setActiveSection');
    render(Nav);

    const button = screen.getByText('Books');
    await fireEvent.click(button);

    expect(setActiveSpy).toHaveBeenCalled();
    const call = setActiveSpy.mock.calls[0]?.[0];
    expect(call?.id).toBe('section-2');
  });
});
