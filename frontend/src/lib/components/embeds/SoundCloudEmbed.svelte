<script lang="ts">
  export let embedUrl: string;
  export let height: number | undefined = undefined;
  export let title: string | undefined = undefined;

  const fallbackHeight = 166;

  $: resolvedHeight = height && height > 0 ? height : fallbackHeight;
  $: iframeTitle = title ? `SoundCloud player: ${title}` : 'SoundCloud player';
</script>

<div class="mt-3 rounded-lg border border-gray-200 overflow-hidden bg-white">
  <iframe
    src={embedUrl}
    title={iframeTitle}
    height={resolvedHeight}
    class="w-full"
    loading="lazy"
    allow="autoplay"
    style="border: 0;"
    data-testid="soundcloud-embed"
  ></iframe>
</div>
