<script lang="ts">
  import type { BookQuoteWithUser } from '../../services/api';

  type QuoteFormSubmitPayload = {
    postId: string;
    quoteText: string;
    pageNumber?: number;
    chapter?: string;
    note?: string;
    quoteId?: string;
  };

  export let postId: string;
  export let existingQuote: BookQuoteWithUser | undefined = undefined;
  export let onSubmit: (payload: QuoteFormSubmitPayload) => Promise<void> | void;
  export let onCancel: () => void;

  let quoteText = '';
  let pageNumberInput: string | number = '';
  let chapter = '';
  let note = '';
  let error: string | null = null;
  let isSubmitting = false;
  let lastHydratedQuoteId: string | null = null;

  $: hydratedQuoteID = existingQuote?.id ?? null;
  $: if (hydratedQuoteID !== lastHydratedQuoteId) {
    quoteText = existingQuote?.quoteText ?? '';
    pageNumberInput =
      typeof existingQuote?.pageNumber === 'number' ? String(existingQuote.pageNumber) : '';
    chapter = existingQuote?.chapter ?? '';
    note = existingQuote?.note ?? '';
    error = null;
    lastHydratedQuoteId = hydratedQuoteID;
  }

  function parsePageNumber(value: string | number): number | undefined {
    const trimmed = String(value ?? '').trim();
    if (!trimmed) {
      return undefined;
    }
    if (!/^\d+$/.test(trimmed)) {
      return NaN;
    }
    const parsed = Number.parseInt(trimmed, 10);
    return parsed > 0 ? parsed : NaN;
  }

  async function handleSubmit() {
    const trimmedQuoteText = quoteText.trim();
    if (!trimmedQuoteText) {
      error = 'Quote text is required.';
      return;
    }

    const pageNumber = parsePageNumber(pageNumberInput);
    if (Number.isNaN(pageNumber)) {
      error = 'Page number must be a positive integer.';
      return;
    }

    error = null;
    isSubmitting = true;

    try {
      await onSubmit({
        postId,
        quoteText: trimmedQuoteText,
        pageNumber,
        chapter: chapter.trim() || undefined,
        note: note.trim() || undefined,
        quoteId: existingQuote?.id,
      });

      if (!existingQuote) {
        quoteText = '';
        pageNumberInput = '';
        chapter = '';
        note = '';
      }
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to save quote.';
    } finally {
      isSubmitting = false;
    }
  }
</script>

<form on:submit|preventDefault={handleSubmit} class="space-y-3 rounded-lg border border-gray-200 bg-gray-50 p-3">
  <div class="space-y-1">
    <label for="quote-text" class="text-sm font-medium text-gray-700">Quote text</label>
    <textarea
      id="quote-text"
      rows="3"
      bind:value={quoteText}
      required
      disabled={isSubmitting}
      class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm text-gray-900 focus:border-primary focus:ring-2 focus:ring-primary/30 disabled:cursor-not-allowed disabled:bg-gray-100"
      placeholder="Add the quote..."
      data-testid="quote-form-text"
    />
  </div>

  <div class="grid gap-3 sm:grid-cols-2">
    <div class="space-y-1">
      <label for="quote-page-number" class="text-sm font-medium text-gray-700">Page number (optional)</label>
      <input
        id="quote-page-number"
        type="number"
        min="1"
        step="1"
        inputmode="numeric"
        bind:value={pageNumberInput}
        disabled={isSubmitting}
        class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm text-gray-900 focus:border-primary focus:ring-2 focus:ring-primary/30 disabled:cursor-not-allowed disabled:bg-gray-100"
        data-testid="quote-form-page-number"
      />
    </div>
    <div class="space-y-1">
      <label for="quote-chapter" class="text-sm font-medium text-gray-700">Chapter (optional)</label>
      <input
        id="quote-chapter"
        type="text"
        bind:value={chapter}
        disabled={isSubmitting}
        class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm text-gray-900 focus:border-primary focus:ring-2 focus:ring-primary/30 disabled:cursor-not-allowed disabled:bg-gray-100"
        placeholder="Chapter 7"
        data-testid="quote-form-chapter"
      />
    </div>
  </div>

  <div class="space-y-1">
    <label for="quote-note" class="text-sm font-medium text-gray-700">Note (optional)</label>
    <textarea
      id="quote-note"
      rows="2"
      bind:value={note}
      disabled={isSubmitting}
      class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm text-gray-900 focus:border-primary focus:ring-2 focus:ring-primary/30 disabled:cursor-not-allowed disabled:bg-gray-100"
      placeholder="Why this quote stood out..."
      data-testid="quote-form-note"
    />
  </div>

  {#if error}
    <p class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700" data-testid="quote-form-error">
      {error}
    </p>
  {/if}

  <div class="flex items-center justify-end gap-2">
    <button
      type="button"
      on:click={onCancel}
      disabled={isSubmitting}
      class="rounded-lg border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50"
    >
      Cancel
    </button>
    <button
      type="submit"
      disabled={isSubmitting}
      class="rounded-lg bg-primary px-3 py-1.5 text-sm font-medium text-white hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
    >
      {#if isSubmitting}
        Saving...
      {:else if existingQuote}
        Save Quote
      {:else}
        Add Quote
      {/if}
    </button>
  </div>
</form>
