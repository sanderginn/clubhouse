<script lang="ts">
  const DEFAULT_HEIGHTS: Record<string, number> = {
    track: 152,
    album: 380,
    playlist: 380,
    artist: 380,
    show: 232,
    episode: 232,
  };

  export let embedUrl: string;
  export let height: number | undefined = undefined;
  export let title: string | undefined = undefined;

  const resolveTypeFromUrl = (value: string): string | undefined => {
    try {
      const parsed = new URL(value);
      const parts = parsed.pathname.split('/').filter(Boolean);
      const embedIndex = parts.indexOf('embed');
      if (embedIndex >= 0 && parts.length > embedIndex + 1) {
        return parts[embedIndex + 1];
      }
      return parts[0];
    } catch {
      return undefined;
    }
  };

  $: contentType = embedUrl ? resolveTypeFromUrl(embedUrl) : undefined;
  $: computedHeight =
    height ?? (contentType ? DEFAULT_HEIGHTS[contentType] : undefined) ?? DEFAULT_HEIGHTS.album;
  $: iframeTitle = title ?? (contentType ? `Spotify ${contentType}` : 'Spotify player');
</script>

<div class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
  <iframe
    src={embedUrl}
    title={iframeTitle}
    class="w-full"
    style={`height: ${computedHeight}px;`}
    frameborder="0"
    loading="lazy"
    sandbox="allow-scripts allow-same-origin allow-presentation"
    allow="fullscreen"
  ></iframe>
</div>
