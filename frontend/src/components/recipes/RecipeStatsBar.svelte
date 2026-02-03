<script lang="ts">
  import RatingStars from './RatingStars.svelte';

  export let saveCount = 0;
  export let cookCount = 0;
  export let averageRating: number | null = null;
  export let showEmpty = false;

  const clampRating = (value: number) => Math.min(5, Math.max(0, value));

  $: normalizedSaveCount = Math.max(0, saveCount);
  $: normalizedCookCount = Math.max(0, cookCount);
  $: normalizedRating = averageRating === null ? null : clampRating(averageRating);
  $: hasSaves = normalizedSaveCount > 0;
  $: hasCooks = normalizedCookCount > 0;
  $: hasRating = normalizedRating !== null && normalizedRating > 0;
  $: hasStats = hasSaves || hasCooks || hasRating;
  $: formattedRating =
    normalizedRating === null
      ? ''
      : Number.isInteger(normalizedRating)
        ? normalizedRating.toFixed(0)
        : normalizedRating.toFixed(1);
</script>

{#if hasStats || showEmpty}
  <div
    class="flex flex-wrap items-center gap-4 rounded-xl border border-gray-200 bg-white px-3 py-2 text-xs text-gray-600"
    data-testid="recipe-stats-bar"
  >
    {#if hasStats}
      {#if hasSaves}
        <div class="inline-flex items-center gap-1.5">
          <span class="inline-flex h-6 w-6 items-center justify-center rounded-full bg-amber-50 text-amber-600">
            <svg
              viewBox="0 0 20 20"
              class="h-3.5 w-3.5"
              fill="currentColor"
              aria-hidden="true"
            >
              <path d="M5 2.5A2.5 2.5 0 0 0 2.5 5v12l7.5-3.5L17.5 17V5A2.5 2.5 0 0 0 15 2.5H5z" />
            </svg>
          </span>
          <span class="font-semibold text-gray-800">{normalizedSaveCount}</span>
          <span>saved</span>
        </div>
      {/if}

      {#if hasCooks}
        <div class="inline-flex items-center gap-1.5">
          <span class="inline-flex h-6 w-6 items-center justify-center rounded-full bg-emerald-50 text-emerald-600">
            <svg
              viewBox="0 0 24 24"
              class="h-3.5 w-3.5"
              fill="none"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <path d="M4 10h16" />
              <path d="M6 10l1.2 9h9.6l1.2-9" />
              <path d="M9 5h6" />
              <path d="M10 5v3" />
              <path d="M14 5v3" />
            </svg>
          </span>
          <span class="font-semibold text-gray-800">{normalizedCookCount}</span>
          <span>cooked</span>
        </div>
      {/if}

      {#if hasRating}
        <div class="inline-flex items-center gap-2">
          <div class="inline-flex items-center gap-1.5">
            <span class="inline-flex h-6 w-6 items-center justify-center rounded-full bg-blue-50 text-blue-600">
              <svg
                viewBox="0 0 20 20"
                class="h-3.5 w-3.5"
                fill="currentColor"
                aria-hidden="true"
              >
                <path d="M10 1.5l2.47 5.4 5.88.5-4.42 3.83 1.33 5.77L10 13.9 4.74 17l1.33-5.77L1.65 7.4l5.88-.5L10 1.5z" />
              </svg>
            </span>
            <span class="font-semibold text-gray-800">{formattedRating}</span>
            <span>avg</span>
          </div>
          <RatingStars value={normalizedRating ?? 0} readonly size="sm" ariaLabel="Average rating" />
        </div>
      {/if}
    {:else}
      <div class="inline-flex items-center gap-2 text-gray-500">
        <span class="inline-flex h-6 w-6 items-center justify-center rounded-full bg-gray-100 text-gray-500">
          <svg
            viewBox="0 0 20 20"
            class="h-3.5 w-3.5"
            fill="currentColor"
            aria-hidden="true"
          >
            <path d="M10 1.5l2.47 5.4 5.88.5-4.42 3.83 1.33 5.77L10 13.9 4.74 17l1.33-5.77L1.65 7.4l5.88-.5L10 1.5z" />
          </svg>
        </span>
        <span>No recipe stats yet</span>
      </div>
    {/if}
  </div>
{/if}
