<script lang="ts">
  import type { EmbedData } from '../../../stores/postStore';

  export let embed: EmbedData;
  export let linkUrl: string;
  export let title: string | undefined = undefined;

  $: playerHeight = embed.height ?? 120;
  $: playerTitle = title ? `${title} on Bandcamp` : 'Bandcamp player';

  let iframeError = false;
</script>

<div class="mt-3 space-y-2 w-full max-w-[700px] mx-auto">
  <div class="overflow-hidden bg-white w-full">
    <iframe
      src={embed.embedUrl}
      title={playerTitle}
      class="block w-full"
      style={`height: ${playerHeight}px; border: 0;`}
      loading="lazy"
      sandbox="allow-scripts allow-same-origin allow-presentation"
      on:error={() => (iframeError = true)}
      on:load={() => (iframeError = false)}
    ></iframe>
  </div>

  {#if iframeError}
    <a
      href={linkUrl}
      target="_blank"
      rel="noopener noreferrer"
      class="inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 text-sm break-all"
    >
      <span>🔗</span>
      <span class="underline">{linkUrl}</span>
    </a>
  {/if}
</div>
