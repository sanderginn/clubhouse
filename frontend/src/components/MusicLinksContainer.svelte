<script lang="ts">
  import { onDestroy } from 'svelte';
  import {
    activeSection,
    sectionLinks,
    isLoadingSectionLinks,
    hasMoreSectionLinks,
    sectionLinksStore,
  } from '../stores';
  import { loadSectionLinks, loadMoreSectionLinks } from '../stores/sectionLinksFeedStore';
  import type { SectionLink } from '../stores/sectionLinksStore';
  import { looksLikeImageUrl } from '../services/linkUtils';
  import RelativeTime from './RelativeTime.svelte';

  let isExpanded = true;
  let lastSectionId: string | null = null;

  $: isMusicSection = $activeSection?.type === 'music';
  $: linkCount = $sectionLinks.length;

  $: if (isMusicSection && $activeSection?.id && $activeSection.id !== lastSectionId) {
    lastSectionId = $activeSection.id;
    isExpanded = true;
    loadSectionLinks($activeSection.id);
  }

  $: if (!isMusicSection && lastSectionId) {
    lastSectionId = null;
    sectionLinksStore.reset();
  }

  onDestroy(() => {
    sectionLinksStore.reset();
  });

  function toggleExpanded() {
    isExpanded = !isExpanded;
  }

  function getThumbnailUrl(link: SectionLink): string | null {
    const metadata = link.metadata;
    const metadataType = typeof metadata?.type === 'string' ? metadata.type.toLowerCase() : '';
    const isImage =
      metadataType === 'image' || metadataType.startsWith('image/') || looksLikeImageUrl(link.url);
    return metadata?.image ?? (isImage ? link.url : null);
  }

  function getProviderLabel(link: SectionLink): string {
    const provider = link.metadata?.provider?.trim();
    if (provider) return provider;
    try {
      return new URL(link.url).hostname.replace(/^www\./, '');
    } catch {
      return 'Link';
    }
  }

  function getProviderInitial(link: SectionLink): string {
    const label = getProviderLabel(link);
    return label.charAt(0).toUpperCase();
  }

  function getTitle(link: SectionLink): string {
    return link.metadata?.title?.trim() || link.metadata?.description?.trim() || link.url;
  }

  async function handleLoadMore() {
    if ($isLoadingSectionLinks) return;
    await loadMoreSectionLinks();
  }
</script>

{#if isMusicSection}
  <div class="bg-white rounded-lg shadow-sm border border-gray-200">
    <button
      type="button"
      class="w-full flex items-center justify-between px-4 py-3"
      on:click={toggleExpanded}
      aria-expanded={isExpanded}
    >
      <div class="flex items-center gap-2">
        <svg
          class={`h-4 w-4 text-gray-500 transition-transform ${isExpanded ? 'rotate-90' : ''}`}
          viewBox="0 0 20 20"
          fill="currentColor"
          aria-hidden="true"
        >
          <path
            fill-rule="evenodd"
            d="M7.21 4.21a.75.75 0 011.06.02l4 4a.75.75 0 010 1.06l-4 4a.75.75 0 11-1.06-1.06L10.69 8 7.23 4.27a.75.75 0 01-.02-1.06z"
            clip-rule="evenodd"
          />
        </svg>
        <span class="text-sm font-semibold text-gray-900">Recent Music Links</span>
        <span
          class="inline-flex items-center rounded-full bg-indigo-50 px-2 py-0.5 text-xs font-semibold text-indigo-700"
        >
          {linkCount}
        </span>
      </div>
    </button>

    {#if isExpanded}
      <div class="border-t border-gray-200 px-4 pb-4 pt-3 space-y-3">
        {#if $sectionLinksStore.error}
          <div class="bg-amber-50 border border-amber-200 rounded-lg p-3 text-sm text-amber-700">
            {$sectionLinksStore.error}
          </div>
        {/if}

        {#if $isLoadingSectionLinks && $sectionLinks.length === 0}
          <div class="flex items-center gap-2 text-sm text-gray-500">
            <svg
              class="animate-spin h-4 w-4"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              />
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              />
            </svg>
            <span>Loading links...</span>
          </div>
        {:else if $sectionLinks.length === 0}
          <p class="text-sm text-gray-500">No music links yet.</p>
        {:else}
          <ul class="space-y-2 max-h-64 overflow-y-auto pr-1">
            {#each $sectionLinks as link (link.id || link.url)}
              {@const thumbnailUrl = getThumbnailUrl(link)}
              {@const title = getTitle(link)}
              <li>
                <a
                  href={link.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="flex items-center gap-3 rounded-lg border border-gray-100 bg-gray-50 p-2 hover:bg-gray-100"
                >
                  <div
                    class="h-10 w-10 flex-shrink-0 overflow-hidden rounded-md bg-gray-200 flex items-center justify-center"
                  >
                    {#if thumbnailUrl}
                      <img
                        src={thumbnailUrl}
                        alt={link.metadata?.title || 'Link thumbnail'}
                        class="h-full w-full object-cover"
                        loading="lazy"
                      />
                    {:else}
                      <span class="text-sm font-semibold text-gray-600">
                        {getProviderInitial(link)}
                      </span>
                    {/if}
                  </div>

                  <div class="min-w-0 flex-1">
                    <div class="text-sm font-semibold text-gray-900 truncate">{title}</div>
                    <div class="flex items-center gap-2 text-xs text-gray-500">
                      <span class="truncate">@{link.username}</span>
                      <span class="text-gray-300">•</span>
                      <RelativeTime
                        dateString={link.createdAt}
                        className="text-xs text-gray-400"
                      />
                    </div>
                  </div>

                  <svg
                    class="h-4 w-4 text-gray-400"
                    viewBox="0 0 20 20"
                    fill="currentColor"
                    aria-hidden="true"
                  >
                    <path
                      d="M12.293 2.293a1 1 0 011.414 0l4 4a1 1 0 010 1.414l-7 7a1 1 0 01-.707.293H6a1 1 0 01-1-1v-3a1 1 0 01.293-.707l7-7z"
                    />
                    <path d="M5 13v3h3l-3-3z" />
                  </svg>
                </a>
              </li>
            {/each}
          </ul>
        {/if}

        {#if $hasMoreSectionLinks}
          <button
            type="button"
            class="w-full rounded-md border border-gray-200 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
            on:click={handleLoadMore}
            disabled={$isLoadingSectionLinks}
          >
            {#if $isLoadingSectionLinks}
              Loading...
            {:else}
              Load more
            {/if}
          </button>
        {/if}
      </div>
    {/if}
  </div>
{/if}
