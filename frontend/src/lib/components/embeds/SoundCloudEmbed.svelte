<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { EmbedController } from '../../embeds/controller';
  import { loadSoundCloudApi } from '../../embeds/soundcloudApi';

  export let embedUrl: string;
  export let height: number | undefined = undefined;
  export let title: string | undefined = undefined;
  export let onReady: ((controller: EmbedController) => void) | undefined = undefined;

  const fallbackHeight = 166;
  const readyEvent = 'ready';

  let iframeElement: HTMLIFrameElement | null = null;
  let widget: {
    seekTo: (milliseconds: number) => void;
    play: () => void;
    bind: (event: string, handler: () => void) => void;
    unbind: (event: string) => void;
  } | null = null;

  $: resolvedHeight = height && height > 0 ? height : fallbackHeight;
  $: iframeTitle = title ? `SoundCloud player: ${title}` : 'SoundCloud player';

  onMount(async () => {
    if (typeof window === 'undefined' || !iframeElement) return;

    try {
      const SC = await loadSoundCloudApi();
      widget = SC.Widget(iframeElement);
      widget.bind(readyEvent, () => {
        if (!onReady) return;
        onReady({
          provider: 'soundcloud',
          supportsSeeking: true,
          seekTo: async (seconds: number) => {
            if (!widget) return;
            widget.seekTo(seconds * 1000);
            widget.play();
          },
        });
      });
    } catch {
      // Ignore failed initialization; controller won't be provided.
    }
  });

  onDestroy(() => {
    if (widget?.unbind) {
      widget.unbind(readyEvent);
    }
  });
</script>

<div class="mt-3 rounded-lg border border-gray-200 overflow-hidden bg-white">
  <iframe
    src={embedUrl}
    title={iframeTitle}
    height={resolvedHeight}
    class="w-full"
    loading="lazy"
    sandbox="allow-scripts allow-same-origin allow-presentation"
    allow="autoplay"
    style="border: 0;"
    data-testid="soundcloud-embed"
    bind:this={iframeElement}
  ></iframe>
</div>
