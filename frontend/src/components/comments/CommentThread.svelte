<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import { commentStore, type CommentThreadState } from '../../stores/commentStore';
  import { currentUser, isAdmin, postStore } from '../../stores';
  import { loadThreadComments, loadMoreThreadComments } from '../../stores/commentFeedStore';
  import { api } from '../../services/api';
  import { buildProfileHref, handleProfileNavigation } from '../../services/profileNavigation';
  import CommentForm from './CommentForm.svelte';
  import ReplyForm from './ReplyForm.svelte';
  import ReactionBar from '../reactions/ReactionBar.svelte';
  import LinkifiedText from '../LinkifiedText.svelte';
  import EditedBadge from '../EditedBadge.svelte';
  import RelativeTime from '../RelativeTime.svelte';
  import { logError } from '../../lib/observability/logger';
  import { recordComponentRender } from '../../lib/observability/performance';

  export let postId: string;
  export let commentCount = 0;
  export let highlightCommentId: string | null = null;
  export let highlightCommentIds: string[] = [];

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
  let editingCommentId: string | null = null;
  let editCommentContent = '';
  let editCommentError: string | null = null;
  let isSavingComment = false;
  let deletingCommentIds = new Set<string>();
  let deleteCommentErrors: Record<string, string | null> = {};
  let lastHighlightId: string | null = null;

  const renderStart = typeof performance !== 'undefined' ? performance.now() : null;

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
      logError('Failed to toggle comment reaction', { commentId, emoji }, e);
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

  function startEdit(commentId: string, content: string) {
    editingCommentId = commentId;
    editCommentContent = content;
    editCommentError = null;
  }

  function cancelEdit() {
    editingCommentId = null;
    editCommentContent = '';
    editCommentError = null;
  }

  async function saveEdit(commentId: string) {
    const trimmed = editCommentContent.trim();
    if (!trimmed) {
      editCommentError = 'Content is required.';
      return;
    }

    isSavingComment = true;
    editCommentError = null;

    try {
      const response = await api.updateComment(commentId, { content: trimmed });
      commentStore.updateComment(postId, response.comment);
      editingCommentId = null;
    } catch (err) {
      editCommentError = err instanceof Error ? err.message : 'Failed to update comment';
    } finally {
      isSavingComment = false;
    }
  }

  async function deleteComment(commentId: string) {
    if (typeof window !== 'undefined') {
      const confirmed = window.confirm('Delete this comment?');
      if (!confirmed) {
        return;
      }
    }

    deleteCommentErrors = { ...deleteCommentErrors, [commentId]: null };
    deletingCommentIds = new Set(deletingCommentIds).add(commentId);
    try {
      await api.deleteComment(commentId);
      const removedCount = commentStore.removeComment(postId, commentId);
      if (removedCount > 0) {
        postStore.incrementCommentCount(postId, -removedCount);
      }
      if (editingCommentId === commentId) {
        cancelEdit();
      }
    } catch (err) {
      deleteCommentErrors = {
        ...deleteCommentErrors,
        [commentId]: err instanceof Error ? err.message : 'Failed to delete comment',
      };
      logError('Failed to delete comment', { commentId, postId }, err);
    } finally {
      const nextDeleting = new Set(deletingCommentIds);
      nextDeleting.delete(commentId);
      deletingCommentIds = nextDeleting;
    }
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

  onMount(() => {
    if (renderStart === null) {
      return;
    }
    recordComponentRender('CommentThread', performance.now() - renderStart);
  });

  $: if (rootEl && shouldLoad) {
    ensureObserver();
  }

  $: if (postId && shouldLoad && isVisible && !thread.loaded && !thread.isLoading && !thread.error) {
    loadThreadComments(postId);
  }

  $: if (
    highlightCommentId &&
    highlightCommentId !== lastHighlightId &&
    thread.loaded &&
    typeof document !== 'undefined'
  ) {
    lastHighlightId = highlightCommentId;
    tick().then(() => {
      const el = document.getElementById(`comment-${highlightCommentId}`);
      if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
    });
  }

  $: highlightIdSet = new Set(
    [highlightCommentId, ...highlightCommentIds].filter((value): value is string => !!value)
  );
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
        <article
          id={`comment-${comment.id}`}
          class={`border border-gray-200 rounded-lg p-3 ${
            highlightIdSet.has(comment.id) ? 'bg-amber-50 ring-2 ring-amber-300' : 'bg-white'
          }`}
        >
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
                <RelativeTime dateString={comment.createdAt} className="text-gray-500 text-xs" />
                <EditedBadge createdAt={comment.createdAt} updatedAt={comment.updatedAt} />
                {#if $currentUser?.id === comment.userId || $isAdmin}
                  <div class="ml-auto flex items-center gap-2">
                    {#if $currentUser?.id === comment.userId}
                      <button
                        type="button"
                        class="inline-flex items-center gap-1 rounded-md border border-gray-200 px-2.5 py-1 text-xs font-medium text-gray-600 hover:text-gray-800 hover:bg-gray-50"
                        on:click={() => startEdit(comment.id, comment.content)}
                      >
                        <svg class="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                          <path
                            d="M4 13.5V16h2.5l7.35-7.35-2.5-2.5L4 13.5zM16.85 5.65a.5.5 0 000-.7l-1.8-1.8a.5.5 0 00-.7 0l-1.6 1.6 2.5 2.5 1.6-1.6z"
                          />
                        </svg>
                        <span>Edit</span>
                      </button>
                    {/if}
                    <button
                      type="button"
                      class="inline-flex items-center gap-1 rounded-md border border-red-200 px-2.5 py-1 text-xs font-medium text-red-600 hover:text-red-700 hover:bg-red-50 disabled:opacity-60"
                      on:click={() => deleteComment(comment.id)}
                      disabled={deletingCommentIds.has(comment.id)}
                    >
                      {deletingCommentIds.has(comment.id) ? 'Deleting...' : 'Delete'}
                    </button>
                  </div>
                {/if}
              </div>

              {#if editingCommentId === comment.id}
                <div class="space-y-2">
                  <textarea
                    class="w-full rounded-lg border border-gray-300 p-2 text-sm text-gray-800 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                    rows="3"
                    bind:value={editCommentContent}
                  />
                  {#if editCommentError}
                    <div class="text-sm text-red-600">{editCommentError}</div>
                  {/if}
                  <div class="flex items-center gap-2">
                    <button
                      type="button"
                      class="px-3 py-1.5 rounded-md bg-blue-600 text-white text-xs hover:bg-blue-700 disabled:opacity-60"
                      on:click={() => saveEdit(comment.id)}
                      disabled={isSavingComment}
                    >
                      {isSavingComment ? 'Saving...' : 'Save'}
                    </button>
                    <button
                      type="button"
                      class="px-3 py-1.5 rounded-md border border-gray-300 text-xs text-gray-700 hover:bg-gray-50 disabled:opacity-60"
                      on:click={cancelEdit}
                      disabled={isSavingComment}
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              {:else}
                <LinkifiedText
                  text={comment.content}
                  className="text-gray-800 text-sm whitespace-pre-wrap break-words"
                  linkClassName="text-blue-600 hover:text-blue-800 underline"
                />
              {/if}

              {#if deleteCommentErrors[comment.id]}
                <div class="mt-2 text-xs text-red-600">{deleteCommentErrors[comment.id]}</div>
              {/if}

              {#if editingCommentId !== comment.id && comment.links?.length}
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

              {#if editingCommentId !== comment.id}
                <div class="mt-2 flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    on:click={() => toggleReply(comment.id)}
                    class="inline-flex items-center gap-1 rounded-full border border-gray-200 bg-white px-2 py-1 text-xs text-gray-600 transition hover:border-gray-300 hover:text-gray-700"
                  >
                    Reply
                  </button>
                  <ReactionBar
                    reactionCounts={comment.reactionCounts ?? {}}
                    userReactions={getUserReactions(comment)}
                    onToggle={(emoji) => toggleCommentReaction(comment.id, emoji)}
                    commentId={comment.id}
                  />
                </div>
              {/if}

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
                    <div
                      id={`comment-${reply.id}`}
                      class={`flex items-start gap-2 ${
                        highlightIdSet.has(reply.id)
                          ? 'bg-amber-50 ring-2 ring-amber-300 rounded-lg p-2'
                          : ''
                      }`}
                    >
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
                          <RelativeTime dateString={reply.createdAt} className="text-gray-500 text-xs" />
                          <EditedBadge createdAt={reply.createdAt} updatedAt={reply.updatedAt} />
                          {#if $currentUser?.id === reply.userId || $isAdmin}
                            <div class="ml-auto flex items-center gap-2">
                              {#if $currentUser?.id === reply.userId}
                                <button
                                  type="button"
                                  class="inline-flex items-center gap-1 rounded-md border border-gray-200 px-2.5 py-1 text-xs font-medium text-gray-600 hover:text-gray-800 hover:bg-gray-50"
                                  on:click={() => startEdit(reply.id, reply.content)}
                                >
                                  <svg class="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                                    <path
                                      d="M4 13.5V16h2.5l7.35-7.35-2.5-2.5L4 13.5zM16.85 5.65a.5.5 0 000-.7l-1.8-1.8a.5.5 0 00-.7 0l-1.6 1.6 2.5 2.5 1.6-1.6z"
                                    />
                                  </svg>
                                  <span>Edit</span>
                                </button>
                              {/if}
                              <button
                                type="button"
                                class="inline-flex items-center gap-1 rounded-md border border-red-200 px-2.5 py-1 text-xs font-medium text-red-600 hover:text-red-700 hover:bg-red-50 disabled:opacity-60"
                                on:click={() => deleteComment(reply.id)}
                                disabled={deletingCommentIds.has(reply.id)}
                              >
                                {deletingCommentIds.has(reply.id) ? 'Deleting...' : 'Delete'}
                              </button>
                            </div>
                          {/if}
                        </div>
                        {#if editingCommentId === reply.id}
                          <div class="space-y-2">
                            <textarea
                              class="w-full rounded-lg border border-gray-300 p-2 text-sm text-gray-800 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                              rows="3"
                              bind:value={editCommentContent}
                            />
                            {#if editCommentError}
                              <div class="text-sm text-red-600">{editCommentError}</div>
                            {/if}
                            <div class="flex items-center gap-2">
                              <button
                                type="button"
                                class="px-3 py-1.5 rounded-md bg-blue-600 text-white text-xs hover:bg-blue-700 disabled:opacity-60"
                                on:click={() => saveEdit(reply.id)}
                                disabled={isSavingComment}
                              >
                                {isSavingComment ? 'Saving...' : 'Save'}
                              </button>
                              <button
                                type="button"
                                class="px-3 py-1.5 rounded-md border border-gray-300 text-xs text-gray-700 hover:bg-gray-50 disabled:opacity-60"
                                on:click={cancelEdit}
                                disabled={isSavingComment}
                              >
                                Cancel
                              </button>
                            </div>
                          </div>
                        {:else}
                          <LinkifiedText
                            text={reply.content}
                            className="text-gray-800 text-sm whitespace-pre-wrap break-words"
                            linkClassName="text-blue-600 hover:text-blue-800 underline"
                          />
                          {#if deleteCommentErrors[reply.id]}
                            <div class="mt-2 text-xs text-red-600">{deleteCommentErrors[reply.id]}</div>
                          {/if}
                          <div class="mt-2">
                            <ReactionBar
                              reactionCounts={reply.reactionCounts ?? {}}
                              userReactions={getUserReactions(reply)}
                              onToggle={(emoji) => toggleCommentReaction(reply.id, emoji)}
                              commentId={reply.id}
                            />
                          </div>
                        {/if}
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
