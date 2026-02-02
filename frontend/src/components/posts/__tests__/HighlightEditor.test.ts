import { describe, it, expect, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';

const { default: HighlightEditor } = await import('../HighlightEditor.svelte');

afterEach(() => {
  cleanup();
});

describe('HighlightEditor', () => {
  it('adds a highlight from mm:ss input', async () => {
    const { getByLabelText, getByText } = render(HighlightEditor, {
      highlights: [],
    });

    const timestampInput = getByLabelText('Timestamp (mm:ss)');
    const labelInput = getByLabelText('Label (optional)');

    await fireEvent.input(timestampInput, { target: { value: '01:30' } });
    await fireEvent.input(labelInput, { target: { value: 'Intro' } });
    await fireEvent.click(getByText('Add highlight'));

    expect(getByText('01:30')).toBeInTheDocument();
    expect(getByText('Intro')).toBeInTheDocument();
  });

  it('removes a highlight', async () => {
    const { getByLabelText, queryByText } = render(HighlightEditor, {
      highlights: [
        { timestamp: 30, label: 'Intro' },
        { timestamp: 90, label: 'Drop' },
      ],
    });

    await fireEvent.click(getByLabelText('Remove highlight 00:30'));

    expect(queryByText('00:30')).not.toBeInTheDocument();
    expect(queryByText('Intro')).not.toBeInTheDocument();
  });

  it('shows max highlights message and disables add', () => {
    const { getByText } = render(HighlightEditor, {
      highlights: Array.from({ length: 20 }, (_, index) => ({
        timestamp: index,
        label: `Label ${index}`,
      })),
    });

    const addButton = getByText('Add highlight') as HTMLButtonElement;

    expect(addButton.disabled).toBe(true);
    expect(getByText('Maximum of 20 highlights reached.')).toBeInTheDocument();
  });
});
