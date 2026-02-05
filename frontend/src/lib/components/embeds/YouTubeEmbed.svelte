<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { EmbedController } from '../../embeds/controller';

  export let embedUrl: string;
  export let title: string = 'YouTube video';
  export let onReady: ((controller: EmbedController) => void) | undefined = undefined;

  const isBrowser = typeof window !== 'undefined';
  let playerContainer: HTMLDivElement | null = null;

  type PlayerInstance = {
    seekTo: (seconds: number, allowSeekAhead?: boolean) => void;
    destroy: () => void;
  };

  let player: PlayerInstance | null = null;

  let videoId: string | null = null;
  let playerHost = 'https://www.youtube.com';

  type YTGlobal = Window & {
    YT?: { Player: new (element: HTMLElement, options: unknown) => PlayerInstance };
    onYouTubeIframeAPIReady?: () => void;
  };

  let apiReadyPromise: Promise<void> | null = null;

  const extractVideoId = (url: string): string | null => {
    try {
      const parsed = new URL(url);
      const parts = parsed.pathname.split('/').filter(Boolean);
      const embedIndex = parts.findIndex((part) => part === 'embed');
      if (embedIndex >= 0 && parts[embedIndex + 1]) {
        return parts[embedIndex + 1];
      }
      const v = parsed.searchParams.get('v');
      return v && v.length > 0 ? v : null;
    } catch {
      return null;
    }
  };

  const resolveHost = (url: string): string => {
    try {
      const host = new URL(url).host;
      if (host.includes('youtube-nocookie.com')) {
        return 'https://www.youtube-nocookie.com';
      }
    } catch {
      // ignore
    }
    return 'https://www.youtube.com';
  };

  const loadYouTubeAPI = (): Promise<void> => {
    if (!isBrowser) {
      return Promise.reject(new Error('YouTube API requires a browser environment'));
    }
    const win = window as YTGlobal;
    if (win.YT?.Player) {
      return Promise.resolve();
    }
    if (apiReadyPromise) {
      return apiReadyPromise;
    }
    apiReadyPromise = new Promise((resolve, reject) => {
      const previousReady = win.onYouTubeIframeAPIReady;
      win.onYouTubeIframeAPIReady = () => {
        if (typeof previousReady === 'function') {
          previousReady();
        }
        resolve();
      };

      const existing = document.querySelector('script[src="https://www.youtube.com/iframe_api"]');
      if (existing) {
        return;
      }

      const script = document.createElement('script');
      script.src = 'https://www.youtube.com/iframe_api';
      script.async = true;
      script.onerror = () => reject(new Error('Failed to load YouTube IFrame API'));
      document.head.appendChild(script);
    });
    return apiReadyPromise;
  };

  $: videoId = extractVideoId(embedUrl);
  $: playerHost = resolveHost(embedUrl);

  onMount(() => {
    if (!isBrowser || !videoId || !playerContainer) {
      return;
    }
    void (async () => {
      try {
        await loadYouTubeAPI();
      } catch {
        return;
      }
      const win = window as YTGlobal;
      if (!win.YT?.Player) return;
      player = new win.YT.Player(playerContainer, {
        videoId,
        host: playerHost,
        playerVars: {
          origin: window.location.origin,
          playsinline: 1,
        },
        events: {
          onReady: () => {
            onReady?.({
              provider: 'youtube',
              supportsSeeking: true,
              seekTo: async (seconds: number) => {
                if (!player) {
                  throw new Error('YouTube player is not ready');
                }
                const safeSeconds = Math.max(0, Math.floor(seconds));
                player.seekTo(safeSeconds, true);
              },
            });
          },
        },
      });
    })();
  });

  onDestroy(() => {
    if (player) {
      player.destroy();
      player = null;
    }
  });
</script>

<div class="relative w-full overflow-hidden rounded-lg bg-black" style="padding-top: 56.25%;">
  <div
    class="absolute inset-0 h-full w-full"
    bind:this={playerContainer}
    aria-label={title}
    data-testid="youtube-embed-frame"
  ></div>
</div>
