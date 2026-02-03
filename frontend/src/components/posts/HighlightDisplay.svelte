<script lang="ts">
  import type { Highlight } from '../../stores/postStore';
  import { formatHighlightTimestamp } from '../../lib/highlights';

  export let highlights: Highlight[] = [];
</script>

{#if highlights?.length}
  <div class="flex flex-wrap gap-2" aria-label="Highlights">
    {#each highlights as highlight (highlight.timestamp + '-' + (highlight.label ?? ''))}
      <span class="inline-flex items-center rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-700">
        <span class="font-medium text-gray-800">{formatHighlightTimestamp(highlight.timestamp)}</span>
        {#if highlight.label}
          <span class="ml-1 text-gray-600">{highlight.label}</span>
        {/if}
      </span>
    {/each}
  </div>
{/if}
