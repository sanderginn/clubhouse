<script lang="ts">
  import type { Highlight } from '../../stores/postStore';

  export let highlights: Highlight[] = [];

  const formatTimestamp = (seconds: number) => {
    const safeSeconds = Math.max(0, Math.floor(seconds));
    const minutes = Math.floor(safeSeconds / 60);
    const remainder = safeSeconds % 60;
    return `${minutes.toString().padStart(2, '0')}:${remainder.toString().padStart(2, '0')}`;
  };
</script>

{#if highlights?.length}
  <div class="flex flex-wrap gap-2" aria-label="Highlights">
    {#each highlights as highlight (highlight.timestamp + '-' + (highlight.label ?? ''))}
      <span class="inline-flex items-center rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-700">
        <span class="font-medium text-gray-800">{formatTimestamp(highlight.timestamp)}</span>
        {#if highlight.label}
          <span class="ml-1 text-gray-600">{highlight.label}</span>
        {/if}
      </span>
    {/each}
  </div>
{/if}
