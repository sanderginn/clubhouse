<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { Highlight } from '../../stores/postStore';
  import { formatHighlightTimestamp } from '../../lib/highlights';
  import { api } from '../../services/api';
  import type { ApiReactionUser } from '../../services/api';

  export let highlights: Highlight[] = [];
  export let postId: string | null = null;
  export let onSeek: ((timestamp: number) => Promise<boolean> | boolean) | undefined = undefined;
  export let onToggleReaction: ((highlight: Highlight) => Promise<void> | void) | undefined = undefined;
  export let unsupportedMessage: string = 'Seeking not supported for this embed.';

  let activeTimestamp: number | null = null;
  let feedbackMessage: string | null = null;
  let feedbackTimeout: ReturnType<typeof setTimeout> | null = null;
  let reactionPulseId: string | null = null;
  let reactionPulseTimeout: ReturnType<typeof setTimeout> | null = null;
  let tooltipHighlightId: string | null = null;
  let tooltipUsers: ApiReactionUser[] = [];
  let tooltipLoading = false;
  let tooltipError: string | null = null;
  let tooltipTimeout: ReturnType<typeof setTimeout> | null = null;

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

  const resetTooltip = () => {
    if (tooltipTimeout) {
      clearTimeout(tooltipTimeout);
      tooltipTimeout = null;
    }
  };

  const normalizeUsers = (users: ApiReactionUser[]): ApiReactionUser[] => {
    const seen = new Set<string>();
    const unique: ApiReactionUser[] = [];
    for (const user of users) {
      if (!seen.has(user.id)) {
        seen.add(user.id);
        unique.push(user);
      }
    }
    return unique;
  };

  const formatUserList = (users: ApiReactionUser[], maxNames = 3): string => {
    const names = users.map((user) => {
      const trimmed = user.username?.trim();
      return trimmed && trimmed.length > 0 ? trimmed : 'Unknown';
    });
    if (names.length <= maxNames) {
      return names.join(', ');
    }
    const remaining = names.length - maxNames;
    return `${names.slice(0, maxNames).join(', ')}, and ${remaining} other${remaining === 1 ? '' : 's'}`;
  };

  const loadTooltipData = async (highlightId: string) => {
    if (!postId) return;
    tooltipLoading = true;
    tooltipError = null;
    try {
      const response = await api.getHighlightReactions(postId, highlightId);
      const group = response.reactions?.[0];
      tooltipUsers = normalizeUsers(group?.users ?? []);
    } catch (error) {
      tooltipError = error instanceof Error ? error.message : 'Failed to load reactions';
      tooltipUsers = [];
    } finally {
      tooltipLoading = false;
    }
  };

  const showTooltipWithDelay = (highlightId: string) => {
    if (!postId || !highlightId) return;
    resetTooltip();
    tooltipTimeout = setTimeout(async () => {
      tooltipHighlightId = highlightId;
      await loadTooltipData(highlightId);
    }, 150);
  };

  const hideTooltip = () => {
    tooltipHighlightId = null;
    tooltipError = null;
    tooltipUsers = [];
    resetTooltip();
  };

  onDestroy(() => {
    resetFeedback();
    resetReactionPulse();
    resetTooltip();
  });
</script>

{#if highlights?.length}
  <div class="flex flex-wrap gap-2" aria-label="Highlights">
    {#each highlights as highlight (highlight.id ?? highlight.timestamp + '-' + (highlight.label ?? ''))}
      <div class="inline-flex items-center gap-1 rounded-full border border-gray-200 bg-white px-1.5 py-1 shadow-sm">
        {#if onSeek}
          <button
            type="button"
            class={`inline-flex items-center rounded-full px-2 py-1 text-xs transition-colors ${
              activeTimestamp === highlight.timestamp
                ? 'bg-blue-100 text-blue-800'
                : 'bg-gray-50 text-gray-700 hover:bg-blue-50 hover:text-blue-800'
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
          <span class="inline-flex items-center rounded-full bg-gray-50 px-2 py-1 text-xs text-gray-700">
            <span class="font-medium text-gray-800">{formatHighlightTimestamp(highlight.timestamp)}</span>
            {#if highlight.label}
              <span class="ml-1 text-gray-600">{highlight.label}</span>
            {/if}
          </span>
        {/if}
        {#if onToggleReaction}
          <div class="relative inline-flex">
            <button
              type="button"
              class={`inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-[11px] transition-transform duration-150 ${
                highlight.viewerReacted ? 'bg-rose-100 text-rose-700' : 'bg-gray-50 text-gray-600'
              } ${reactionPulseId === (highlight.id ?? `${highlight.timestamp}-${highlight.label ?? ''}`) ? 'scale-105' : ''}`}
              on:mouseenter={() => showTooltipWithDelay(highlight.id ?? '')}
              on:mouseleave={hideTooltip}
              on:focusin={() => showTooltipWithDelay(highlight.id ?? '')}
              on:focusout={hideTooltip}
              on:click={() => handleToggleReaction(highlight)}
              aria-pressed={highlight.viewerReacted ? 'true' : 'false'}
              aria-label={`Heart highlight ${formatHighlightTimestamp(highlight.timestamp)}${
                highlight.label ? ` ${highlight.label}` : ''
              }`}
            >
              <span class="text-sm leading-none">
                {highlight.viewerReacted ? '❤️' : '🤍'}
              </span>
              {#if (highlight.heartCount ?? 0) > 0}
                <span class="font-medium">{highlight.heartCount}</span>
              {/if}
            </button>
            {#if tooltipHighlightId === highlight.id}
              <div
                class="absolute left-0 top-full z-20 mt-2 w-56 rounded-lg border border-gray-200 bg-white p-3 text-xs shadow-lg"
                role="tooltip"
              >
                {#if tooltipLoading}
                  <p class="text-gray-500">Loading reactions...</p>
                {:else if tooltipError}
                  <p class="text-red-500">{tooltipError}</p>
                {:else if tooltipUsers.length === 0}
                  <p class="text-gray-500">No reactions yet.</p>
                {:else}
                  <div class="flex flex-wrap items-center gap-2 leading-snug">
                    <span class="text-sm text-gray-700">❤️</span>
                    <span class="min-w-0 text-gray-600 break-words">
                      {formatUserList(tooltipUsers)}
                    </span>
                  </div>
                {/if}
              </div>
            {/if}
          </div>
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
