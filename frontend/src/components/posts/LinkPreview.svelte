<script lang="ts">
  import type { LinkMetadata } from '../../stores/postStore';

  export let metadata: LinkMetadata;
  export let onRemove: (() => void) | undefined = undefined;
</script>

<div class="rounded-lg border border-gray-200 bg-gray-50 p-3">
  <div class="flex items-start gap-3">
    {#if metadata.image}
      <img
        src={metadata.image}
        alt={metadata.title || 'Link preview'}
        class="w-16 h-16 object-cover rounded flex-shrink-0"
      />
    {:else}
      <div class="w-16 h-16 bg-gray-200 rounded flex-shrink-0 flex items-center justify-center">
        <svg class="w-8 h-8 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"
          />
        </svg>
      </div>
    {/if}

    <div class="flex-1 min-w-0">
      {#if metadata.provider}
        <span class="text-xs text-gray-500 uppercase tracking-wide">
          {metadata.provider}
        </span>
      {/if}
      {#if metadata.title}
        <h4 class="text-sm font-medium text-gray-900 truncate">
          {metadata.title}
        </h4>
      {/if}
      {#if metadata.description}
        <p class="text-xs text-gray-600 line-clamp-2">{metadata.description}</p>
      {/if}
      <p class="text-xs text-gray-400 truncate mt-1">{metadata.url}</p>
    </div>

    {#if onRemove}
      <button
        type="button"
        on:click={onRemove}
        class="p-1 text-gray-400 hover:text-gray-600"
        aria-label="Remove link"
      >
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M6 18L18 6M6 6l12 12"
          />
        </svg>
      </button>
    {/if}
  </div>

  {#if $$slots.footer}
    <div class="mt-3">
      <slot name="footer" />
    </div>
  {/if}
</div>
