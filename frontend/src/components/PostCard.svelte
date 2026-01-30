<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { Post } from '../stores/postStore';
  import { postStore, currentUser } from '../stores';
  import { api } from '../services/api';
  import CommentThread from './comments/CommentThread.svelte';
  import EditedBadge from './EditedBadge.svelte';
  import ReactionBar from './reactions/ReactionBar.svelte';
  import RelativeTime from './RelativeTime.svelte';
  import { buildProfileHref, handleProfileNavigation } from '../services/profileNavigation';
  import { buildThreadHref } from '../services/routeNavigation';
  import LinkifiedText from './LinkifiedText.svelte';
  import { getImageLinkUrl, isInternalUploadUrl, stripInternalUploadUrls } from '../services/linkUtils';
  import { sections } from '../stores/sectionStore';
  import { getSectionSlugById } from '../services/sectionSlug';
  import { logError } from '../lib/observability/logger';
  import { recordComponentRender } from '../lib/observability/performance';

  export let post: Post;
  export let highlightCommentId: string | null = null;

  $: userReactions = new Set(post.viewerReactions ?? []);
  $: sectionSlug = getSectionSlugById($sections, post.sectionId) ?? post.sectionId;
  let copiedLink = false;
  let copyTimeout: ReturnType<typeof setTimeout> | null = null;
  let menuOpen = false;
  let isEditing = false;
  let editContent = '';
  let editError: string | null = null;
  let isSaving = false;

  async function copyThreadLink() {
    if (typeof window === 'undefined') return;
    const url = new URL(buildThreadHref(sectionSlug, post.id), window.location.origin).toString();
    let copied = false;

    if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
      try {
        await navigator.clipboard.writeText(url);
        copied = true;
      } catch {
        copied = false;
      }
    }

    if (!copied && typeof document !== 'undefined' && typeof document.execCommand === 'function') {
      const textarea = document.createElement('textarea');
      textarea.value = url;
      textarea.setAttribute('readonly', '');
      textarea.style.position = 'absolute';
      textarea.style.left = '-9999px';
      document.body.appendChild(textarea);
      textarea.select();
      copied = document.execCommand('copy');
      document.body.removeChild(textarea);
    }

    if (copied) {
      copiedLink = true;
      if (copyTimeout) {
        clearTimeout(copyTimeout);
      }
      copyTimeout = setTimeout(() => {
        copiedLink = false;
      }, 2000);
    }
  }

  onDestroy(() => {
    if (copyTimeout) {
      clearTimeout(copyTimeout);
    }
  });

  function toggleMenu() {
    menuOpen = !menuOpen;
  }

  function closeMenu() {
    menuOpen = false;
  }

  function startEdit() {
    editContent = post.content;
    editError = null;
    isEditing = true;
    closeMenu();
  }

  function cancelEdit() {
    isEditing = false;
    editContent = post.content;
    editError = null;
  }

  async function saveEdit() {
    const trimmed = editContent.trim();
    if (!trimmed) {
      editError = 'Content is required.';
      return;
    }

    isSaving = true;
    editError = null;

    try {
      const response = await api.updatePost(post.id, { content: trimmed });
      postStore.upsertPost(response.post);
      post = { ...post, ...response.post };
      isEditing = false;
    } catch (err) {
      editError = err instanceof Error ? err.message : 'Failed to update post';
    } finally {
      isSaving = false;
    }
  }

  async function toggleReaction(emoji: string) {
    const hasReacted = userReactions.has(emoji);
    // Optimistic update
    postStore.toggleReaction(post.id, emoji);

    try {
      if (hasReacted) {
        await api.removePostReaction(post.id, emoji);
      } else {
        await api.addPostReaction(post.id, emoji);
      }
    } catch (e) {
      logError('Failed to toggle reaction', { postId: post.id, emoji }, e);
      // Revert on error
      postStore.toggleReaction(post.id, emoji);
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

  $: link = post.links?.[0];
  $: metadata = link?.metadata;
  $: imageUrl = getImageLinkUrl(link);
  $: isInternalUploadLink = link ? isInternalUploadUrl(link.url) : false;
  $: displayContent =
    !isEditing && imageUrl && isInternalUploadLink
      ? stripInternalUploadUrls(post.content)
      : post.content;
  $: canEdit = $currentUser?.id === post.userId;

  let imageLoadFailed = false;
  let lastImageUrl: string | undefined;
  $: if (imageUrl !== lastImageUrl) {
    imageLoadFailed = false;
    lastImageUrl = imageUrl;
  }

  const renderStart = typeof performance !== 'undefined' ? performance.now() : null;
  onMount(() => {
    if (renderStart === null) {
      return;
    }
    recordComponentRender('PostCard', performance.now() - renderStart);
  });
</script>

<svelte:window on:click={closeMenu} />

<article class="bg-white rounded-lg shadow-sm border border-gray-200 p-4 hover:shadow-md transition-shadow">
  <div class="flex items-start gap-3">
    {#if post.user?.id}
      <a
        href={buildProfileHref(post.user.id)}
        on:click={(event) => handleProfileNavigation(event, post.user?.id)}
        class="flex-shrink-0"
        aria-label={`View ${(post.user?.username ?? 'user')}'s profile`}
      >
        {#if post.user?.profilePictureUrl}
          <img
            src={post.user.profilePictureUrl}
            alt={post.user.username}
            class="w-10 h-10 rounded-full object-cover"
          />
        {:else}
          <div class="w-10 h-10 rounded-full bg-gray-200 flex items-center justify-center">
            <span class="text-gray-500 text-sm font-medium">
              {post.user?.username?.charAt(0).toUpperCase() || '?'}
            </span>
          </div>
        {/if}
      </a>
    {:else}
      {#if post.user?.profilePictureUrl}
        <img
          src={post.user.profilePictureUrl}
          alt={post.user.username}
          class="w-10 h-10 rounded-full object-cover flex-shrink-0"
        />
      {:else}
        <div class="w-10 h-10 rounded-full bg-gray-200 flex items-center justify-center flex-shrink-0">
          <span class="text-gray-500 text-sm font-medium">
            {post.user?.username?.charAt(0).toUpperCase() || '?'}
          </span>
        </div>
      {/if}
    {/if}

    <div class="flex-1 min-w-0">
      <div class="flex items-center gap-2 mb-1">
        {#if post.user?.id}
          <a
            href={buildProfileHref(post.user.id)}
            class="font-medium text-gray-900 truncate hover:underline"
            on:click={(event) => handleProfileNavigation(event, post.user?.id)}
          >
            {post.user?.username || 'Unknown'}
          </a>
        {:else}
          <span class="font-medium text-gray-900 truncate">
            {post.user?.username || 'Unknown'}
          </span>
        {/if}
        <span class="text-gray-400 text-sm">·</span>
        <RelativeTime dateString={post.createdAt} className="text-gray-500 text-sm" />
        <EditedBadge createdAt={post.createdAt} updatedAt={post.updatedAt} />
        {#if canEdit}
          <div class="ml-auto relative">
            <button
              type="button"
              class="p-1 rounded-md text-gray-400 hover:text-gray-600 hover:bg-gray-100"
              aria-haspopup="true"
              aria-expanded={menuOpen}
              aria-label="Open post actions"
              on:click|stopPropagation={toggleMenu}
            >
              <svg class="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
                <path d="M6 10a2 2 0 114 0 2 2 0 01-4 0zm6 0a2 2 0 114 0 2 2 0 01-4 0zm-10 0a2 2 0 114 0 2 2 0 01-4 0z" />
              </svg>
            </button>
            {#if menuOpen}
              <div
                class="absolute right-0 mt-2 w-28 rounded-lg border border-gray-200 bg-white shadow-lg py-1 z-20"
                role="menu"
              >
                <button
                  type="button"
                  class="w-full text-left px-3 py-2 text-sm text-gray-700 hover:bg-gray-100"
                  on:click={startEdit}
                  role="menuitem"
                >
                  Edit
                </button>
              </div>
            {/if}
          </div>
        {/if}
      </div>

      {#if isEditing}
        <div class="mb-3 space-y-2">
          <textarea
            class="w-full rounded-lg border border-gray-300 p-2 text-sm text-gray-800 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
            rows="4"
            bind:value={editContent}
          />
          {#if editError}
            <div class="text-sm text-red-600">{editError}</div>
          {/if}
          <div class="flex items-center gap-2">
            <button
              type="button"
              class="px-3 py-1.5 rounded-md bg-blue-600 text-white text-sm hover:bg-blue-700 disabled:opacity-60"
              on:click={saveEdit}
              disabled={isSaving}
            >
              {isSaving ? 'Saving...' : 'Save'}
            </button>
            <button
              type="button"
              class="px-3 py-1.5 rounded-md border border-gray-300 text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-60"
              on:click={cancelEdit}
              disabled={isSaving}
            >
              Cancel
            </button>
          </div>
        </div>
      {:else}
        <LinkifiedText
          text={displayContent}
          className="text-gray-800 whitespace-pre-wrap break-words mb-3"
        />
      {/if}

      {#if !isEditing && link && imageUrl}
        <div class="mb-3 rounded-lg border border-gray-200 overflow-hidden bg-gray-50">
          {#if imageLoadFailed}
            <div class="flex items-center justify-center px-4 py-6 text-sm text-gray-500">
              Image unavailable. Try opening the link directly.
            </div>
          {:else}
            <img
              src={imageUrl}
              alt={metadata?.title || 'Uploaded image'}
              class="w-full max-h-[28rem] object-contain bg-white"
              loading="lazy"
              on:error={() => {
                imageLoadFailed = true;
              }}
            />
          {/if}
        </div>
        {#if !isInternalUploadLink || imageLoadFailed}
          <a
            href={link.url}
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 text-sm break-all"
          >
            <span>🔗</span>
            <span class="underline">{link.url}</span>
          </a>
        {/if}
      {:else if !isEditing && link && metadata}
        <a
          href={link.url}
          target="_blank"
          rel="noopener noreferrer"
          class="block rounded-lg border border-gray-200 overflow-hidden hover:border-gray-300 transition-colors"
        >
          <div class="flex">
            {#if metadata.image}
              <div class="w-24 h-24 flex-shrink-0">
                <img
                  src={metadata.image}
                  alt={metadata.title || 'Link preview'}
                  class="w-full h-full object-cover"
                />
              </div>
            {/if}
            <div class="flex-1 p-3 min-w-0">
              <div class="flex items-center gap-1 mb-1">
                <span>{getProviderIcon(metadata.provider)}</span>
                {#if metadata.provider}
                  <span class="text-xs text-gray-500 capitalize">{metadata.provider}</span>
                {/if}
              </div>
              {#if metadata.title}
                <h4 class="font-medium text-gray-900 text-sm truncate">
                  {metadata.title}
                </h4>
              {/if}
              {#if metadata.description}
                <p class="text-gray-600 text-xs line-clamp-2 mt-0.5">
                  {metadata.description}
                </p>
              {/if}
              {#if metadata.author}
                <p class="text-gray-500 text-xs mt-1">
                  by {metadata.author}
                </p>
              {/if}
            </div>
          </div>
        </a>
      {:else if !isEditing && link}
        <a
          href={link.url}
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 text-sm break-all"
        >
          <span>🔗</span>
          <span class="underline">{link.url}</span>
        </a>
      {/if}

      <div class="flex items-center gap-4 mt-3 text-gray-500 text-sm">
        <div class="flex items-center gap-1">
          <span>💬</span>
          <span>{post.commentCount || 0}</span>
        </div>
        <button
          type="button"
          class="text-xs text-blue-600 hover:text-blue-800"
          on:click={copyThreadLink}
        >
          {copiedLink ? 'Copied!' : 'Copy link'}
        </button>
      </div>

      <div class="mt-3">
        <ReactionBar
          reactionCounts={post.reactionCounts ?? {}}
          userReactions={userReactions}
          onToggle={toggleReaction}
          postId={post.id}
        />
      </div>

      <div class="mt-4 border-t border-gray-200 pt-4">
        <CommentThread
          postId={post.id}
          commentCount={post.commentCount ?? 0}
          {highlightCommentId}
        />
      </div>
    </div>
  </div>
</article>
