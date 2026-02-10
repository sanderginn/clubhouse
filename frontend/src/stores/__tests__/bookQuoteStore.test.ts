import { beforeEach, describe, expect, it, vi } from 'vitest';
import { get } from 'svelte/store';

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

const { bookQuoteStore } = await import('../bookQuoteStore');

function buildQuote(overrides: Record<string, unknown> = {}) {
  return {
    id: 'quote-1',
    postId: 'post-1',
    userId: 'user-1',
    quoteText: 'Start quote',
    pageNumber: 42,
    chapter: 'Chapter 1',
    note: 'Original note',
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

describe('bookQuoteStore', () => {
  it('initializes with empty per-post state maps', () => {
    const state = get(bookQuoteStore);

    expect(state.quotes).toEqual({});
    expect(state.cursors).toEqual({});
    expect(state.hasMore).toEqual({});
    expect(state.isLoading).toEqual({});
    expect(state.errors).toEqual({});
  });

  it('addQuote, editQuote, and removeQuote update store data', async () => {
    const postId = 'post-1';
    const quoteId = 'quote-1';

    apiCreateBookQuote.mockResolvedValue({
      quote: buildQuote({ id: quoteId, postId, quoteText: 'Created quote' }),
    });
    apiUpdateBookQuote.mockResolvedValue({
      quote: buildQuote({
        id: quoteId,
        postId,
        quoteText: 'Edited quote',
        note: 'Updated note',
        updatedAt: '2026-02-10T11:00:00Z',
      }),
    });
    apiDeleteBookQuote.mockResolvedValue(undefined);

    await bookQuoteStore.addQuote(postId, { quoteText: 'Created quote' });
    let state = get(bookQuoteStore);
    expect(state.quotes[postId]).toHaveLength(1);
    expect(state.quotes[postId][0].quoteText).toBe('Created quote');

    await bookQuoteStore.editQuote(quoteId, { quoteText: 'Edited quote', note: 'Updated note' });
    state = get(bookQuoteStore);
    expect(state.quotes[postId]).toHaveLength(1);
    expect(state.quotes[postId][0].quoteText).toBe('Edited quote');
    expect(state.quotes[postId][0].note).toBe('Updated note');

    await bookQuoteStore.removeQuote(quoteId);
    state = get(bookQuoteStore);
    expect(state.quotes[postId]).toBeUndefined();

    expect(apiCreateBookQuote).toHaveBeenCalledWith(postId, { quoteText: 'Created quote' });
    expect(apiUpdateBookQuote).toHaveBeenCalledWith(quoteId, {
      quoteText: 'Edited quote',
      note: 'Updated note',
    });
    expect(apiDeleteBookQuote).toHaveBeenCalledWith(quoteId);
  });

  it('loadQuotesForPost loads first page and appends deduped pagination results', async () => {
    const postId = 'post-2';
    const first = buildQuote({ id: 'quote-a', postId, quoteText: 'First' });
    const second = buildQuote({ id: 'quote-b', postId, quoteText: 'Second' });
    const third = buildQuote({ id: 'quote-c', postId, quoteText: 'Third' });

    apiGetPostQuotes
      .mockResolvedValueOnce({
        quotes: [first, second],
        nextCursor: 'cursor-1',
        hasMore: true,
      })
      .mockResolvedValueOnce({
        quotes: [second, third],
        hasMore: false,
      });

    await bookQuoteStore.loadQuotesForPost(postId, undefined, 2);
    let state = get(bookQuoteStore);
    expect(state.quotes[postId].map((quote) => quote.id)).toEqual(['quote-a', 'quote-b']);
    expect(state.cursors[postId]).toBe('cursor-1');
    expect(state.hasMore[postId]).toBe(true);

    await bookQuoteStore.loadQuotesForPost(postId, 'cursor-1', 2);
    state = get(bookQuoteStore);
    expect(state.quotes[postId].map((quote) => quote.id)).toEqual(['quote-a', 'quote-b', 'quote-c']);
    expect(state.cursors[postId]).toBeNull();
    expect(state.hasMore[postId]).toBe(false);

    expect(apiGetPostQuotes).toHaveBeenNthCalledWith(1, postId, undefined, 2);
    expect(apiGetPostQuotes).toHaveBeenNthCalledWith(2, postId, 'cursor-1', 2);
  });
});
