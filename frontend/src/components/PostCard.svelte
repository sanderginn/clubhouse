<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { Link, Post } from '../stores/postStore';
  import { postStore, currentUser } from '../stores';
  import { api } from '../services/api';
  import CommentThread from './comments/CommentThread.svelte';
  import EditedBadge from './EditedBadge.svelte';
  import ReactionBar from './reactions/ReactionBar.svelte';
  import RelativeTime from './RelativeTime.svelte';
  import { buildProfileHref, handleProfileNavigation } from '../services/profileNavigation';
  import { buildThreadHref } from '../services/routeNavigation';
  import LinkifiedText from './LinkifiedText.svelte';
  import { getImageLinkUrl } from '../services/linkUtils';
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
  let isEditing = false;
  let editContent = '';
  let editError: string | null = null;
  let isSaving = false;
  let editImageAction: 'keep' | 'remove' | 'replace' = 'keep';
  let editImageUploadUrl: string | null = null;
  let editImageUploadError: string | null = null;
  let editImageUploading = false;
  let editImageUploadProgress = 0;
  let editImageInput: HTMLInputElement | null = null;

  const MAX_UPLOAD_BYTES = 10 * 1024 * 1024;
  const MAX_UPLOAD_LABEL = '10 MB';

  const ALLOWED_IMAGE_MIME_TYPES = [
    'image/jpeg',
    'image/png',
    'image/gif',
    'image/webp',
    'image/bmp',
    'image/avif',
    'image/tiff',
  ];
  const ALLOWED_IMAGE_EXTENSIONS = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp', 'avif', 'tif', 'tiff'];
  const ACCEPTED_IMAGE_TYPES = [
    ...ALLOWED_IMAGE_MIME_TYPES,
    ...ALLOWED_IMAGE_EXTENSIONS.map((ext) => `.${ext}`),
  ].join(',');

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

  function startEdit() {
    editContent = post.content;
    editError = null;
    editImageAction = 'keep';
    editImageUploadUrl = null;
    editImageUploadError = null;
    editImageUploading = false;
    editImageUploadProgress = 0;
    isEditing = true;
  }

  function cancelEdit() {
    isEditing = false;
    editContent = post.content;
    editError = null;
    editImageAction = 'keep';
    editImageUploadUrl = null;
    editImageUploadError = null;
    editImageUploading = false;
    editImageUploadProgress = 0;
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
      const linksPayload = buildEditLinksPayload();
      const response = await api.updatePost(post.id, {
        content: trimmed,
        links: linksPayload,
      });
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
  $: canEdit = $currentUser?.id === post.userId;
  $: imageLinks = (post.links ?? []).filter((item) => Boolean(getImageLinkUrl(item)));
  $: originalImageUrl =
    imageLinks.length > 0 ? getImageLinkUrl(imageLinks[0] as Link) : undefined;
  $: editImagePreviewUrl =
    editImageAction === 'replace' && editImageUploadUrl
      ? editImageUploadUrl
      : editImageAction === 'keep'
        ? originalImageUrl
        : undefined;

  let imageLoadFailed = false;
  let lastImageUrl: string | undefined;
  $: if (imageUrl !== lastImageUrl) {
    imageLoadFailed = false;
    lastImageUrl = imageUrl;
  }

  let editImageLoadFailed = false;
  let lastEditImageUrl: string | undefined;
  $: if (editImagePreviewUrl !== lastEditImageUrl) {
    editImageLoadFailed = false;
    lastEditImageUrl = editImagePreviewUrl;
  }

  function validateImageFile(file: File): string | null {
    if (file.type && !ALLOWED_IMAGE_MIME_TYPES.includes(file.type)) {
      return 'Only image files are supported.';
    }
    if (
      !file.type &&
      !new RegExp(`\\.(${ALLOWED_IMAGE_EXTENSIONS.join('|')})$`, 'i').test(file.name)
    ) {
      return 'Only image files are supported.';
    }
    if (file.size > MAX_UPLOAD_BYTES) {
      return `Images must be ${MAX_UPLOAD_LABEL} or smaller.`;
    }
    return null;
  }

  async function handleEditImageSelect(event: Event) {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) {
      return;
    }

    const validationError = validateImageFile(file);
    if (validationError) {
      editImageUploadError = validationError;
      input.value = '';
      return;
    }

    editImageUploading = true;
    editImageUploadProgress = 0;
    editImageUploadError = null;

    try {
      const response = await api.uploadImage(file, (progress) => {
        editImageUploadProgress = progress;
      });
      editImageUploadUrl = response.url;
      editImageAction = 'replace';
    } catch (err) {
      editImageUploadError = err instanceof Error ? err.message : 'Upload failed';
    } finally {
      editImageUploading = false;
      input.value = '';
    }
  }

  function removeEditImage() {
    editImageAction = 'remove';
    editImageUploadUrl = null;
    editImageUploadError = null;
    editImageUploading = false;
    editImageUploadProgress = 0;
  }

  function undoEditImageRemoval() {
    editImageAction = 'keep';
    editImageUploadUrl = null;
    editImageUploadError = null;
    editImageUploading = false;
    editImageUploadProgress = 0;
  }

  function buildEditLinksPayload(): { url: string }[] | null | undefined {
    if (editImageAction === 'keep') {
      return undefined;
    }

    const originalLinks = post.links ?? [];
    const firstImageIndex = originalLinks.findIndex((item) => Boolean(getImageLinkUrl(item)));
    if (firstImageIndex === -1) {
      return undefined;
    }

    if (editImageAction === 'remove') {
      return originalLinks
        .filter((_, index) => index !== firstImageIndex)
        .map((item) => ({ url: item.url }));
    }

    const uploadUrl = editImageUploadUrl;
    if (editImageAction === 'replace' && uploadUrl) {
      return originalLinks.map((item, index) => ({
        url: index === firstImageIndex ? uploadUrl : item.url,
      }));
    }

    return undefined;
  }

  const renderStart = typeof performance !== 'undefined' ? performance.now() : null;
  onMount(() => {
    if (renderStart === null) {
      return;
    }
    recordComponentRender('PostCard', performance.now() - renderStart);
  });
