<script lang="ts">
  import type { EmbedData } from '../../../stores/postStore';

  export let embed: EmbedData;
  export let linkUrl: string;
  export let title: string | undefined = undefined;

  $: playerHeight = embed.height ?? 120;
  $: playerTitle = title ? `${title} on Bandcamp` : 'Bandcamp player';
</script>

<div class="mt-3 space-y-2">
  <iframe
    src={embed.embedUrl}
    title={playerTitle}
    class="w-full rounded-lg border border-gray-200"
    style={`height: ${playerHeight}px;`}
    loading="lazy"
    allow="encrypted-media"
    sandbox="allow-same-origin allow-scripts allow-popups"
  ></iframe>
  <a
    href={linkUrl}
    target="_blank"
    rel="noopener noreferrer"
    class="inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 text-sm break-all"
  >
    <span>🔗</span>
    <span class="underline">{linkUrl}</span>
  </a>
</div>
