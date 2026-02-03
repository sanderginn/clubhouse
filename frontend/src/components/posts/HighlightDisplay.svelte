<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { Highlight } from '../../stores/postStore';
  import { formatHighlightTimestamp } from '../../lib/highlights';

  export let highlights: Highlight[] = [];
  export let onSeek: ((timestamp: number) => Promise<boolean> | boolean) | undefined = undefined;
  export let unsupportedMessage: string = 'Seeking not supported for this embed.';

  let activeTimestamp: number | null = null;
  let feedbackMessage: string | null = null;
  let feedbackTimeout: ReturnType<typeof setTimeout> | null = null;

  const resetFeedback = () => {
    if (feedbackTimeout) {
      clearTimeout(feedbackTimeout);
      feedbackTimeout = null;
    }
  };

  const setFeedback = (message: string) => {
    resetFeedback();
    feedbackMessage = message;
    feedbackTimeout = setTimeout(() => {
      feedbackMessage = null;
      activeTimestamp = null;
      feedbackTimeout = null;
    }, 2000);
  };

  const handleSeek = async (timestamp: number) => {
    if (!onSeek) return;

    try {
      const result = await onSeek(timestamp);
      if (result) {
        activeTimestamp = timestamp;
        setFeedback(`Jumped to ${formatHighlightTimestamp(timestamp)}.`);
        return;
      }
      setFeedback(unsupportedMessage);
    } catch {
      setFeedback('Unable to seek right now.');
    }
  };

  onDestroy(() => {
    resetFeedback();
  });
</script>

{#if highlights?.length}
  <div class="flex flex-wrap gap-2" aria-label="Highlights">
    {#each highlights as highlight (highlight.timestamp + '-' + (highlight.label ?? ''))}
      {#if onSeek}
        <button
          type="button"
          class={`inline-flex items-center rounded-full px-2 py-1 text-xs transition-colors ${
            activeTimestamp === highlight.timestamp
              ? 'bg-blue-100 text-blue-800'
              : 'bg-gray-100 text-gray-700 hover:bg-blue-50 hover:text-blue-800'
          }`}
          on:click={() => handleSeek(highlight.timestamp)}
          aria-pressed={activeTimestamp === highlight.timestamp ? 'true' : 'false'}
        >
          <span class="font-medium">{formatHighlightTimestamp(highlight.timestamp)}</span>
          {#if highlight.label}
            <span class="ml-1 text-gray-600">{highlight.label}</span>
          {/if}
        </button>
      {:else}
        <span class="inline-flex items-center rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-700">
          <span class="font-medium text-gray-800">{formatHighlightTimestamp(highlight.timestamp)}</span>
          {#if highlight.label}
            <span class="ml-1 text-gray-600">{highlight.label}</span>
          {/if}
        </span>
      {/if}
    {/each}
  </div>
  {#if feedbackMessage}
    <p class="mt-2 text-xs text-gray-500" role="status" aria-live="polite">
      {feedbackMessage}
    </p>
  {/if}
{/if}
