<script lang="ts">
  export let onSelect: (emoji: string) => void;
  export let onClose: (() => void) | undefined = undefined;

  const emojis = ['👍', '❤️', '😂', '🎉', '😮', '😢', '🔥', '👀'];
</script>

<div class="flex items-center gap-1 rounded-full border border-gray-200 bg-white px-2 py-1 shadow-sm">
  {#each emojis as emoji}
    <button
      type="button"
      class="h-7 w-7 rounded-full text-sm transition hover:bg-gray-100"
      on:click={() => onSelect(emoji)}
      aria-label={`React with ${emoji}`}
    >
      {emoji}
    </button>
  {/each}
  {#if onClose}
    <button
      type="button"
      class="ml-1 h-7 w-7 rounded-full text-xs text-gray-400 transition hover:bg-gray-100 hover:text-gray-600"
      on:click={onClose}
      aria-label="Close emoji picker"
    >
      ✕
    </button>
  {/if}
</div>
