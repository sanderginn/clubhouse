<script lang="ts">
  import { searchResults, isSearching, searchError, searchQuery, lastSearchQuery, searchScope, activeSection } from '../../stores';
  import PostCard from '../PostCard.svelte';

  function formatDate(dateString: string): string {
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    if (diffMins < 1) return 'just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;

    return date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: date.getFullYear() !== now.getFullYear() ? 'numeric' : undefined,
    });
  }

  $: normalizedQuery = $searchQuery.trim();
  $: hasQuery = normalizedQuery.length > 0;
  $: showResults = $lastSearchQuery && $lastSearchQuery === normalizedQuery;
</script>

<section class="space-y-4">
  {#if !hasQuery}
    <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6 text-center">
      <p class="text-gray-500">Start typing to search posts and comments.</p>
    </div>
  {:else if $isSearching}
    <div class="flex justify-center py-8">
      <div class="flex items-center gap-2 text-gray-500">
        <svg class="animate-spin h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          />
        </svg>
        <span>Searching...</span>
      </div>
    </div>
  {:else if $searchError}
    <div class="bg-red-50 border border-red-200 rounded-lg p-4 text-center">
      <p class="text-red-600">{$searchError}</p>
    </div>
  {:else if showResults && $searchResults.length === 0}
    <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6 text-center">
      <p class="text-gray-500">No results for "{$lastSearchQuery}".</p>
    </div>
  {:else if !showResults}
    <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6 text-center">
      <p class="text-gray-500">Press Search to see results.</p>
    </div>
  {:else}
    <div class="flex items-center justify-between text-sm text-gray-500">
      <span>
        Showing {$searchResults.length} result{$searchResults.length === 1 ? '' : 's'} for
        "{$lastSearchQuery}"
      </span>
      <span>
        {#if $searchScope === 'global'}
          All sections
        {:else if $activeSection}
          {$activeSection.name}
        {:else}
          Section
        {/if}
      </span>
    </div>

    {#each $searchResults as result (result.type + (result.post?.id ?? result.comment?.id ?? ''))}
      {#if result.type === 'post' && result.post}
        <PostCard post={result.post} />
      {:else if result.type === 'comment' && result.comment}
        <article class="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
          <div class="flex items-start gap-3">
            {#if result.comment.user?.profilePictureUrl}
              <img
                src={result.comment.user.profilePictureUrl}
                alt={result.comment.user.username}
                class="w-9 h-9 rounded-full object-cover flex-shrink-0"
              />
            {:else}
              <div class="w-9 h-9 rounded-full bg-gray-200 flex items-center justify-center flex-shrink-0">
                <span class="text-gray-500 text-sm font-medium">
                  {result.comment.user?.username?.charAt(0).toUpperCase() || '?'}
                </span>
              </div>
            {/if}

            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 mb-1">
                <span class="font-medium text-gray-900 truncate">
                  {result.comment.user?.username || 'Unknown'}
                </span>
                <span class="text-gray-400 text-sm">commented</span>
                <time class="text-gray-500 text-sm" datetime={result.comment.createdAt}>
                  {formatDate(result.comment.createdAt)}
                </time>
              </div>

              <p class="text-gray-800 whitespace-pre-wrap break-words">
                {result.comment.content}
              </p>

              {#if result.comment.links && result.comment.links.length > 0}
                <div class="mt-2 text-sm text-blue-600 break-all">
                  <a
                    href={result.comment.links[0].url}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="underline"
                  >
                    {result.comment.links[0].url}
                  </a>
                </div>
              {/if}
            </div>
          </div>
        </article>
      {/if}
    {/each}
  {/if}
</section>
