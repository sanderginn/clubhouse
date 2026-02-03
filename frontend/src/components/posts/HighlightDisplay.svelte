<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { Highlight } from '../../stores/postStore';
  import { formatHighlightTimestamp } from '../../lib/highlights';

  export let highlights: Highlight[] = [];
  export let onSeek: ((timestamp: number) => Promise<boolean> | boolean) | undefined = undefined;
  export let onToggleReaction: ((highlight: Highlight) => Promise<void> | void) | undefined = undefined;
  export let unsupportedMessage: string = 'Seeking not supported for this embed.';

  let activeTimestamp: number | null = null;
  let feedbackMessage: string | null = null;
  let feedbackTimeout: ReturnType<typeof setTimeout> | null = null;
  let reactionPulseId: string | null = null;
  let reactionPulseTimeout: ReturnType<typeof setTimeout> | null = null;

  const resetFeedback = () => {
    if (feedbackTimeout) {
      clearTimeout(feedbackTimeout);
      feedbackTimeout = null;
    }
  };

  const resetReactionPulse = () => {
    if (reactionPulseTimeout) {
      clearTimeout(reactionPulseTimeout);
      reactionPulseTimeout = null;
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

  const handleToggleReaction = (highlight: Highlight) => {
    if (!onToggleReaction) return;
    const key = highlight.id ?? `${highlight.timestamp}-${highlight.label ?? ''}`;
    reactionPulseId = key;
    resetReactionPulse();
    reactionPulseTimeout = setTimeout(() => {
      reactionPulseId = null;
      reactionPulseTimeout = null;
    }, 450);
    onToggleReaction(highlight);
  };

  onDestroy(() => {
    resetFeedback();
    resetReactionPulse();
  });
</script>

{#if highlights?.length}
  <div class="flex flex-wrap gap-2" aria-label="Highlights">
    {#each highlights as highlight (highlight.id ?? highlight.timestamp + '-' + (highlight.label ?? ''))}
      <div class="inline-flex items-center gap-1">
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
        {#if onToggleReaction}
          <button
            type="button"
            class={`inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs transition-transform duration-150 ${
              highlight.viewerReacted ? 'bg-rose-100 text-rose-700' : 'bg-gray-100 text-gray-600'
            } ${reactionPulseId === (highlight.id ?? `${highlight.timestamp}-${highlight.label ?? ''}`) ? 'scale-110' : ''}`}
            on:click={() => handleToggleReaction(highlight)}
            aria-pressed={highlight.viewerReacted ? 'true' : 'false'}
            aria-label={`Heart highlight ${formatHighlightTimestamp(highlight.timestamp)}${
              highlight.label ? ` ${highlight.label}` : ''
            }`}
          >
            <span class="text-base leading-none">
              {highlight.viewerReacted ? '❤️' : '🤍'}
            </span>
            {#if (highlight.heartCount ?? 0) > 0}
              <span class="font-medium">{highlight.heartCount}</span>
            {/if}
          </button>
        {/if}
      </div>
    {/each}
  </div>
  {#if feedbackMessage}
    <p class="mt-2 text-xs text-gray-500" role="status" aria-live="polite">
      {feedbackMessage}
    </p>
  {/if}
{/if}
