<script lang="ts">
  import { onDestroy } from 'svelte';
  import { activeSection, posts, isLoadingPosts, postsError, hasMorePosts, postStore } from '../stores';
  import { loadFeed, loadMorePosts } from '../stores/feedStore';
  import PostCard from './PostCard.svelte';

  let observer: IntersectionObserver | null = null;
  let observedElement: HTMLElement | null = null;
  let sentinel: HTMLElement;
  let isLoadingMore = false;

  $: if ($activeSection?.id) {
    loadFeed($activeSection.id);
  }

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
          if (entry?.isIntersecting && $hasMorePosts && !isLoadingMore && !$isLoadingPosts) {
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
</script>

<div class="space-y-4">
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
  {:else if $postsError}
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
    {#each $posts as post (post.id)}
      <PostCard {post} />
    {/each}

    <div bind:this={sentinel} class="h-4" />

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

    {#if !$hasMorePosts && $posts.length > 0}
      <div class="text-center py-4 text-gray-400 text-sm">
        You've reached the end
      </div>
    {/if}
  {/if}
</div>
