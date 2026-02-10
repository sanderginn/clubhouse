<script lang="ts">
  import { api, type BookQuoteWithUser } from '../../services/api';
  import { bookQuoteStore } from '../../stores/bookQuoteStore';
  import QuoteForm from './QuoteForm.svelte';

  export let quote: BookQuoteWithUser;
  export let currentUserId: string;
  export let isAdmin: boolean;

  let isEditing = false;
  let isDeleting = false;
  let error: string | null = null;

  type QuoteFormSubmitPayload = {
    postId: string;
    quoteText: string;
    pageNumber?: number;
    chapter?: string;
    note?: string;
    quoteId?: string;
  };

  $: canManage = currentUserId === quote.userId || isAdmin;
  $: quoteReference = [formatPageNumber(quote.pageNumber), formatChapter(quote.chapter)]
    .filter((part): part is string => Boolean(part))
    .join(' - ');
  $: authorLabel = quote.displayName?.trim() || quote.username;
  $: formattedDate = formatDate(quote.createdAt);

  function formatPageNumber(pageNumber?: number): string | null {
    if (typeof pageNumber !== 'number' || pageNumber < 1) {
      return null;
    }
    return `Page ${pageNumber}`;
  }

  function formatChapter(chapter?: string): string | null {
    if (typeof chapter !== 'string') {
      return null;
    }
    const trimmed = chapter.trim();
    return trimmed ? `Chapter ${trimmed}` : null;
  }

  function formatDate(value: string): string {
    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) {
      return value;
    }
    return parsed.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  }

  function startEdit() {
    isEditing = true;
    error = null;
  }

  function cancelEdit() {
    isEditing = false;
    error = null;
  }

  async function handleEditSubmit(payload: QuoteFormSubmitPayload) {
    const response = await api.updateBookQuote(quote.id, {
      quoteText: payload.quoteText,
      pageNumber: payload.pageNumber,
      chapter: payload.chapter,
      note: payload.note,
    });
    bookQuoteStore.applyQuote(response.quote);
    isEditing = false;
  }

  async function deleteQuote() {
    if (typeof window !== 'undefined') {
      const confirmed = window.confirm('Delete this quote?');
      if (!confirmed) {
        return;
      }
    }

    isDeleting = true;
    error = null;
    try {
      await api.deleteBookQuote(quote.id);
      bookQuoteStore.applyQuoteRemoval(quote.id);
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to delete quote.';
    } finally {
      isDeleting = false;
    }
  }
</script>

<article class="space-y-3 rounded-xl border border-gray-200 bg-white p-4 shadow-sm" data-testid="quote-card">
  {#if isEditing}
    <QuoteForm postId={quote.postId} existingQuote={quote} onSubmit={handleEditSubmit} onCancel={cancelEdit} />
  {:else}
    <blockquote class="rounded-lg border-l-4 border-slate-400 bg-slate-50 px-4 py-3 text-sm text-slate-800">
      <p data-testid="quote-text">{quote.quoteText}</p>
    </blockquote>

    {#if quoteReference}
      <p class="text-xs font-medium uppercase tracking-wide text-slate-500" data-testid="quote-reference">
        {quoteReference}
      </p>
    {/if}

    {#if quote.note}
      <p class="text-sm text-slate-500" data-testid="quote-note">{quote.note}</p>
    {/if}

    <div class="flex flex-wrap items-center justify-between gap-2">
      <p class="text-xs text-slate-600" data-testid="quote-author">
        <span class="font-medium text-slate-800">{authorLabel}</span>
        <span> (@{quote.username})</span>
      </p>
      <time class="text-xs text-slate-500" datetime={quote.createdAt} data-testid="quote-date">{formattedDate}</time>
    </div>

    {#if canManage}
      <div class="flex items-center justify-end gap-2">
        <button
          type="button"
          on:click={startEdit}
          class="rounded-lg border border-gray-300 px-2.5 py-1 text-xs font-medium text-gray-700 hover:bg-gray-100"
          data-testid="quote-edit-button"
        >
          Edit
        </button>
        <button
          type="button"
          on:click={deleteQuote}
          disabled={isDeleting}
          class="rounded-lg border border-red-200 px-2.5 py-1 text-xs font-medium text-red-700 hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-60"
          data-testid="quote-delete-button"
        >
          {isDeleting ? 'Deleting...' : 'Delete'}
        </button>
      </div>
    {/if}
  {/if}

  {#if error}
    <p class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700" data-testid="quote-card-error">
      {error}
    </p>
  {/if}
</article>
