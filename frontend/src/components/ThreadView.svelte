<script lang="ts">
  import { activeSection, sections, sectionStore, threadRouteStore, posts } from '../stores';
  import { loadThreadTargetPost } from '../stores/threadRouteStore';
  import { buildFeedHref, pushPath } from '../services/routeNavigation';
  import PostCard from './PostCard.svelte';

  export let highlightCommentId: string | null = null;

  $: threadPost = $threadRouteStore.postId
    ? $posts.find((post) => post.id === $threadRouteStore.postId) ?? null
    : null;
  $: sectionContext = $threadRouteStore.sectionId
    ? $sections.find((section) => section.id === $threadRouteStore.sectionId) ?? null
    : null;

  $: if ($threadRouteStore.postId && $threadRouteStore.status === 'idle') {
    loadThreadTargetPost($threadRouteStore.postId, $threadRouteStore.sectionId);
  }

  $: if (sectionContext && $activeSection?.id !== sectionContext.id) {
    sectionStore.setActiveSection(sectionContext);
  }

  function handleSectionClick() {
    if (!sectionContext) return;
    sectionStore.setActiveSection(sectionContext);
    pushPath(buildFeedHref(sectionContext.slug));
  }
</script>

<div class="space-y-4">
  {#if sectionContext}
    <button
      class="inline-flex items-center gap-2 text-sm font-medium text-gray-600 hover:text-gray-900"
      on:click={handleSectionClick}
    >
      <span class="text-lg" aria-hidden="true">{sectionContext.icon}</span>
      <span class="truncate">{sectionContext.name}</span>
      <span class="text-gray-400">/ Thread</span>
    </button>
  {:else}
    <div class="text-xs font-semibold uppercase tracking-wide text-gray-400">Thread</div>
  {/if}

  {#if $threadRouteStore.status === 'loading' && !threadPost}
    <div class="flex items-center gap-2 text-gray-500 text-sm">
      <svg
        class="animate-spin h-4 w-4"
        xmlns="http://www.w3.org/2000/svg"
        fill="none"
        viewBox="0 0 24 24"
      >
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
        />
      </svg>
      <span>Loading thread...</span>
    </div>
  {:else if $threadRouteStore.status === 'not_found'}
    <div class="bg-amber-50 border border-amber-200 rounded-lg p-4 text-sm text-amber-800">
      This thread is no longer available.
    </div>
  {:else if $threadRouteStore.status === 'error'}
    <div class="bg-red-50 border border-red-200 rounded-lg p-4 text-sm text-red-700">
      Unable to load this thread. {$threadRouteStore.error}
    </div>
  {:else if threadPost}
    <PostCard post={threadPost} {highlightCommentId} />
  {:else}
    <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
      <h1 class="text-xl font-semibold text-gray-900 mb-2">Thread unavailable</h1>
      <p class="text-gray-600">We couldn't load this thread. Try again in a moment.</p>
    </div>
  {/if}
</div>
