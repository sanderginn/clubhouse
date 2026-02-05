<script lang="ts">
  import { onMount } from 'svelte';
  import type { EmbedController } from '../../embeds/controller';

  export let embedUrl: string;
  export let title: string = 'YouTube video';
  export let onReady: ((controller: EmbedController) => void) | undefined = undefined;

  const isBrowser = typeof window !== 'undefined';
  let iframeEl: HTMLIFrameElement | null = null;
  let resolvedEmbedUrl = embedUrl;
  let embedOrigin = '*';

  const buildEmbedUrl = (url: string): string => {
    try {
      const parsed = new URL(url);
      parsed.searchParams.set('enablejsapi', '1');
      if (isBrowser) {
        parsed.searchParams.set('origin', window.location.origin);
      }
      return parsed.toString();
    } catch {
      return url;
    }
  };

  const resolveOrigin = (url: string): string => {
    try {
      return new URL(url).origin;
    } catch {
      return '*';
    }
  };

  $: resolvedEmbedUrl = buildEmbedUrl(embedUrl);
  $: embedOrigin = resolveOrigin(resolvedEmbedUrl);

  onMount(() => {
    if (!onReady) return;
    onReady({
      provider: 'youtube',
      supportsSeeking: true,
      seekTo: async (seconds: number) => {
        if (!iframeEl || !iframeEl.contentWindow) {
          throw new Error('YouTube player is not ready');
        }
        const safeSeconds = Math.max(0, Math.floor(seconds));
        const message = JSON.stringify({
          event: 'command',
          func: 'seekTo',
          args: [safeSeconds, true]
        });
        iframeEl.contentWindow.postMessage(message, embedOrigin);
      },
    });
  });
</script>

<div class="relative w-full overflow-hidden rounded-lg bg-black" style="padding-top: 56.25%;">
  <iframe
    class="absolute inset-0 h-full w-full"
    bind:this={iframeEl}
    src={resolvedEmbedUrl}
    title={title}
    sandbox="allow-scripts allow-same-origin allow-presentation"
    allow="fullscreen; web-share"
    referrerpolicy="strict-origin-when-cross-origin"
    data-testid="youtube-embed-frame"
  ></iframe>
</div>
