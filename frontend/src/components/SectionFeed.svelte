<script lang="ts">
  import { onDestroy, tick } from 'svelte';
  import {
    activeSection,
    posts,
    filteredPosts,
    isLoadingPosts,
    postsError,
    postsPaginationError,
    hasMorePosts,
    postStore,
    threadRouteStore,
    musicLengthFilter,
  } from '../stores';
  import { loadFeed, loadMorePosts } from '../stores/feedStore';
  import { loadThreadTargetPost } from '../stores/threadRouteStore';
  import PostCard from './PostCard.svelte';

  let observer: IntersectionObserver | null = null;
  let observedElement: HTMLElement | null = null;
  let sentinel: HTMLElement;
  let isLoadingMore = false;
  let lastScrolledPostId: string | null = null;

  $: if ($activeSection?.id) {
    loadFeed($activeSection.id);
  }

  $: displayPosts = $filteredPosts;
  $: hasFilteredResults = displayPosts.length > 0;
  $: filterLabel =
    $musicLengthFilter === 'tracks' ? 'tracks' : $musicLengthFilter === 'sets' ? 'sets/mixes' : '';

  async function handleLoadMore() {
    if (isLoadingMore || !$hasMorePosts) return;
    isLoadingMore = true;
    await loadMorePosts();
    isLoadingMore = false;
  }

  function ensureObserver() {
    if (!observer) {
      observer = new IntersectionObserver(
        (entries) => {
          const entry = entries[0];
          if (
            entry?.isIntersecting &&
            $hasMorePosts &&
            !isLoadingMore &&
            !$isLoadingPosts &&
            !$postsPaginationError
          ) {
            handleLoadMore();
          }
        },
        {
          root: null,
          rootMargin: '100px',
          threshold: 0,
        }
      );
    }

    if (observer && sentinel && observedElement !== sentinel) {
      observer.disconnect();
      observer.observe(sentinel);
      observedElement = sentinel;
    }
  }

  onDestroy(() => {
    if (observer) {
      observer.disconnect();
    }
    postStore.reset();
  });

  if (typeof window !== 'undefined') {
    ensureObserver();
  }

  $: if (sentinel) {
    ensureObserver();
  }

  $: if (!($threadRouteStore.postId)) {
    lastScrolledPostId = null;
  } else if (lastScrolledPostId && $threadRouteStore.postId !== lastScrolledPostId) {
    lastScrolledPostId = null;
  }

  $: if (
    $threadRouteStore.postId &&
    $threadRouteStore.sectionId &&
    $activeSection?.id === $threadRouteStore.sectionId &&
    !$isLoadingPosts &&
    $threadRouteStore.status === 'idle'
  ) {
    loadThreadTargetPost($threadRouteStore.postId, $threadRouteStore.sectionId);
  }

  $: if (
    $threadRouteStore.postId &&
    $threadRouteStore.sectionId &&
    $activeSection?.id === $threadRouteStore.sectionId &&
    $threadRouteStore.status === 'ready' &&
    lastScrolledPostId !== $threadRouteStore.postId &&
    typeof window !== 'undefined'
  ) {
    const targetPostId = $threadRouteStore.postId;
    tick().then(() => {
      const element = document.getElementById(`post-${targetPostId}`);
      if (element) {
        element.scrollIntoView({ behavior: 'smooth', block: 'start' });
        lastScrolledPostId = targetPostId;
      }
    });
  }
</script>

<div class="space-y-4">
  {#if $threadRouteStore.postId && $threadRouteStore.sectionId === $activeSection?.id}
    {#if $threadRouteStore.status === 'not_found'}
      <div class="bg-amber-50 border border-amber-200 rounded-lg p-4 text-sm text-amber-800">
        This thread is no longer available.
      </div>
    {:else if $threadRouteStore.status === 'error'}
      <div class="bg-red-50 border border-red-200 rounded-lg p-4 text-sm text-red-700">
        Unable to load this thread. {$threadRouteStore.error}
      </div>
    {/if}
  {/if}
  {#if $isLoadingPosts && $posts.length === 0}
    <div class="flex justify-center py-8">
      <div class="flex items-center gap-2 text-gray-500">
        <svg
          class="animate-spin h-5 w-5"
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
        <span>Loading posts...</span>
      </div>
    </div>
  {:else if $postsError && $posts.length === 0}
    <div class="bg-red-50 border border-red-200 rounded-lg p-4 text-center">
      <p class="text-red-600">{$postsError}</p>
      <button
        on:click={() => $activeSection && loadFeed($activeSection.id)}
        class="mt-2 text-sm text-red-700 underline hover:no-underline"
      >
        Try again
      </button>
    </div>
  {:else if $posts.length === 0}
    <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-8 text-center">
      <p class="text-gray-500">No posts yet. Be the first to share something!</p>
    </div>
  {:else}
    {#if !hasFilteredResults}
      <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6 text-center">
        <p class="text-gray-500">
          No {filterLabel} posts yet. Try switching the length filter.
        </p>
      </div>
    {:else}
      {#each displayPosts as post (post.id)}
        {@const isTarget = $threadRouteStore.postId === post.id}
        <div
          id={`post-${post.id}`}
          class={`scroll-mt-24 ${isTarget ? 'ring-2 ring-blue-200 rounded-lg' : ''}`}
        >
          <PostCard {post} />
        </div>
      {/each}
    {/if}

    <div bind:this={sentinel} class="h-4"></div>

    {#if isLoadingMore}
      <div class="flex justify-center py-4">
        <div class="flex items-center gap-2 text-gray-500">
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
          <span class="text-sm">Loading more...</span>
        </div>
      </div>
    {/if}

    {#if $postsPaginationError}
      <div class="bg-amber-50 border border-amber-200 rounded-lg p-3 text-sm text-amber-700">
        <p>Could not load more posts. {$postsPaginationError}</p>
        <button
          on:click={handleLoadMore}
          class="mt-2 text-xs text-amber-800 underline hover:no-underline"
        >
          Try again
        </button>
      </div>
    {/if}

    {#if !$hasMorePosts && $posts.length > 0}
      <div class="text-center py-4 text-gray-400 text-sm">
        You've reached the end
      </div>
    {/if}
  {/if}
</div>
