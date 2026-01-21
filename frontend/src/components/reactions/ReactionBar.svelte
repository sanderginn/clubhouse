<script lang="ts">
  import EmojiPicker from './EmojiPicker.svelte';

  export let reactionCounts: Record<string, number> = {};
  export let userReactions: Set<string> = new Set();
  export let onToggle: (emoji: string) => void;

  let showPicker = false;

  $: orderedReactions = Object.entries(reactionCounts).sort((a, b) => b[1] - a[1]);
</script>

<div class="flex flex-wrap items-center gap-2">
  {#each orderedReactions as [emoji, count]}
    <button
      type="button"
      class={`inline-flex items-center gap-1 rounded-full border px-2 py-1 text-xs transition ${
        userReactions.has(emoji)
          ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
          : 'border-gray-200 bg-white text-gray-600 hover:border-gray-300'
      }`}
      on:click={() => onToggle(emoji)}
      aria-label={`React with ${emoji}`}
    >
      <span>{emoji}</span>
      <span>{count}</span>
    </button>
  {/each}

  <div class="relative">
    <button
      type="button"
      class="inline-flex items-center gap-1 rounded-full border border-gray-200 bg-white px-2 py-1 text-xs text-gray-600 transition hover:border-gray-300"
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
            onToggle(emoji);
          }}
          onClose={() => (showPicker = false)}
        />
      </div>
    {/if}
  </div>
</div>
