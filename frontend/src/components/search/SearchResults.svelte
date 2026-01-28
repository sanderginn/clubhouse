<script lang="ts">
  import { searchResults, isSearching, searchError, searchQuery, lastSearchQuery, searchScope, activeSection, sections } from '../../stores';
  import PostCard from '../PostCard.svelte';
  import ReactionBar from '../reactions/ReactionBar.svelte';
  import { api } from '../../services/api';
  import { buildProfileHref, handleProfileNavigation } from '../../services/profileNavigation';
  import type { CommentResult } from '../../stores/searchStore';

  // Track pending reactions to prevent double-clicks
  let pendingReactions = new Set<string>();
  let sectionById = new Map<string, { id: string; name: string; type: string; icon: string }>();

  async function toggleCommentReaction(comment: CommentResult, emoji: string) {
    const key = `${comment.id}-${emoji}`;
    if (pendingReactions.has(key)) return;

    const userReactions = new Set(comment.viewerReactions ?? []);
    const hasReacted = userReactions.has(emoji);

    // Optimistic update
    pendingReactions.add(key);
    if (hasReacted) {
      comment.viewerReactions = (comment.viewerReactions ?? []).filter((e) => e !== emoji);
      if (comment.reactionCounts && comment.reactionCounts[emoji]) {
        comment.reactionCounts[emoji]--;
        if (comment.reactionCounts[emoji] <= 0) {
          delete comment.reactionCounts[emoji];
        }
      }
    } else {
      comment.viewerReactions = [...(comment.viewerReactions ?? []), emoji];
      comment.reactionCounts = { ...(comment.reactionCounts ?? {}), [emoji]: (comment.reactionCounts?.[emoji] ?? 0) + 1 };
    }
    // Trigger reactivity
    $searchResults = $searchResults;

    try {
      if (hasReacted) {
        await api.removeCommentReaction(comment.id, emoji);
      } else {
        await api.addCommentReaction(comment.id, emoji);
      }
    } catch (e) {
      console.error('Failed to toggle comment reaction:', e);
      // Revert on error
      if (hasReacted) {
        comment.viewerReactions = [...(comment.viewerReactions ?? []), emoji];
        comment.reactionCounts = { ...(comment.reactionCounts ?? {}), [emoji]: (comment.reactionCounts?.[emoji] ?? 0) + 1 };
      } else {
        comment.viewerReactions = (comment.viewerReactions ?? []).filter((e) => e !== emoji);
        if (comment.reactionCounts && comment.reactionCounts[emoji]) {
          comment.reactionCounts[emoji]--;
          if (comment.reactionCounts[emoji] <= 0) {
            delete comment.reactionCounts[emoji];
          }
        }
      }
      $searchResults = $searchResults;
    } finally {
      pendingReactions.delete(key);
    }
  }

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
  $: sectionById = new Map($sections.map((section) => [section.id, section]));

  function resolveSection(sectionId?: string | null) {
    if (sectionId) {
      return sectionById.get(sectionId) ?? null;
    }
    if ($searchScope === 'section') {
      return $activeSection ?? null;
    }
    return null;
  }

  function formatSectionLabel(section: { name?: string; type?: string } | null): string {
    if (!section) return 'Section';
    return section.name || section.type || 'Section';
  }
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
        {@const section = resolveSection(result.post.sectionId)}
        <div class="space-y-2">
          <div class="flex items-center gap-2 text-xs text-gray-500">
            <span class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5">
              <span>{section?.icon ?? '📁'}</span>
              <span class="capitalize">{formatSectionLabel(section)}</span>
            </span>
          </div>
          <PostCard post={result.post} />
        </div>
      {:else if result.type === 'comment' && result.comment}
        {@const comment = result.comment}
        {@const section = resolveSection(comment.sectionId)}
        <article class="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
          <div class="flex items-center gap-2 text-xs text-gray-500 mb-2">
            <span class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5">
              <span>{section?.icon ?? '📁'}</span>
              <span class="capitalize">{formatSectionLabel(section)}</span>
            </span>
          </div>
          <div class="flex items-start gap-3">
            {#if comment.user?.id}
              <a
                href={buildProfileHref(comment.user.id)}
                class="flex-shrink-0"
                on:click={(event) => handleProfileNavigation(event, comment.user?.id)}
                aria-label={`View ${(comment.user?.username ?? 'user')}'s profile`}
              >
                {#if comment.user?.profilePictureUrl}
                  <img
                    src={comment.user.profilePictureUrl}
                    alt={comment.user.username}
                    class="w-9 h-9 rounded-full object-cover"
                  />
                {:else}
                  <div class="w-9 h-9 rounded-full bg-gray-200 flex items-center justify-center">
                    <span class="text-gray-500 text-sm font-medium">
                      {comment.user?.username?.charAt(0).toUpperCase() || '?'}
                    </span>
                  </div>
                {/if}
              </a>
            {:else}
              {#if comment.user?.profilePictureUrl}
                <img
                  src={comment.user.profilePictureUrl}
                  alt={comment.user.username}
                  class="w-9 h-9 rounded-full object-cover flex-shrink-0"
                />
              {:else}
                <div class="w-9 h-9 rounded-full bg-gray-200 flex items-center justify-center flex-shrink-0">
                  <span class="text-gray-500 text-sm font-medium">
                    {comment.user?.username?.charAt(0).toUpperCase() || '?'}
                  </span>
                </div>
              {/if}
            {/if}

            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 mb-1">
                {#if comment.user?.id}
                  <a
                    href={buildProfileHref(comment.user.id)}
                    class="font-medium text-gray-900 truncate hover:underline"
                    on:click={(event) => handleProfileNavigation(event, comment.user?.id)}
                  >
                    {comment.user?.username || 'Unknown'}
                  </a>
                {:else}
                  <span class="font-medium text-gray-900 truncate">
                    {comment.user?.username || 'Unknown'}
                  </span>
                {/if}
                <span class="text-gray-400 text-sm">commented</span>
                <time class="text-gray-500 text-sm" datetime={comment.createdAt}>
                  {formatDate(comment.createdAt)}
                </time>
              </div>

              <p class="text-gray-800 whitespace-pre-wrap break-words">
                {comment.content}
              </p>

              {#if comment.links && comment.links.length > 0}
                <div class="mt-2 text-sm text-blue-600 break-all">
                  <a
                    href={comment.links[0].url}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="underline"
                  >
                    {comment.links[0].url}
                  </a>
                </div>
              {/if}

              <div class="mt-3">
                <ReactionBar
                  reactionCounts={comment.reactionCounts ?? {}}
                  userReactions={new Set(comment.viewerReactions ?? [])}
                  onToggle={(emoji) => toggleCommentReaction(comment, emoji)}
                />
              </div>
            </div>
          </div>
        </article>
      {/if}
    {/each}
  {/if}
</section>
