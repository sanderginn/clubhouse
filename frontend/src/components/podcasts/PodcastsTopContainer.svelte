<script lang="ts">
  import { onDestroy } from 'svelte';
  import { activeSection, podcastStore } from '../../stores';
  import type { Link, PodcastMetadata, Post } from '../../stores/postStore';
  import type { RecentPodcastItem } from '../../services/api';
  import { buildStandaloneThreadHref, pushPath } from '../../services/routeNavigation';
  import RelativeTime from '../RelativeTime.svelte';

  type PodcastMode = 'recent' | 'saved';

  let activeMode: PodcastMode = 'recent';
  let lastSectionId: string | null = null;

  $: isPodcastSection = $activeSection?.type === 'podcast';
  $: sectionId = isPodcastSection && $activeSection?.id ? $activeSection.id : null;

  $: if (sectionId && sectionId !== lastSectionId) {
    lastSectionId = sectionId;
    activeMode = 'recent';
    podcastStore.loadRecentPodcasts(sectionId);
    podcastStore.loadSavedPodcasts(sectionId);
  }

  $: if (!sectionId && lastSectionId) {
    lastSectionId = null;
    podcastStore.reset();
  }

  onDestroy(() => {
    podcastStore.reset();
  });

  function setMode(mode: PodcastMode) {
    activeMode = mode;
  }

  function normalizeKind(kind: unknown): 'show' | 'episode' | null {
    if (typeof kind !== 'string') return null;
    const normalized = kind.trim().toLowerCase();
    if (normalized === 'show' || normalized === 'episode') {
      return normalized;
    }
    return null;
  }

  function getKindLabel(kind: 'show' | 'episode' | null): string {
    if (kind === 'show') return 'Show';
    if (kind === 'episode') return 'Episode';
    return 'Podcast';
  }

  function getKindBadgeClasses(kind: 'show' | 'episode' | null): string {
    if (kind === 'show') {
      return 'bg-blue-50 text-blue-700 border-blue-200';
    }
    if (kind === 'episode') {
      return 'bg-emerald-50 text-emerald-700 border-emerald-200';
    }
    return 'bg-gray-100 text-gray-700 border-gray-200';
  }

  function getDomain(url: string): string {
    try {
      return new URL(url).hostname.replace(/^www\./, '');
    } catch {
      return url;
    }
  }

  function getRecentTitle(item: RecentPodcastItem): string {
    const firstHighlight = item.podcast.highlightEpisodes?.[0]?.title?.trim();
    if (firstHighlight) {
      return firstHighlight;
    }
    const metadataTitle = item.title?.trim();
    if (metadataTitle) {
      return metadataTitle;
    }
    const kind = normalizeKind(item.podcast.kind);
    if (kind === 'show') {
      return 'Podcast show';
    }
    if (kind === 'episode') {
      return 'Podcast episode';
    }
    return 'Podcast link';
  }

  function getPodcastLink(post: Post): Link | null {
    if (!post.links || post.links.length === 0) {
      return null;
    }
    return (
      post.links.find((link) => {
        const metadataPodcast = link.metadata?.podcast as PodcastMetadata | undefined;
        return normalizeKind(metadataPodcast?.kind) !== null;
      }) ??
      post.links.find((link) => !!link.metadata?.podcast) ??
      post.links[0]
    );
  }

  function getSavedPostTitle(post: Post): string {
    const link = getPodcastLink(post);
    const metadataTitle = link?.metadata?.title?.trim();
    if (metadataTitle) {
      return metadataTitle;
    }
    const content = post.content?.trim();
    return content || 'Saved podcast';
  }

  function getSavedPostDomain(post: Post): string | null {
    const url = getPodcastLink(post)?.url;
    if (!url) {
      return null;
    }
    return getDomain(url);
  }

  function getSavedPostKind(post: Post): 'show' | 'episode' | null {
    const metadataPodcast = getPodcastLink(post)?.metadata?.podcast as PodcastMetadata | undefined;
    return normalizeKind(metadataPodcast?.kind);
  }

  function navigateToPost(postId: string) {
    const href = buildStandaloneThreadHref(postId);
    pushPath(href);
    if (typeof window !== 'undefined') {
      window.dispatchEvent(new PopStateEvent('popstate', { state: window.history.state }));
    }
  }

  async function retryCurrentMode() {
    if (!sectionId) return;
    if (activeMode === 'recent') {
      await podcastStore.loadRecentPodcasts(sectionId);
      return;
    }
    await podcastStore.loadSavedPodcasts(sectionId);
  }

  async function loadMoreCurrentMode() {
    if (activeMode === 'recent') {
      await podcastStore.loadMoreRecentPodcasts();
      return;
    }
    await podcastStore.loadMoreSavedPodcasts();
  }
</script>

