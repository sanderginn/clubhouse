import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it } from 'vitest';
import QuoteCard from './QuoteCard.svelte';

const baseQuote = {
  id: 'quote-1',
  postId: 'post-1',
  userId: 'user-1',
  quoteText: 'This is how worlds begin.',
  pageNumber: 120,
  chapter: '12',
  note: 'Sets up the ending perfectly.',
  createdAt: '2026-01-15T08:30:00Z',
  updatedAt: '2026-01-15T08:30:00Z',
  username: 'sander',
  displayName: 'Sander Ginn',
};

afterEach(() => {
  cleanup();
});

describe('QuoteCard', () => {
  it('renders quote text, context, note, author, and date', () => {
    render(QuoteCard, {
      quote: baseQuote,
      currentUserId: 'user-1',
      isAdmin: false,
    });

    expect(screen.getByTestId('quote-text')).toHaveTextContent('This is how worlds begin.');
    expect(screen.getByTestId('quote-reference')).toHaveTextContent('Page 120 - Chapter 12');
    expect(screen.getByTestId('quote-note')).toHaveTextContent('Sets up the ending perfectly.');
    expect(screen.getByTestId('quote-author')).toHaveTextContent('Sander Ginn (@sander)');
    expect(screen.getByTestId('quote-date')).toHaveAttribute('datetime', '2026-01-15T08:30:00Z');
  });

  it('shows edit and delete controls for quote owner', () => {
    render(QuoteCard, {
      quote: baseQuote,
      currentUserId: 'user-1',
      isAdmin: false,
    });

    expect(screen.getByTestId('quote-edit-button')).toBeInTheDocument();
    expect(screen.getByTestId('quote-delete-button')).toBeInTheDocument();
  });

  it('shows edit and delete controls for admin on another user quote', () => {
    render(QuoteCard, {
      quote: { ...baseQuote, userId: 'user-2' },
      currentUserId: 'admin-1',
      isAdmin: true,
    });

    expect(screen.getByTestId('quote-edit-button')).toBeInTheDocument();
    expect(screen.getByTestId('quote-delete-button')).toBeInTheDocument();
  });

  it('hides edit and delete controls for unrelated non-admin user', () => {
    render(QuoteCard, {
      quote: { ...baseQuote, userId: 'user-2' },
      currentUserId: 'user-3',
      isAdmin: false,
    });

    expect(screen.queryByTestId('quote-edit-button')).not.toBeInTheDocument();
    expect(screen.queryByTestId('quote-delete-button')).not.toBeInTheDocument();
  });

  it('opens inline quote form when edit is clicked', async () => {
    render(QuoteCard, {
      quote: baseQuote,
      currentUserId: 'user-1',
      isAdmin: false,
    });

    await fireEvent.click(screen.getByTestId('quote-edit-button'));
    expect(screen.getByLabelText('Quote text')).toHaveValue('This is how worlds begin.');
    expect(screen.getByRole('button', { name: 'Save Quote' })).toBeInTheDocument();
  });
});
