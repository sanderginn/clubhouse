<script lang="ts">
  import type { BookStats } from '../../stores/postStore';
  import BookshelfSaveButton from './BookshelfSaveButton.svelte';
  import ReadButton from './ReadButton.svelte';

  export let postId: string;
  export let bookStats: BookStats;

  const clampCount = (value: number) => Math.max(0, Number.isFinite(value) ? value : 0);
  const clampRating = (value: number) => Math.min(5, Math.max(0, value));

  $: normalizedBookshelfCount = clampCount(bookStats?.bookshelfCount ?? 0);
  $: normalizedReadCount = clampCount(bookStats?.readCount ?? 0);
  $: normalizedAverageRating =
    typeof bookStats?.averageRating === 'number' && Number.isFinite(bookStats.averageRating)
      ? clampRating(bookStats.averageRating)
      : null;
  $: hasAverageRating = normalizedAverageRating !== null && normalizedAverageRating > 0;
  $: formattedAverageRating =
    normalizedAverageRating === null
      ? ''
      : Number.isInteger(normalizedAverageRating)
        ? normalizedAverageRating.toFixed(0)
        : normalizedAverageRating.toFixed(1);
</script>

<div
  class="flex flex-col gap-3 rounded-xl border border-gray-200 bg-white px-3 py-2 text-xs text-gray-600 sm:flex-row sm:items-center sm:justify-between"
  data-testid="book-stats-bar"
>
  <div class="flex flex-wrap items-center gap-3" data-testid="book-stats-list">
    <div class="inline-flex items-center gap-1.5" data-testid="book-bookshelf-stat">
      <span class="inline-flex h-6 w-6 items-center justify-center rounded-full bg-indigo-50 text-indigo-700">
        🔖
      </span>
      <span class="font-semibold text-gray-800" data-testid="book-bookshelf-count">{normalizedBookshelfCount}</span>
      <span>saved</span>
    </div>

    <div class="inline-flex items-center gap-1.5" data-testid="book-read-stat">
      <span class="inline-flex h-6 w-6 items-center justify-center rounded-full bg-emerald-50 text-emerald-700">
        📖
      </span>
      <span class="font-semibold text-gray-800" data-testid="book-read-count">{normalizedReadCount}</span>
      <span>read</span>
    </div>

    {#if hasAverageRating}
      <div class="inline-flex items-center gap-1.5" data-testid="book-average-rating">
        <span class="inline-flex h-6 w-6 items-center justify-center rounded-full bg-amber-50 text-amber-700">
          ★
        </span>
        <span class="font-semibold text-gray-800" data-testid="book-average-rating-value">
          {formattedAverageRating}
        </span>
      </div>
    {/if}
  </div>

  <div class="flex flex-wrap items-center gap-2" data-testid="book-action-buttons">
    <BookshelfSaveButton {postId} {bookStats} />
    <ReadButton {postId} {bookStats} />
  </div>
</div>
