<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { EmbedController } from '../../embeds/controller';
  import { loadYouTubeApi, type YouTubePlayer } from '../../embeds/youtubeApi';

  export let embedUrl: string;
  export let title: string = 'YouTube video';
  export let onReady: ((controller: EmbedController) => void) | undefined = undefined;

  let iframeElement: HTMLIFrameElement | null = null;
  let player: YouTubePlayer | null = null;

  const buildApiEmbedUrl = (value: string, origin?: string): string => {
    try {
      const url = new URL(value);
      url.searchParams.set('enablejsapi', '1');
      if (origin) {
        url.searchParams.set('origin', origin);
      }
      return url.toString();
    } catch {
      return value;
    }
  };

  $: apiEmbedUrl =
    typeof window === 'undefined' ? embedUrl : buildApiEmbedUrl(embedUrl, window.location.origin);

  onMount(async () => {
    if (typeof window === 'undefined' || !iframeElement) return;

    try {
      const ytApi = await loadYouTubeApi();
      player = new ytApi.Player(iframeElement, {
        events: {
          onReady: () => {
            if (!onReady) return;
            onReady({
              provider: 'youtube',
              supportsSeeking: true,
              seekTo: async (seconds: number) => {
                if (!player) return;
                player.seekTo(seconds, true);
                player.playVideo();
              },
            });
          },
        },
      });
    } catch {
      // Ignore failed initialization; controller won't be provided.
    }
  });

  onDestroy(() => {
    if (player && typeof player.destroy === 'function') {
      player.destroy();
    }
  });
</script>

<div class="relative w-full overflow-hidden rounded-lg bg-black" style="padding-top: 56.25%;">
  <iframe
    class="absolute inset-0 h-full w-full"
    src={apiEmbedUrl}
    title={title}
    sandbox="allow-scripts allow-same-origin allow-presentation"
    allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
    allowfullscreen
    referrerpolicy="strict-origin-when-cross-origin"
    data-testid="youtube-embed-frame"
    bind:this={iframeElement}
  ></iframe>
</div>