{#if isPodcastSection}
  <section
    class="rounded-lg border border-gray-200 bg-white shadow-sm"
    data-testid="podcasts-top-container"
  >
    <div class="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
      <div>
        <h2 class="text-base font-semibold text-gray-900">Podcasts</h2>
        <p class="text-xs text-gray-500">Switch between recent shared links and your saved podcasts.</p>
      </div>
      <div class="inline-flex items-center gap-1 rounded-full bg-gray-100 p-1" role="tablist">
        <button
          type="button"
          role="tab"
          class={`rounded-full px-3 py-1 text-xs font-semibold transition-colors ${
            activeMode === 'recent'
              ? 'bg-white text-gray-900 shadow-sm'
              : 'text-gray-600 hover:text-gray-800'
          }`}
          aria-selected={activeMode === 'recent'}
          data-testid="podcasts-mode-recent"
          on:click={() => setMode('recent')}
        >
          Recent
        </button>
        <button
          type="button"
          role="tab"
          class={`rounded-full px-3 py-1 text-xs font-semibold transition-colors ${
            activeMode === 'saved'
              ? 'bg-white text-gray-900 shadow-sm'
              : 'text-gray-600 hover:text-gray-800'
          }`}
          aria-selected={activeMode === 'saved'}
          data-testid="podcasts-mode-saved"
          on:click={() => setMode('saved')}
        >
          Saved
        </button>
      </div>
    </div>

    <div class="border-t border-gray-200 px-4 pb-4 pt-3">
      {#if activeMode === 'recent'}
        {#if $podcastStore.recentError}
          <div
            class="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700"
            data-testid="podcasts-recent-error"
          >
            <p>{$podcastStore.recentError}</p>
            <button class="mt-2 text-xs font-semibold underline" type="button" on:click={retryCurrentMode}>
              Try again
            </button>
          </div>
        {:else if $podcastStore.isLoadingRecent && $podcastStore.recentItems.length === 0}
          <p class="text-sm text-gray-500" data-testid="podcasts-recent-loading">Loading recent podcasts...</p>
        {:else if $podcastStore.recentItems.length === 0}
          <p class="text-sm text-gray-500" data-testid="podcasts-recent-empty">
            No podcast links shared yet.
          </p>
        {:else}
          <ul class="space-y-2" data-testid="podcasts-recent-list">
            {#each $podcastStore.recentItems as item (item.linkId)}
              {@const kind = normalizeKind(item.podcast.kind)}
              <li data-testid={`podcasts-recent-item-${item.linkId}`}>
                <a
                  href={item.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="flex items-start justify-between gap-3 rounded-md border border-gray-100 bg-gray-50 p-3 hover:bg-gray-100"
                >
                  <div class="min-w-0">
                    <p class="truncate text-sm font-semibold text-gray-900">{getRecentTitle(item)}</p>
                    <p class="truncate text-xs text-gray-500">{getDomain(item.url)}</p>
                    <div class="mt-1 flex items-center gap-2 text-xs text-gray-500">
                      <span>@{item.username}</span>
                      <span class="text-gray-300">•</span>
                      <RelativeTime dateString={item.linkCreatedAt} className="text-xs text-gray-400" />
                    </div>
                  </div>
                  <span
                    class={`inline-flex flex-shrink-0 items-center rounded-full border px-2 py-0.5 text-xs font-semibold ${getKindBadgeClasses(kind)}`}
                  >
                    {getKindLabel(kind)}
                  </span>
                </a>
              </li>
            {/each}
          </ul>
        {/if}

        {#if $podcastStore.recentHasMore}
          <button
            class="mt-3 w-full rounded-md border border-gray-200 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
            type="button"
            data-testid="podcasts-recent-load-more"
            on:click={loadMoreCurrentMode}
            disabled={$podcastStore.isLoadingRecent}
          >
            {#if $podcastStore.isLoadingRecent}
              Loading...
            {:else}
              Load more
            {/if}
          </button>
        {/if}
      {:else}
        {#if $podcastStore.error}
          <div
            class="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700"
            data-testid="podcasts-saved-error"
          >
            <p>{$podcastStore.error}</p>
            <button class="mt-2 text-xs font-semibold underline" type="button" on:click={retryCurrentMode}>
              Try again
            </button>
          </div>
        {:else if $podcastStore.isLoadingSaved && $podcastStore.savedPosts.length === 0}
          <p class="text-sm text-gray-500" data-testid="podcasts-saved-loading">Loading saved podcasts...</p>
        {:else if $podcastStore.savedPosts.length === 0}
          <p class="text-sm text-gray-500" data-testid="podcasts-saved-empty">
            You have not saved any podcasts in this section yet.
          </p>
        {:else}
          <ul class="space-y-2" data-testid="podcasts-saved-list">
            {#each $podcastStore.savedPosts as post (post.id)}
              {@const kind = getSavedPostKind(post)}
              <li data-testid={`podcasts-saved-item-${post.id}`}>
                <button
                  type="button"
                  class="flex w-full items-start justify-between gap-3 rounded-md border border-gray-100 bg-gray-50 p-3 text-left hover:bg-gray-100"
                  on:click={() => navigateToPost(post.id)}
                >
                  <div class="min-w-0">
                    <p class="truncate text-sm font-semibold text-gray-900">{getSavedPostTitle(post)}</p>
                    {#if getSavedPostDomain(post)}
                      <p class="truncate text-xs text-gray-500">{getSavedPostDomain(post)}</p>
                    {/if}
                    <div class="mt-1 flex items-center gap-2 text-xs text-gray-500">
                      <span>@{post.user?.username ?? 'unknown'}</span>
                      <span class="text-gray-300">•</span>
                      <RelativeTime dateString={post.createdAt} className="text-xs text-gray-400" />
                    </div>
                  </div>
                  <span
                    class={`inline-flex flex-shrink-0 items-center rounded-full border px-2 py-0.5 text-xs font-semibold ${getKindBadgeClasses(kind)}`}
                  >
                    {getKindLabel(kind)}
                  </span>
                </button>
              </li>
            {/each}
          </ul>
        {/if}

        {#if $podcastStore.hasMore}
          <button
            class="mt-3 w-full rounded-md border border-gray-200 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
            type="button"
            data-testid="podcasts-saved-load-more"
            on:click={loadMoreCurrentMode}
            disabled={$podcastStore.isLoadingSaved}
          >
            {#if $podcastStore.isLoadingSaved}
              Loading...
            {:else}
              Load more
            {/if}
          </button>
        {/if}
      {/if}
    </div>
  </section>
{/if}
