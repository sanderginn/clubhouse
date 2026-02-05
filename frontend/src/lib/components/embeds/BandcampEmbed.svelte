<script lang="ts">
  import type { EmbedData } from '../../../stores/postStore';

  export let embed: EmbedData;
  export let linkUrl: string;
  export let title: string | undefined = undefined;

  $: playerHeight = embed.height ?? 120;
  $: playerTitle = title ? `${title} on Bandcamp` : 'Bandcamp player';
</script>

<div class="mt-3 space-y-2">
  <div class="rounded-lg border border-gray-200 overflow-hidden bg-white">
    <iframe
      src={embed.embedUrl}
      title={playerTitle}
      class="block w-full"
      style={`height: ${playerHeight}px; border: 0;`}
      loading="lazy"
      sandbox="allow-scripts allow-same-origin allow-presentation"
    ></iframe>
  </div>
  <a
    href={linkUrl}
    target="_blank"
    rel="noopener noreferrer"
    class="inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 text-sm"
  >
    <span aria-hidden="true">↗</span>
    <span class="underline">Open on Bandcamp</span>
  </a>
</div>
