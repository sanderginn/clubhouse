<script lang="ts">
  import { onMount } from 'svelte';
  import type { EmbedController } from '../../embeds/controller';

  export let embedUrl: string;
  export let title: string = 'YouTube video';
  export let onReady: ((controller: EmbedController) => void) | undefined = undefined;

  onMount(() => {
    if (!onReady) return;
    onReady({
      provider: 'youtube',
      supportsSeeking: false,
      seekTo: async () => {
        // YouTube embeds without the iframe API do not support programmatic seeking.
      },
    });
  });
</script>

<div class="relative w-full overflow-hidden rounded-lg bg-black" style="padding-top: 56.25%;">
  <iframe
    class="absolute inset-0 h-full w-full"
    src={embedUrl}
    title={title}
    sandbox="allow-scripts allow-same-origin allow-presentation"
    allow="fullscreen; web-share"
    referrerpolicy="strict-origin-when-cross-origin"
    data-testid="youtube-embed-frame"
  ></iframe>
</div>
