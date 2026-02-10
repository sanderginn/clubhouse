import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const apiGetPostQuotes = vi.hoisted(() => vi.fn());
const apiCreateBookQuote = vi.hoisted(() => vi.fn());
const apiUpdateBookQuote = vi.hoisted(() => vi.fn());
const apiDeleteBookQuote = vi.hoisted(() => vi.fn());

vi.mock('../../services/api', () => ({
  api: {
    getPostQuotes: apiGetPostQuotes,
    createBookQuote: apiCreateBookQuote,
    updateBookQuote: apiUpdateBookQuote,
    deleteBookQuote: apiDeleteBookQuote,
  },
}));

const { bookQuoteStore } = await import('../../stores/bookQuoteStore');
const { default: QuoteList } = await import('./QuoteList.svelte');

function buildQuote(overrides: Record<string, unknown> = {}) {
  return {
    id: 'quote-1',
    postId: 'post-1',
    userId: 'user-1',
    quoteText: 'Base quote',
    pageNumber: 10,
    chapter: '1',
    note: 'Note',
    createdAt: '2026-02-10T10:00:00Z',
    updatedAt: '2026-02-10T10:00:00Z',
    username: 'reader',
    displayName: 'Reader',
    ...overrides,
  };
}

beforeEach(() => {
  bookQuoteStore.reset();
  apiGetPostQuotes.mockReset();
  apiCreateBookQuote.mockReset();
  apiUpdateBookQuote.mockReset();
  apiDeleteBookQuote.mockReset();
});

afterEach(() => {
  cleanup();
});

describe('QuoteList', () => {
  it('renders quotes ordered by newest first', async () => {
    const olderQuote = buildQuote({
      id: 'quote-old',
      quoteText: 'Older quote',
      createdAt: '2026-02-09T10:00:00Z',
    });
    const newerQuote = buildQuote({
      id: 'quote-new',
      quoteText: 'Newer quote',
      createdAt: '2026-02-10T12:00:00Z',
    });
    apiGetPostQuotes.mockResolvedValueOnce({
      quotes: [olderQuote, newerQuote],
      nextCursor: null,
      hasMore: false,
    });

    render(QuoteList, {
      postId: 'post-1',
      currentUserId: 'user-1',
      isAdmin: false,
    });

    await waitFor(() => {
      expect(apiGetPostQuotes).toHaveBeenCalledWith('post-1', undefined, 20);
    });

    await waitFor(() => {
      expect(screen.getAllByTestId('quote-text')).toHaveLength(2);
    });
    const quoteTexts = screen.getAllByTestId('quote-text');
    expect(quoteTexts[0]).toHaveTextContent('Newer quote');
    expect(quoteTexts[1]).toHaveTextContent('Older quote');
  });

  it('opens the add quote form when Add Quote is clicked', async () => {
    apiGetPostQuotes.mockResolvedValueOnce({
      quotes: [],
      nextCursor: null,
      hasMore: false,
    });

    render(QuoteList, {
      postId: 'post-1',
      currentUserId: 'user-1',
      isAdmin: false,
    });

    await waitFor(() => {
      expect(apiGetPostQuotes).toHaveBeenCalledWith('post-1', undefined, 20);
    });

    await fireEvent.click(screen.getByTestId('quote-add-button'));
    expect(screen.getByTestId('quote-form-text')).toBeInTheDocument();
  });

  it('shows empty state when a book has no quotes', async () => {
    apiGetPostQuotes.mockResolvedValueOnce({
      quotes: [],
      nextCursor: null,
      hasMore: false,
    });

    render(QuoteList, {
      postId: 'post-1',
      currentUserId: 'user-1',
      isAdmin: false,
    });

    await waitFor(() => {
      expect(
        screen.getByText('No quotes yet. Be the first to share a passage!')
      ).toBeInTheDocument();
    });
  });

  it('loads additional quotes when pagination is available', async () => {
    apiGetPostQuotes
      .mockResolvedValueOnce({
        quotes: [buildQuote({ id: 'quote-1', quoteText: 'First quote' })],
        nextCursor: 'cursor-1',
        hasMore: true,
      })
      .mockResolvedValueOnce({
        quotes: [buildQuote({ id: 'quote-2', quoteText: 'Second quote' })],
        nextCursor: null,
        hasMore: false,
      });

    render(QuoteList, {
      postId: 'post-1',
      currentUserId: 'user-1',
      isAdmin: false,
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Load more quotes' })).toBeInTheDocument();
    });

    await fireEvent.click(screen.getByRole('button', { name: 'Load more quotes' }));

    await waitFor(() => {
      expect(apiGetPostQuotes).toHaveBeenNthCalledWith(2, 'post-1', 'cursor-1', 20);
    });

    expect(screen.getAllByTestId('quote-card')).toHaveLength(2);
  });
});
