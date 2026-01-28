<script lang="ts">
  import EmojiPicker from './EmojiPicker.svelte';
  import { api } from '../../services/api';
  import type { ApiReactionGroup, ApiReactionUser } from '../../services/api';

  export let reactionCounts: Record<string, number> = {};
  export let userReactions: Set<string> = new Set();
  export let onToggle: (emoji: string) => Promise<void> | void;
  export let postId: string | null = null;
  export let commentId: string | null = null;

  let showPicker = false;
  let pendingEmoji: string | null = null;
  let showTooltip = false;
  let tooltipLoading = false;
  let tooltipError: string | null = null;
  let tooltipReactions: ApiReactionGroup[] = [];
  let lastCountsKey = '';
  let tooltipTimeout: ReturnType<typeof setTimeout> | null = null;

  $: orderedReactions = Object.entries(reactionCounts).sort((a, b) => b[1] - a[1]);
  $: countsKey = orderedReactions
    .map(([emoji, count]) => `${emoji}:${count}`)
    .join('|');
  $: tooltipReady = !!postId || !!commentId;

  async function handleToggle(emoji: string) {
    if (pendingEmoji) return;
    pendingEmoji = emoji;
    try {
      await onToggle(emoji);
    } finally {
      pendingEmoji = null;
    }
  }

  function hideTooltip() {
    showTooltip = false;
    tooltipTimeout && clearTimeout(tooltipTimeout);
    tooltipTimeout = null;
  }

  function normalizeUsers(users: ApiReactionUser[]): ApiReactionUser[] {
    const seen = new Set<string>();
    const unique: ApiReactionUser[] = [];
    for (const user of users) {
      if (!seen.has(user.id)) {
        seen.add(user.id);
        unique.push(user);
      }
    }
    return unique;
  }

  function normalizeGroups(groups: ApiReactionGroup[]): ApiReactionGroup[] {
    return groups
      .map((group) => ({
        emoji: group.emoji,
        users: normalizeUsers(group.users ?? []),
      }))
      .filter((group) => group.users.length > 0);
  }

  async function loadTooltipData(force = false) {
    if (!tooltipReady) return;
    if (!force && tooltipReactions.length > 0 && countsKey === lastCountsKey) return;
    tooltipLoading = true;
    tooltipError = null;
    try {
      const response = postId
        ? await api.getPostReactions(postId)
        : await api.getCommentReactions(commentId ?? '');
      tooltipReactions = normalizeGroups(response.reactions ?? []);
      lastCountsKey = countsKey;
    } catch (error) {
      tooltipError =
        error instanceof Error ? error.message : 'Failed to load reactions';
    } finally {
      tooltipLoading = false;
    }
  }

  function showTooltipWithDelay() {
    if (!tooltipReady || orderedReactions.length === 0) return;
    if (tooltipTimeout) {
      clearTimeout(tooltipTimeout);
    }
    tooltipTimeout = setTimeout(async () => {
      showTooltip = true;
      await loadTooltipData();
    }, 150);
  }
</script>

<div class="flex flex-wrap items-center gap-2">
  <div
    class="relative"
    role="group"
    on:mouseenter={showTooltipWithDelay}
    on:mouseleave={hideTooltip}
    on:focusin={showTooltipWithDelay}
    on:focusout={hideTooltip}
  >
    <div class="flex flex-wrap items-center gap-2">
      {#each orderedReactions as [emoji, count]}
        <button
          type="button"
          disabled={pendingEmoji !== null}
          class={`inline-flex items-center gap-1 rounded-full border px-2 py-1 text-xs transition ${
            userReactions.has(emoji)
              ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
              : 'border-gray-200 bg-white text-gray-600 hover:border-gray-300'
          } ${pendingEmoji === emoji ? 'opacity-50 cursor-wait' : ''}`}
          on:click={() => handleToggle(emoji)}
          aria-label={`React with ${emoji}`}
        >
          <span>{emoji}</span>
          <span>{count}</span>
        </button>
      {/each}
    </div>

    {#if showTooltip && tooltipReady && orderedReactions.length > 0}
      <div
        class="absolute left-0 top-full z-20 mt-2 w-64 rounded-lg border border-gray-200 bg-white p-3 text-xs shadow-lg"
        role="tooltip"
      >
        {#if tooltipLoading}
          <p class="text-gray-500">Loading reactions...</p>
        {:else if tooltipError}
          <p class="text-red-500">{tooltipError}</p>
        {:else if tooltipReactions.length === 0}
          <p class="text-gray-500">No reactions yet.</p>
        {:else}
          <div class="space-y-2">
            {#each tooltipReactions as reaction}
              <div>
                <div class="font-medium text-gray-700">{reaction.emoji}</div>
                <div class="mt-1 flex flex-wrap gap-2">
                  {#each reaction.users as user}
                    <div class="flex items-center gap-2">
                      {#if user.profile_picture_url}
                        <img
                          src={user.profile_picture_url}
                          alt={user.username}
                          class="h-5 w-5 rounded-full object-cover"
                        />
                      {:else}
                        <div class="h-5 w-5 rounded-full bg-gray-200 flex items-center justify-center text-[10px] text-gray-500">
                          {user.username?.charAt(0).toUpperCase() || '?'}
                        </div>
                      {/if}
                      <span class="text-gray-600">{user.username}</span>
                    </div>
                  {/each}
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  </div>

  <div class="relative">
    <button
      type="button"
      disabled={pendingEmoji !== null}
      class="inline-flex items-center gap-1 rounded-full border border-gray-200 bg-white px-2 py-1 text-xs text-gray-600 transition hover:border-gray-300 disabled:opacity-50"
      on:click={() => (showPicker = !showPicker)}
      aria-label="Add reaction"
    >
      <span>➕</span>
      <span>React</span>
    </button>
    {#if showPicker}
      <div class="absolute left-0 top-9 z-10">
        <EmojiPicker
          onSelect={(emoji) => {
            showPicker = false;
            handleToggle(emoji);
          }}
          onClose={() => (showPicker = false)}
        />
      </div>
    {/if}
  </div>
</div>