</script>

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
      <div class="flex flex-wrap items-center gap-2 mb-1">
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
        <div class="ml-auto flex items-center gap-2 relative">
          {#if canEdit}
            <button
              type="button"
              class="inline-flex items-center gap-1 rounded-md border border-gray-200 px-2.5 py-1 text-xs font-medium text-gray-600 hover:text-gray-800 hover:bg-gray-50"
              on:click={startEdit}
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
            class="inline-flex items-center gap-1 rounded-md border border-gray-200 px-2.5 py-1 text-xs font-medium text-gray-600 hover:text-gray-800 hover:bg-gray-50"
            on:click={copyThreadLink}
          >
            <svg class="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
              <path
                d="M7 5a3 3 0 013-3h4a3 3 0 013 3v4a3 3 0 01-3 3h-1a1 1 0 110-2h1a1 1 0 001-1V5a1 1 0 00-1-1h-4a1 1 0 00-1 1v1a1 1 0 11-2 0V5z"
              />
              <path
                d="M3 8a3 3 0 013-3h4a3 3 0 013 3v4a3 3 0 01-3 3H6a3 3 0 01-3-3V8zm3-1a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H6z"
              />
            </svg>
            <span>Share</span>
          </button>
          {#if copiedLink}
            <span
              class="absolute -top-6 right-0 rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] text-emerald-700 shadow"
              role="status"
              aria-live="polite"
            >
              Link copied
            </span>
          {/if}
        </div>
      </div>

      {#if isEditing}
        <div class="mb-3 space-y-2">
          <textarea
            class="w-full rounded-lg border border-gray-300 p-2 text-sm text-gray-800 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
            rows="4"
            bind:value={editContent}
          />
          {#if originalImageUrl || editImageAction !== 'keep'}
            <div class="space-y-2">
              {#if editImagePreviewUrl}
                <div class="rounded-lg border border-gray-200 overflow-hidden bg-gray-50">
                  {#if editImageLoadFailed}
                    <div class="flex items-center justify-center px-4 py-6 text-sm text-gray-500">
                      Image unavailable. Try uploading again.
                    </div>
                  {:else}
                    <img
                      src={editImagePreviewUrl}
                      alt="Post preview"
                      class="w-full max-h-[24rem] object-contain bg-white"
                      loading="lazy"
                      on:error={() => {
                        editImageLoadFailed = true;
                      }}
                    />
                  {/if}
                </div>
              {/if}
              <input
                type="file"
                bind:this={editImageInput}
                on:change={handleEditImageSelect}
                accept={ACCEPTED_IMAGE_TYPES}
                class="hidden"
              />
              <div class="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  class="px-2.5 py-1.5 rounded-md border border-gray-300 text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-60"
                  on:click={() => editImageInput?.click()}
                  disabled={isSaving || editImageUploading}
                >
                  Replace image
                </button>
                {#if editImageAction !== 'remove'}
                  <button
                    type="button"
                    class="px-2.5 py-1.5 rounded-md border border-red-200 text-sm text-red-600 hover:bg-red-50 disabled:opacity-60"
                    on:click={removeEditImage}
                    disabled={isSaving || editImageUploading}
                  >
                    Remove image
                  </button>
                {:else}
                  <button
                    type="button"
                    class="px-2.5 py-1.5 rounded-md border border-gray-300 text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-60"
                    on:click={undoEditImageRemoval}
                    disabled={isSaving || editImageUploading}
                  >
                    Keep image
                  </button>
                {/if}
              </div>
              {#if editImageUploading}
                <div class="text-xs text-gray-500">Uploading image... {editImageUploadProgress}%</div>
                <div class="h-1 w-full bg-gray-200 rounded">
                  <div
                    class="h-1 bg-blue-600 rounded"
                    style={`width: ${editImageUploadProgress}%`}
                  ></div>
                </div>
              {/if}
              {#if editImageUploadError}
                <div class="text-sm text-red-600">{editImageUploadError}</div>
              {/if}
              {#if editImageAction === 'remove'}
                <div class="text-xs text-amber-700 bg-amber-50 border border-amber-200 rounded px-2 py-1">
                  Image will be removed when you save.
                </div>
              {:else if editImageAction === 'replace'}
                <div class="text-xs text-blue-700 bg-blue-50 border border-blue-200 rounded px-2 py-1">
                  New image will replace the existing one when you save.
                </div>
              {/if}
            </div>
          {/if}
          {#if editError}
            <div class="text-sm text-red-600">{editError}</div>
          {/if}
          <div class="flex items-center gap-2">
            <button
              type="button"
              class="px-3 py-1.5 rounded-md bg-blue-600 text-white text-sm hover:bg-blue-700 disabled:opacity-60"
              on:click={saveEdit}
              disabled={isSaving || editImageUploading}
            >
              {isSaving ? 'Saving...' : 'Save'}
            </button>
            <button
              type="button"
              class="px-3 py-1.5 rounded-md border border-gray-300 text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-60"
              on:click={cancelEdit}
              disabled={isSaving || editImageUploading}
            >
              Cancel
            </button>
          </div>
        </div>
      {:else}
        <LinkifiedText
          text={post.content}
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
        <a
          href={link.url}
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 text-sm break-all"
        >
          <span>🔗</span>
          <span class="underline">{link.url}</span>
        </a>
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
