import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import QuoteForm from './QuoteForm.svelte';

afterEach(() => {
  cleanup();
});

describe('QuoteForm', () => {
  it('validates required quote text', async () => {
    const onSubmit = vi.fn();
    render(QuoteForm, {
      postId: 'post-1',
      onSubmit,
      onCancel: vi.fn(),
    });

    const form = screen.getByTestId('quote-form-text').closest('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);

    expect(onSubmit).not.toHaveBeenCalled();
    expect(screen.getByText('Quote text is required.')).toBeInTheDocument();
  });

  it('validates page number as positive integer', async () => {
    const onSubmit = vi.fn();
    render(QuoteForm, {
      postId: 'post-1',
      onSubmit,
      onCancel: vi.fn(),
    });

    await fireEvent.input(screen.getByLabelText('Quote text'), {
      target: { value: 'The sky above the port was the color of television.' },
    });
    await fireEvent.input(screen.getByLabelText('Page number (optional)'), {
      target: { value: '0' },
    });

    const form = screen.getByTestId('quote-form-text').closest('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);

    expect(onSubmit).not.toHaveBeenCalled();
    expect(screen.getByText('Page number must be a positive integer.')).toBeInTheDocument();
  });

  it('submits create payload', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(QuoteForm, {
      postId: 'post-1',
      onSubmit,
      onCancel: vi.fn(),
    });

    await fireEvent.input(screen.getByLabelText('Quote text'), {
      target: { value: 'A person can change at the edge of a forest.' },
    });
    await fireEvent.input(screen.getByLabelText('Page number (optional)'), {
      target: { value: '42' },
    });
    await fireEvent.input(screen.getByLabelText('Chapter (optional)'), {
      target: { value: '5' },
    });
    await fireEvent.input(screen.getByLabelText('Note (optional)'), {
      target: { value: 'Important turning point' },
    });

    const form = screen.getByTestId('quote-form-text').closest('form');
    if (!form) throw new Error('form not found');
    await fireEvent.submit(form);

    expect(onSubmit).toHaveBeenCalledWith({
      postId: 'post-1',
      quoteText: 'A person can change at the edge of a forest.',
      pageNumber: 42,
      chapter: '5',
      note: 'Important turning point',
      quoteId: undefined,
    });
  });

  it('pre-fills fields for edit mode', () => {
    render(QuoteForm, {
      postId: 'post-1',
      existingQuote: {
        id: 'quote-1',
        postId: 'post-1',
        userId: 'user-1',
        quoteText: 'Existing quote',
        pageNumber: 88,
        chapter: 'Epilogue',
        note: 'Original note',
        createdAt: '2026-01-01T12:00:00Z',
        updatedAt: '2026-01-01T12:00:00Z',
        username: 'reader',
        displayName: 'Reader One',
      },
      onSubmit: vi.fn(),
      onCancel: vi.fn(),
    });

    expect(screen.getByLabelText('Quote text')).toHaveValue('Existing quote');
    expect(screen.getByLabelText('Page number (optional)')).toHaveValue(88);
    expect(screen.getByLabelText('Chapter (optional)')).toHaveValue('Epilogue');
    expect(screen.getByLabelText('Note (optional)')).toHaveValue('Original note');
    expect(screen.getByRole('button', { name: 'Save Quote' })).toBeInTheDocument();
  });
});
