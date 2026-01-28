<script lang="ts">
  import {
    searchResults,
    isSearching,
    searchError,
    searchQuery,
    lastSearchQuery,
    searchScope,
    activeSection,
    searchStore,
    postStore,
    sections,
    sectionStore,
    uiStore,
  } from '../../stores';
  import PostCard from '../PostCard.svelte';
  import ReactionBar from '../reactions/ReactionBar.svelte';
  import LinkifiedText from '../LinkifiedText.svelte';
  import { api } from '../../services/api';
  import { buildProfileHref, handleProfileNavigation } from '../../services/profileNavigation';
  import { buildSectionHref, pushPath } from '../../services/routeNavigation';
  import type { Post } from '../../stores/postStore';
  import type { CommentResult, SearchResult } from '../../stores/searchStore';

  // Track pending reactions to prevent double-clicks
  let pendingReactions = new Set<string>();

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

  function openPostThread(post: Post | undefined) {
    if (!post) return;
    const targetSection = $sections.find((section) => section.id === post.sectionId);
    const switchingSection = targetSection && $activeSection?.id !== targetSection.id;
    if (targetSection) {
      uiStore.setActiveView('feed');
      pushPath(buildSectionHref(targetSection.id));
    }
    if (switchingSection) {
      sectionStore.setActiveSection(targetSection);
    }
    if (switchingSection) {
      let resolved = false;
      let sawLoading = false;
      let shouldUnsubscribe = false;
      let unsubscribe: (() => void) | null = null;
      const maybeUnsubscribe = () => {
        if (unsubscribe) {
          unsubscribe();
          unsubscribe = null;
        } else {
          shouldUnsubscribe = true;
        }
      };
      unsubscribe = postStore.subscribe((state) => {
        if (resolved) return;
        if (state.isLoading) {
          sawLoading = true;
          return;
        }
        if (!sawLoading) return;
        resolved = true;
        postStore.upsertPost(post);
        maybeUnsubscribe();
      });
      if (shouldUnsubscribe && unsubscribe) {
        unsubscribe();
        unsubscribe = null;
      }
    } else {
      postStore.upsertPost(post);
    }
    searchStore.setQuery('');
    if (typeof window !== 'undefined') {
      window.scrollTo({ top: 0, behavior: 'smooth' });
    }
  }

  type SearchResultWithLink = SearchResult & { linkMetadata?: { id?: string } };

  function resultKey(result: SearchResult, index: number): string {
    if (result.type === 'comment') {
      return `comment-${result.comment?.id ?? index}`;
    }
    if (result.type === 'post') {
      return `post-${result.post?.id ?? index}`;
    }
    const linkId = (result as SearchResultWithLink).linkMetadata?.id;
    return linkId ? `link-${linkId}` : `${result.type}-${index}`;
  }

  function resolveSectionName(result: SearchResult): string | null {
    const sectionId =
      (result.type === 'post' && result.post?.sectionId) ||
      (result.type === 'comment' && result.post?.sectionId) ||
      $activeSection?.id ||
      null;
    if (!sectionId) return null;
    const section = $sections.find((item) => item.id === sectionId);
    return section?.name ?? $activeSection?.name ?? null;
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

    {#each $searchResults as result, index (resultKey(result, index))}
      {@const sectionName = resolveSectionName(result)}
      {#if result.type === 'post' && result.post}
        {#if sectionName}
          <div class="inline-flex items-center gap-2 text-xs font-medium text-gray-500">
            <span class="px-2 py-0.5 rounded-full bg-gray-100 border border-gray-200">
              {sectionName}
            </span>
          </div>
        {/if}
        <PostCard post={result.post} />
      {:else if result.type === 'comment' && result.comment}
        {@const comment = result.comment}
        {@const parentPost = result.post}
        <article class="bg-white rounded-lg shadow-sm border border-gray-200 p-4 space-y-4">
          {#if parentPost}
            <div class="rounded-lg border border-gray-200 bg-gray-50 p-3">
              <div class="flex items-center justify-between text-xs text-gray-500 mb-2">
                <div class="flex items-center gap-2">
                  <span>Parent post</span>
                  {#if sectionName}
                    <span class="px-2 py-0.5 rounded-full bg-white border border-gray-200 text-gray-500">
                      {sectionName}
                    </span>
                  {/if}
                </div>
                <button
                  type="button"
                  class="text-blue-600 hover:text-blue-800 underline"
                  on:click={() => openPostThread(parentPost)}
                >
                  View full thread
                </button>
              </div>
              <div class="flex items-start gap-3">
                {#if parentPost.user?.profilePictureUrl}
                  <img
                    src={parentPost.user.profilePictureUrl}
                    alt={parentPost.user.username}
                    class="w-8 h-8 rounded-full object-cover flex-shrink-0"
                  />
                {:else}
                  <div class="w-8 h-8 rounded-full bg-gray-200 flex items-center justify-center flex-shrink-0">
                    <span class="text-gray-500 text-xs font-medium">
                      {parentPost.user?.username?.charAt(0).toUpperCase() || '?'}
                    </span>
                  </div>
                {/if}

                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2 mb-1">
                    <span class="text-sm font-medium text-gray-900 truncate">
                      {parentPost.user?.username || 'Unknown'}
                    </span>
                    <span class="text-gray-400 text-xs">·</span>
                    <time class="text-gray-500 text-xs" datetime={parentPost.createdAt}>
                      {formatDate(parentPost.createdAt)}
                    </time>
                  </div>
                  <p class="text-gray-800 text-sm whitespace-pre-wrap break-words line-clamp-3">
                    {parentPost.content}
                  </p>
                  {#if parentPost.links && parentPost.links.length > 0}
                    <div class="mt-2 text-xs text-blue-600 break-all">
                      <a
                        href={parentPost.links[0].url}
                        target="_blank"
                        rel="noopener noreferrer"
                        class="underline"
                      >
                        {parentPost.links[0].url}
                      </a>
                    </div>
                  {/if}
                </div>
              </div>
            </div>
          {/if}

          {#if !parentPost && sectionName}
            <div class="inline-flex items-center gap-2 text-xs font-medium text-gray-500">
              <span class="px-2 py-0.5 rounded-full bg-gray-100 border border-gray-200">
                {sectionName}
              </span>
            </div>
          {/if}

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

              <LinkifiedText
                text={comment.content}
                highlightQuery={normalizedQuery}
                className="text-gray-800 whitespace-pre-wrap break-words"
                linkClassName="text-blue-600 hover:text-blue-800 underline"
              />

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
