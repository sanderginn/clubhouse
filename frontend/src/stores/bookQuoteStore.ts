import { derived, writable } from 'svelte/store';
import {
  api,
  type BookQuoteWithUser,
  type CreateBookQuoteRequest,
  type UpdateBookQuoteRequest,
} from '../services/api';

export interface BookQuoteStoreState {
  quotes: Record<string, BookQuoteWithUser[]>;
  cursors: Record<string, string | null>;
  hasMore: Record<string, boolean>;
  isLoading: Record<string, boolean>;
  errors: Record<string, string | null>;
}

const initialState: BookQuoteStoreState = {
  quotes: {},
  cursors: {},
  hasMore: {},
  isLoading: {},
  errors: {},
};

function appendUniqueQuotes(
  existingQuotes: BookQuoteWithUser[],
  newQuotes: BookQuoteWithUser[]
): BookQuoteWithUser[] {
  const seen = new Set(existingQuotes.map((quote) => quote.id));
  const uniqueQuotes = newQuotes.filter((quote) => {
    if (seen.has(quote.id)) {
      return false;
    }
    seen.add(quote.id);
    return true;
  });
  return [...existingQuotes, ...uniqueQuotes];
}

function upsertQuote(
  quotesByPost: Record<string, BookQuoteWithUser[]>,
  quote: BookQuoteWithUser
): Record<string, BookQuoteWithUser[]> {
  const nextQuotesByPost = { ...quotesByPost };
  const existingQuotes = nextQuotesByPost[quote.postId] ?? [];
  const existingIndex = existingQuotes.findIndex((item) => item.id === quote.id);

  if (existingIndex === -1) {
    nextQuotesByPost[quote.postId] = [quote, ...existingQuotes];
    return nextQuotesByPost;
  }

  const updatedQuotes = [...existingQuotes];
  updatedQuotes[existingIndex] = {
    ...updatedQuotes[existingIndex],
    ...quote,
  };
  nextQuotesByPost[quote.postId] = updatedQuotes;
  return nextQuotesByPost;
}

function removeQuoteByID(
  quotesByPost: Record<string, BookQuoteWithUser[]>,
  quoteID: string
): Record<string, BookQuoteWithUser[]> {
  const nextQuotesByPost = { ...quotesByPost };
  for (const [postID, quotes] of Object.entries(nextQuotesByPost)) {
    const filtered = quotes.filter((quote) => quote.id !== quoteID);
    if (filtered.length !== quotes.length) {
      if (filtered.length > 0) {
        nextQuotesByPost[postID] = filtered;
      } else {
        delete nextQuotesByPost[postID];
      }
      return nextQuotesByPost;
    }
  }
  return nextQuotesByPost;
}

function createBookQuoteStore() {
  const { subscribe, set, update } = writable<BookQuoteStoreState>({ ...initialState });

  const setLoading = (postId: string, isLoading: boolean) =>
    update((state) => ({
      ...state,
      isLoading: {
        ...state.isLoading,
        [postId]: isLoading,
      },
      errors: {
        ...state.errors,
        [postId]: isLoading ? null : state.errors[postId] ?? null,
      },
    }));

  const setError = (postId: string, error: string | null) =>
    update((state) => ({
      ...state,
      isLoading: {
        ...state.isLoading,
        [postId]: false,
      },
      errors: {
        ...state.errors,
        [postId]: error,
      },
    }));

  return {
    subscribe,
    reset: () => set({ ...initialState }),
    setQuotesForPost: (
      postId: string,
      quotes: BookQuoteWithUser[],
      nextCursor: string | null,
      hasMore: boolean
    ) =>
      update((state) => ({
        ...state,
        quotes: {
          ...state.quotes,
          [postId]: quotes,
        },
        cursors: {
          ...state.cursors,
          [postId]: nextCursor,
        },
        hasMore: {
          ...state.hasMore,
          [postId]: hasMore,
        },
        isLoading: {
          ...state.isLoading,
          [postId]: false,
        },
        errors: {
          ...state.errors,
          [postId]: null,
        },
      })),
    appendQuotesForPost: (
      postId: string,
      quotes: BookQuoteWithUser[],
      nextCursor: string | null,
      hasMore: boolean
    ) =>
      update((state) => ({
        ...state,
        quotes: {
          ...state.quotes,
          [postId]: appendUniqueQuotes(state.quotes[postId] ?? [], quotes),
        },
        cursors: {
          ...state.cursors,
          [postId]: nextCursor,
        },
        hasMore: {
          ...state.hasMore,
          [postId]: hasMore,
        },
        isLoading: {
          ...state.isLoading,
          [postId]: false,
        },
        errors: {
          ...state.errors,
          [postId]: null,
        },
      })),
    applyQuote: (quote: BookQuoteWithUser) =>
      update((state) => ({
        ...state,
        quotes: upsertQuote(state.quotes, quote),
        errors: {
          ...state.errors,
          [quote.postId]: null,
        },
      })),
    applyQuoteRemoval: (quoteID: string) =>
      update((state) => ({
        ...state,
        quotes: removeQuoteByID(state.quotes, quoteID),
      })),
    loadQuotesForPost: async (postId: string, cursor?: string, limit?: number): Promise<void> => {
      setLoading(postId, true);
      try {
        const response = await api.getPostQuotes(postId, cursor, limit);
        const nextCursor = response.nextCursor ?? null;
        if (cursor) {
          bookQuoteStore.appendQuotesForPost(postId, response.quotes ?? [], nextCursor, response.hasMore);
        } else {
          bookQuoteStore.setQuotesForPost(postId, response.quotes ?? [], nextCursor, response.hasMore);
        }
      } catch (error) {
        setError(postId, error instanceof Error ? error.message : 'Failed to load quotes');
      }
    },
    addQuote: async (postId: string, req: CreateBookQuoteRequest): Promise<void> => {
      try {
        const response = await api.createBookQuote(postId, req);
        bookQuoteStore.applyQuote(response.quote);
      } catch (error) {
        setError(postId, error instanceof Error ? error.message : 'Failed to add quote');
      }
    },
    editQuote: async (quoteId: string, req: UpdateBookQuoteRequest): Promise<void> => {
      try {
        const response = await api.updateBookQuote(quoteId, req);
        bookQuoteStore.applyQuote(response.quote);
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Failed to update quote';
        update((state) => {
          const postID =
            Object.entries(state.quotes).find(([, quotes]) =>
              quotes.some((quote) => quote.id === quoteId)
            )?.[0] ?? '';
          if (!postID) {
            return state;
          }
          return {
            ...state,
            isLoading: {
              ...state.isLoading,
              [postID]: false,
            },
            errors: {
              ...state.errors,
              [postID]: message,
            },
          };
        });
      }
    },
    removeQuote: async (quoteId: string): Promise<void> => {
      try {
        await api.deleteBookQuote(quoteId);
        bookQuoteStore.applyQuoteRemoval(quoteId);
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Failed to remove quote';
        update((state) => {
          const postID =
            Object.entries(state.quotes).find(([, quotes]) =>
              quotes.some((quote) => quote.id === quoteId)
            )?.[0] ?? '';
          if (!postID) {
            return state;
          }
          return {
            ...state,
            isLoading: {
              ...state.isLoading,
              [postID]: false,
            },
            errors: {
              ...state.errors,
              [postID]: message,
            },
          };
        });
      }
    },
  };
}

export const bookQuoteStore = createBookQuoteStore();
export const quotesByPost = derived(bookQuoteStore, ($store) => $store.quotes);
