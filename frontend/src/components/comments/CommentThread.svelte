<script lang="ts">
  import { onDestroy } from 'svelte';
  import { commentStore, type CommentThreadState } from '../../stores/commentStore';
  import { loadThreadComments, loadMoreThreadComments } from '../../stores/commentFeedStore';
  import { api } from '../../services/api';
  import { buildProfileHref, handleProfileNavigation } from '../../services/profileNavigation';
  import CommentForm from './CommentForm.svelte';
  import ReplyForm from './ReplyForm.svelte';
  import ReactionBar from '../reactions/ReactionBar.svelte';
  import LinkifiedText from '../LinkifiedText.svelte';

  export let postId: string;
  export let commentCount = 0;

  const emptyThread: CommentThreadState = {
    comments: [],
    isLoading: false,
    error: null,
    cursor: null,
    hasMore: true,
    loaded: false,
    seenCommentIds: new Set(),
  };

  let openReplies = new Set<string>();
  let rootEl: HTMLElement | null = null;
  let observer: IntersectionObserver | null = null;
  let isVisible = false;

  function getUserReactions(comment: { viewerReactions?: string[] }): Set<string> {
    return new Set(comment.viewerReactions ?? []);
  }

  async function toggleCommentReaction(commentId: string, emoji: string) {
    const threadState = $commentStore[postId];
    if (!threadState) return;

    // Helper to find comment in tree
    const findComment = (comments: typeof threadState.comments): { viewerReactions?: string[] } | undefined => {
      for (const c of comments) {
        if (c.id === commentId) return c;
        if (c.replies) {
          const found = findComment(c.replies);
          if (found) return found;
        }
      }
    };

    const comment = findComment(threadState.comments);
    const hasReacted = comment ? new Set(comment.viewerReactions ?? []).has(emoji) : false;

    // Optimistic update
    commentStore.toggleReaction(postId, commentId, emoji);

    try {
      if (hasReacted) {
        await api.removeCommentReaction(commentId, emoji);
      } else {
        await api.addCommentReaction(commentId, emoji);
      }
    } catch (e) {
      console.error('Failed to toggle comment reaction:', e);
      // Revert
      commentStore.toggleReaction(postId, commentId, emoji);
    }
  }

  $: thread = $commentStore[postId] ?? emptyThread;
  $: shouldLoad = commentCount > 0;

  function toggleReply(commentId: string) {
    if (openReplies.has(commentId)) {
      openReplies = new Set([...openReplies].filter((id) => id !== commentId));
    } else {
      openReplies = new Set([...openReplies, commentId]);
    }
  }

  function closeReply(commentId: string) {
    openReplies = new Set([...openReplies].filter((id) => id !== commentId));
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

  function getProviderIcon(provider: string | undefined): string {
    switch (provider) {
      case 'spotify':
        return '🎵';
      case 'youtube':
        return '▶️';
      case 'soundcloud':
        return '☁️';
      case 'imdb':
      case 'rottentomatoes':
        return '🎬';
      case 'goodreads':
        return '📚';
      case 'eventbrite':
      case 'ra':
        return '📅';
      default:
        return '🔗';
    }
  }

  function ensureObserver() {
    if (!rootEl || typeof window === 'undefined' || !shouldLoad) return;
    if (!observer) {
      observer = new IntersectionObserver(
        (entries) => {
          const entry = entries[0];
          if (entry?.isIntersecting) {
            isVisible = true;
          }
        },
        {
          root: null,
          rootMargin: '120px',
          threshold: 0,
        }
      );
    }
    if (observer) {
      observer.disconnect();
      observer.observe(rootEl);
    }
  }

  onDestroy(() => {
    observer?.disconnect();
  });

  $: if (rootEl && shouldLoad) {
    ensureObserver();
  }

  $: if (postId && shouldLoad && isVisible && !thread.loaded && !thread.isLoading && !thread.error) {
    loadThreadComments(postId);
  }
</script>

<div class="space-y-4" bind:this={rootEl}>
  <div class="border border-gray-200 rounded-lg p-3 bg-gray-50">
    <CommentForm {postId} />
  </div>

  {#if thread.isLoading && thread.comments.length === 0}
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
      <span>Loading comments...</span>
    </div>
  {:else if thread.error}
    <div class="bg-red-50 border border-red-200 rounded-lg p-3 text-sm text-red-600">
      <p>{thread.error}</p>
      <button
        on:click={() => loadThreadComments(postId)}
        class="mt-2 text-xs text-red-700 underline hover:no-underline"
      >
        Try again
      </button>
    </div>
  {:else if thread.comments.length === 0}
    <div class="text-sm text-gray-500">No comments yet. Start the conversation.</div>
  {:else}
    <div class="space-y-4">
      {#each thread.comments as comment (comment.id)}
        <article class="bg-white border border-gray-200 rounded-lg p-3">
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
                    class="w-8 h-8 rounded-full object-cover"
                  />
                {:else}
                  <div class="w-8 h-8 rounded-full bg-gray-200 flex items-center justify-center">
                    <span class="text-gray-500 text-xs font-medium">
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
                  class="w-8 h-8 rounded-full object-cover flex-shrink-0"
                />
              {:else}
                <div class="w-8 h-8 rounded-full bg-gray-200 flex items-center justify-center flex-shrink-0">
                  <span class="text-gray-500 text-xs font-medium">
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
                    class="font-medium text-gray-900 text-sm truncate hover:underline"
                    on:click={(event) => handleProfileNavigation(event, comment.user?.id)}
                  >
                    {comment.user?.username || 'Unknown'}
                  </a>
                {:else}
                  <span class="font-medium text-gray-900 text-sm truncate">
                    {comment.user?.username || 'Unknown'}
                  </span>
                {/if}
                <span class="text-gray-400 text-xs">·</span>
                <time class="text-gray-500 text-xs" datetime={comment.createdAt}>
                  {formatDate(comment.createdAt)}
                </time>
              </div>

              <LinkifiedText
                text={comment.content}
                className="text-gray-800 text-sm whitespace-pre-wrap break-words"
                linkClassName="text-blue-600 hover:text-blue-800 underline"
              />

              {#if comment.links?.length}
                {#each comment.links as link (link.url)}
                  <div class="mt-2">
                    {#if link.metadata}
                      <a
                        href={link.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        class="block rounded-lg border border-gray-200 overflow-hidden hover:border-gray-300 transition-colors"
                      >
                        <div class="flex">
                          {#if link.metadata.image}
                            <div class="w-16 h-16 flex-shrink-0">
                              <img
                                src={link.metadata.image}
                                alt={link.metadata.title || 'Link preview'}
                                class="w-full h-full object-cover"
                              />
                            </div>
                          {/if}
                          <div class="flex-1 p-2 min-w-0">
                            <div class="flex items-center gap-1 mb-1">
                              <span>{getProviderIcon(link.metadata.provider)}</span>
                              {#if link.metadata.provider}
                                <span class="text-xs text-gray-500 capitalize">
                                  {link.metadata.provider}
                                </span>
                              {/if}
                            </div>
                            {#if link.metadata.title}
                              <h4 class="font-medium text-gray-900 text-xs truncate">
                                {link.metadata.title}
                              </h4>
                            {/if}
                            {#if link.metadata.description}
                              <p class="text-gray-600 text-xs line-clamp-2">
                                {link.metadata.description}
                              </p>
                            {/if}
                          </div>
                        </div>
                      </a>
                    {:else}
                      <a
                        href={link.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        class="inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 text-xs break-all"
                      >
                        <span>🔗</span>
                        <span class="underline">{link.url}</span>
                      </a>
                    {/if}
                  </div>
                {/each}
              {/if}

              <div class="mt-2">
                <ReactionBar
                  reactionCounts={comment.reactionCounts ?? {}}
                  userReactions={getUserReactions(comment)}
                  onToggle={(emoji) => toggleCommentReaction(comment.id, emoji)}
                />
              </div>

              <div class="mt-2">
                <button
                  type="button"
                  on:click={() => toggleReply(comment.id)}
                  class="text-xs text-gray-500 hover:text-gray-700"
                >
                  Reply
                </button>
              </div>

              {#if openReplies.has(comment.id)}
                <div class="mt-3">
                  <ReplyForm
                    {postId}
                    parentCommentId={comment.id}
                    on:cancel={() => closeReply(comment.id)}
                    on:submit={() => closeReply(comment.id)}
                  />
                </div>
              {/if}

              {#if comment.replies?.length}
                <div class="mt-4 space-y-3 border-l border-gray-200 pl-4">
                  {#each comment.replies as reply (reply.id)}
                    <div class="flex items-start gap-2">
                      {#if reply.user?.id}
                        <a
                          href={buildProfileHref(reply.user.id)}
                          class="flex-shrink-0"
                          on:click={(event) => handleProfileNavigation(event, reply.user?.id)}
                          aria-label={`View ${(reply.user?.username ?? 'user')}'s profile`}
                        >
                          {#if reply.user?.profilePictureUrl}
                            <img
                              src={reply.user.profilePictureUrl}
                              alt={reply.user.username}
                              class="w-7 h-7 rounded-full object-cover"
                            />
                          {:else}
                            <div class="w-7 h-7 rounded-full bg-gray-200 flex items-center justify-center">
                              <span class="text-gray-500 text-xs font-medium">
                                {reply.user?.username?.charAt(0).toUpperCase() || '?'}
                              </span>
                            </div>
                          {/if}
                        </a>
                      {:else}
                        {#if reply.user?.profilePictureUrl}
                          <img
                            src={reply.user.profilePictureUrl}
                            alt={reply.user.username}
                            class="w-7 h-7 rounded-full object-cover flex-shrink-0"
                          />
                        {:else}
                          <div class="w-7 h-7 rounded-full bg-gray-200 flex items-center justify-center flex-shrink-0">
                            <span class="text-gray-500 text-xs font-medium">
                              {reply.user?.username?.charAt(0).toUpperCase() || '?'}
                            </span>
                          </div>
                        {/if}
                      {/if}

                      <div class="flex-1 min-w-0">
                        <div class="flex items-center gap-2 mb-1">
                          {#if reply.user?.id}
                            <a
                              href={buildProfileHref(reply.user.id)}
                              class="font-medium text-gray-900 text-xs truncate hover:underline"
                              on:click={(event) => handleProfileNavigation(event, reply.user?.id)}
                            >
                              {reply.user?.username || 'Unknown'}
                            </a>
                          {:else}
                            <span class="font-medium text-gray-900 text-xs truncate">
                              {reply.user?.username || 'Unknown'}
                            </span>
                          {/if}
                          <span class="text-gray-400 text-xs">·</span>
                          <time class="text-gray-500 text-xs" datetime={reply.createdAt}>
                            {formatDate(reply.createdAt)}
                          </time>
                        </div>
                        <LinkifiedText
                          text={reply.content}
                          className="text-gray-800 text-sm whitespace-pre-wrap break-words"
                          linkClassName="text-blue-600 hover:text-blue-800 underline"
                        />
                        <div class="mt-2">
                          <ReactionBar
                            reactionCounts={reply.reactionCounts ?? {}}
                            userReactions={getUserReactions(reply)}
                            onToggle={(emoji) => toggleCommentReaction(reply.id, emoji)}
                          />
                        </div>
                      </div>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          </div>
        </article>
      {/each}

      {#if thread.hasMore && commentCount > thread.comments.length}
        <div class="flex justify-center">
          <button
            type="button"
            on:click={() => loadMoreThreadComments(postId)}
            class="text-xs text-gray-600 hover:text-gray-900"
          >
            Load more comments
          </button>
        </div>
      {/if}
    </div>
  {/if}
</div>
