<script lang="ts">
  import { get } from 'svelte/store';
  import type { BookQuoteWithUser } from '../../services/api';
  import { bookQuoteStore } from '../../stores/bookQuoteStore';
  import QuoteCard from './QuoteCard.svelte';
  import QuoteForm from './QuoteForm.svelte';

  type QuoteFormSubmitPayload = {
    postId: string;
    quoteText: string;
    pageNumber?: number;
    chapter?: string;
    note?: string;
    quoteId?: string;
  };

  const PAGE_SIZE = 20;

  export let postId: string;
  export let currentUserId = '';
  export let isAdmin = false;

  let isExpanded = true;
  let isAddFormOpen = false;
  let lastLoadedPostID: string | null = null;
  let hasLoadedInitialPage = false;

  $: quoteState = $bookQuoteStore;
  $: quotes = quoteState.quotes[postId] ?? [];
  $: cursor = quoteState.cursors[postId] ?? null;
  $: hasMore = quoteState.hasMore[postId] ?? false;
  $: isLoading = quoteState.isLoading[postId] ?? false;
  $: error = quoteState.errors[postId] ?? null;
  $: sortedQuotes = sortQuotesByNewest(quotes);

  $: if (postId && lastLoadedPostID !== postId) {
    lastLoadedPostID = postId;
    isAddFormOpen = false;
    void loadInitialQuotes(postId);
  }

  function sortQuotesByNewest(items: BookQuoteWithUser[]): BookQuoteWithUser[] {
    return [...items].sort((a, b) => {
      const left = Date.parse(a.createdAt);
      const right = Date.parse(b.createdAt);
      const normalizedLeft = Number.isFinite(left) ? left : 0;
      const normalizedRight = Number.isFinite(right) ? right : 0;
      return normalizedRight - normalizedLeft;
    });
  }

  async function loadInitialQuotes(targetPostID: string) {
    hasLoadedInitialPage = false;
    await bookQuoteStore.loadQuotesForPost(targetPostID, undefined, PAGE_SIZE);
    if (lastLoadedPostID === targetPostID) {
      hasLoadedInitialPage = true;
    }
  }

  async function loadMoreQuotes() {
    if (!hasMore || isLoading || !cursor) {
      return;
    }
    await bookQuoteStore.loadQuotesForPost(postId, cursor, PAGE_SIZE);
  }

  async function handleAddQuote(payload: QuoteFormSubmitPayload) {
    await bookQuoteStore.addQuote(postId, {
      quoteText: payload.quoteText,
      pageNumber: payload.pageNumber,
      chapter: payload.chapter,
      note: payload.note,
    });

    const latestState = get(bookQuoteStore);
    const postError = latestState.errors[postId];
    if (postError) {
      throw new Error(postError);
    }

    isAddFormOpen = false;
  }

  function toggleExpanded() {
    isExpanded = !isExpanded;
  }
</script>

<section class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm" data-testid="quote-list">
  <div class="flex items-center justify-between gap-3">
    <button
      type="button"
      class="inline-flex items-center gap-2 text-left text-sm font-semibold text-gray-900"
      aria-expanded={isExpanded}
      on:click={toggleExpanded}
      data-testid="quote-list-toggle"
    >
      <span>Quotes</span>
      <span
        class="inline-flex min-w-6 items-center justify-center rounded-full border border-gray-200 bg-gray-50 px-2 py-0.5 text-xs font-semibold text-gray-700"
        data-testid="quote-count-badge"
      >
        {sortedQuotes.length}
      </span>
      <span class="text-xs text-gray-500">{isExpanded ? '▾' : '▸'}</span>
    </button>

    {#if isExpanded}
      <button
        type="button"
        class="rounded-lg border border-gray-300 px-3 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-100"
        on:click={() => {
          isAddFormOpen = !isAddFormOpen;
        }}
        data-testid="quote-add-button"
      >
        {isAddFormOpen ? 'Close' : 'Add Quote'}
      </button>
    {/if}
  </div>

  {#if isExpanded}
    <div class="mt-3 space-y-3">
      {#if isAddFormOpen}
        <QuoteForm postId={postId} onSubmit={handleAddQuote} onCancel={() => (isAddFormOpen = false)} />
      {/if}

      {#if error}
        <p class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700" data-testid="quote-list-error">
          {error}
        </p>
      {/if}

      {#if isLoading && !hasLoadedInitialPage}
        <p class="text-sm text-gray-600">Loading quotes...</p>
      {:else if hasLoadedInitialPage && sortedQuotes.length === 0}
        <p class="rounded-lg border border-dashed border-gray-300 bg-gray-50 px-3 py-3 text-sm text-gray-600">
          No quotes yet. Be the first to share a passage!
        </p>
      {:else if sortedQuotes.length > 0}
        <ul class="space-y-3" data-testid="quote-list-items">
          {#each sortedQuotes as quote (quote.id)}
            <li>
              <QuoteCard {quote} {currentUserId} {isAdmin} />
            </li>
          {/each}
        </ul>
      {/if}

      {#if hasMore}
        <div class="flex justify-center">
          <button
            type="button"
            class="text-xs font-medium text-gray-600 hover:text-gray-900 disabled:cursor-not-allowed disabled:opacity-60"
            on:click={loadMoreQuotes}
            disabled={isLoading}
          >
            {isLoading ? 'Loading...' : 'Load more quotes'}
          </button>
        </div>
      {/if}
    </div>
  {/if}
</section>
