<script lang="ts">
  import EmojiPicker from './EmojiPicker.svelte';

  export let reactionCounts: Record<string, number> = {};
  export let userReactions: Set<string> = new Set();
  export let onToggle: (emoji: string) => Promise<void> | void;

  let showPicker = false;
  let pendingEmoji: string | null = null;

  $: orderedReactions = Object.entries(reactionCounts).sort((a, b) => b[1] - a[1]);

  async function handleToggle(emoji: string) {
    if (pendingEmoji) return;
    pendingEmoji = emoji;
    try {
      await onToggle(emoji);
    } finally {
      pendingEmoji = null;
    }
  }
</script>

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
