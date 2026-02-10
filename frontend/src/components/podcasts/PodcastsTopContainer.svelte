<script lang="ts">
  import { activeSection } from '../../stores';

  $: isPodcastSection = $activeSection?.type === 'podcast';
</script>

{#if isPodcastSection}
  <div
    class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm"
    data-testid="podcasts-top-container"
  >
    <h2 class="text-sm font-semibold text-gray-900">Podcast Spotlight</h2>
    <p class="mt-1 text-sm text-gray-600">
      Podcast highlights and episode tools will appear here for this section.
    </p>
  </div>
{/if}
