<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { Link, Post } from '../stores/postStore';
  import { postStore, currentUser, isAdmin } from '../stores';
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
  import { lockBodyScroll, unlockBodyScroll } from '../lib/scrollLock';

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
  let isImageLightboxOpen = false;
  let lightboxImageUrl: string | null = null;
  let lightboxAltText = 'Full size image';
  let isDeleting = false;
  let deleteError: string | null = null;

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
    if (isImageLightboxOpen) {
      unlockBodyScroll();
    }
  });

  function openImageLightbox(url: string, altText: string) {
    if (!url) {
      return;
    }
    if (!isImageLightboxOpen) {
      lockBodyScroll();
    }
    lightboxImageUrl = url;
    lightboxAltText = altText || 'Full size image';
    isImageLightboxOpen = true;
  }

  function closeImageLightbox() {
    if (!isImageLightboxOpen) {
      return;
    }
    isImageLightboxOpen = false;
    lightboxImageUrl = null;
    unlockBodyScroll();
  }

  function handleLightboxKeydown(event: KeyboardEvent) {
    if (!isImageLightboxOpen) {
      return;
    }
    if (event.key === 'Escape') {
      closeImageLightbox();
    }
  }

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

  $: imageLinks = (post.links ?? []).filter((item) => Boolean(getImageLinkUrl(item)));
  $: imageItems = imageLinks
    .map((item) => ({
      link: item,
      url: getImageLinkUrl(item),
      title: item.metadata?.title || 'Uploaded image',
    }))
    .filter((item): item is { link: Link; url: string; title: string } => Boolean(item.url));
  $: primaryLink = post.links?.[0];
  $: primaryLinkIsImage = primaryLink ? Boolean(getImageLinkUrl(primaryLink)) : false;
  $: metadata = primaryLink?.metadata;
  $: primaryImageUrl = imageItems.length > 0 ? imageItems[0].url : undefined;
  $: isInternalUploadLink =
    imageItems.length > 0 ? isInternalUploadUrl(imageItems[0].link.url) : false;
  $: displayContent =
    !isEditing && primaryImageUrl && isInternalUploadLink
      ? stripInternalUploadUrls(post.content)
      : post.content;
  $: canEdit = $currentUser?.id === post.userId;
  $: canDelete = $currentUser?.id === post.userId || $isAdmin;
  $: originalImageUrl =
    imageLinks.length > 0 ? getImageLinkUrl(imageLinks[0] as Link) : undefined;
  $: editImagePreviewUrl =
    editImageAction === 'replace' && editImageUploadUrl
      ? editImageUploadUrl
      : editImageAction === 'keep'
        ? originalImageUrl
        : undefined;
  $: activeImageItem = imageItems[activeImageIndex];
  $: activeImageUrl = activeImageItem?.url;
  $: activeImageLink = activeImageItem?.link;
  $: activeImageTitle = activeImageItem?.title ?? 'Uploaded image';
  $: activeImageFailed = imageLoadFailures.has(activeImageIndex);
  $: isActiveImageInternal = activeImageLink
    ? isInternalUploadUrl(activeImageLink.url)
    : false;

  let activeImageIndex = 0;
  let loadedImageIndices = new Set<number>();
  let imageLoadFailures = new Set<number>();
  let lastImageSignature = '';

  $: {
    const signature = imageItems.map((item) => item.url).join('|');
    if (signature !== lastImageSignature) {
      activeImageIndex = 0;
      loadedImageIndices = new Set();
      imageLoadFailures = new Set();
      if (imageItems.length > 0) {
        loadedImageIndices.add(0);
      }
      lastImageSignature = signature;
    }
  }

  function markImageLoaded(index: number) {
    if (!loadedImageIndices.has(index)) {
      const next = new Set(loadedImageIndices);
      next.add(index);
      loadedImageIndices = next;
    }
  }

  function markImageFailed(index: number) {
    if (!imageLoadFailures.has(index)) {
      const next = new Set(imageLoadFailures);
      next.add(index);
      imageLoadFailures = next;
    }
  }

  function shouldLoadImage(index: number): boolean {
    return loadedImageIndices.has(index) || index === activeImageIndex;
  }

  function goToImage(index: number) {
    if (imageItems.length === 0) {
      return;
    }
    const clamped = (index + imageItems.length) % imageItems.length;
    markImageLoaded(clamped);
    if (imageItems.length > 1) {
      markImageLoaded((clamped + 1) % imageItems.length);
      markImageLoaded((clamped - 1 + imageItems.length) % imageItems.length);
    }
    activeImageIndex = clamped;
  }

  function nextImage() {
    goToImage(activeImageIndex + 1);
  }

  function previousImage() {
    goToImage(activeImageIndex - 1);
  }

  function handleCarouselKeydown(event: KeyboardEvent) {
    if (imageItems.length <= 1) {
      return;
    }
    if (event.key === 'ArrowLeft') {
      event.preventDefault();
      previousImage();
    }
    if (event.key === 'ArrowRight') {
      event.preventDefault();
      nextImage();
    }
  }

  let touchStartX = 0;
  let touchStartY = 0;
  let touchActive = false;

  function handleTouchStart(event: TouchEvent) {
    if (imageItems.length <= 1) {
      return;
    }
    const touch = event.touches[0];
    if (!touch) {
      return;
    }
    touchStartX = touch.clientX;
    touchStartY = touch.clientY;
    touchActive = true;
  }

  function handleTouchEnd(event: TouchEvent) {
    if (!touchActive || imageItems.length <= 1) {
      touchActive = false;
      return;
    }
    const touch = event.changedTouches[0];
    if (!touch) {
      touchActive = false;
      return;
    }
    const deltaX = touch.clientX - touchStartX;
    const deltaY = touch.clientY - touchStartY;
    touchActive = false;
    if (Math.abs(deltaX) > 40 && Math.abs(deltaX) > Math.abs(deltaY) * 1.5) {
      if (deltaX < 0) {
        nextImage();
      } else {
        previousImage();
      }
    }
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

  async function deletePost() {
    if (typeof window !== 'undefined') {
      const confirmed = window.confirm('Delete this post?');
      if (!confirmed) {
        return;
      }
    }

    isDeleting = true;
    deleteError = null;

    try {
      await api.deletePost(post.id);
      postStore.removePost(post.id);
    } catch (err) {
      deleteError = err instanceof Error ? err.message : 'Failed to delete post';
      logError('Failed to delete post', { postId: post.id }, err);
    } finally {
      isDeleting = false;
    }
  }

  const renderStart = typeof performance !== 'undefined' ? performance.now() : null;
  onMount(() => {
    if (renderStart === null) {
      return;
    }
    recordComponentRender('PostCard', performance.now() - renderStart);
  });
</script>

<svelte:window on:keydown={handleLightboxKeydown} />

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
          src={post.user?.profilePictureUrl}
          alt={post.user?.username}
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
          {#if canDelete}
            <button
              type="button"
              class="inline-flex items-center gap-1 rounded-md border border-red-200 px-2.5 py-1 text-xs font-medium text-red-600 hover:text-red-700 hover:bg-red-50 disabled:opacity-60"
              on:click={deletePost}
              disabled={isDeleting}
            >
              <svg class="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                <path
                  d="M6 7a1 1 0 011 1v6a1 1 0 11-2 0V8a1 1 0 011-1zm4 0a1 1 0 011 1v6a1 1 0 11-2 0V8a1 1 0 011-1zm-1-5a1 1 0 00-1 1v1H5a1 1 0 000 2h10a1 1 0 100-2h-3V3a1 1 0 00-1-1H9z"
                />
              </svg>
              <span>{isDeleting ? 'Deleting...' : 'Delete'}</span>
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

      {#if deleteError}
        <div class="mb-2 text-xs text-red-600">{deleteError}</div>
      {/if}

      {#if isEditing}
        <div class="mb-3 space-y-4">
          {#if originalImageUrl || editImageAction !== 'keep'}
            <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 space-y-3">
              <div class="text-xs font-semibold uppercase tracking-wide text-gray-500">
                Image
              </div>
              {#if editImagePreviewUrl}
                <div class="rounded-lg border border-gray-200 overflow-hidden bg-white">
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
                  class="w-full sm:w-auto px-2.5 py-1.5 rounded-md border border-gray-300 text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-60"
                  on:click={() => editImageInput?.click()}
                  disabled={isSaving || editImageUploading}
                >
                  Replace image
                </button>
                {#if editImageAction !== 'remove'}
                  <button
                    type="button"
                    class="w-full sm:w-auto px-2.5 py-1.5 rounded-md border border-red-200 text-sm text-red-600 hover:bg-red-50 disabled:opacity-60"
                    on:click={removeEditImage}
                    disabled={isSaving || editImageUploading}
                  >
                    Remove image
                  </button>
                {:else}
                  <button
                    type="button"
                    class="w-full sm:w-auto px-2.5 py-1.5 rounded-md border border-gray-300 text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-60"
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
          text={displayContent}
          className="text-gray-800 whitespace-pre-wrap break-words mb-3"
        />
      {/if}

      {#if !isEditing && imageItems.length > 0}
        {#if imageItems.length === 1}
          <div class="mb-3 rounded-lg border border-gray-200 overflow-hidden bg-gray-50">
            {#if activeImageFailed}
              <div class="flex items-center justify-center px-4 py-6 text-sm text-gray-500">
                Image unavailable. Try opening the link directly.
              </div>
            {:else if activeImageUrl}
              <button
                type="button"
                class="w-full text-left"
                aria-label="Open full-size image"
                aria-haspopup="dialog"
                on:click={() => openImageLightbox(activeImageUrl, activeImageTitle)}
              >
                <img
                  src={activeImageUrl}
                  alt={activeImageTitle}
                  class="w-full max-h-[28rem] object-contain bg-white"
                  loading="lazy"
                  on:error={() => {
                    markImageFailed(0);
                  }}
                />
              </button>
            {/if}
          </div>
        {:else}
          <div class="mb-3">
            <div class="relative rounded-lg border border-gray-200 overflow-hidden bg-gray-50">
              <!-- svelte-ignore a11y-no-noninteractive-tabindex a11y-no-noninteractive-element-interactions -->
              <div
                class="relative"
                tabindex="0"
                role="region"
                aria-roledescription="carousel"
                aria-label="Post images"
                on:keydown={handleCarouselKeydown}
                on:touchstart={handleTouchStart}
                on:touchend={handleTouchEnd}
                on:touchcancel={() => {
                  touchActive = false;
                }}
                style="touch-action: pan-y;"
              >
                <div
                  class="flex transition-transform duration-300 ease-out"
                  style={`transform: translateX(-${activeImageIndex * 100}%);`}
                >
                  {#each imageItems as item, index}
                    <div class="w-full flex-shrink-0">
                      {#if imageLoadFailures.has(index)}
                        <div class="flex items-center justify-center px-4 py-6 text-sm text-gray-500">
                          Image unavailable. Try opening the link directly.
                        </div>
                      {:else}
                        <button
                          type="button"
                          class="w-full text-left"
                          aria-label={`Open image ${index + 1} in full size`}
                          aria-haspopup="dialog"
                          on:click={() => openImageLightbox(item.url, item.title)}
                        >
                          <img
                            src={shouldLoadImage(index) ? item.url : undefined}
                            alt={item.title}
                            class="w-full max-h-[28rem] object-contain bg-white"
                            loading={index === activeImageIndex ? 'eager' : 'lazy'}
                            on:error={() => {
                              markImageFailed(index);
                            }}
                          />
                        </button>
                      {/if}
                    </div>
                  {/each}
                </div>

                <div class="absolute inset-y-0 left-2 hidden sm:flex items-center">
                  <button
                    type="button"
                    class="flex h-9 w-9 items-center justify-center rounded-full bg-white/80 text-gray-700 shadow hover:bg-white"
                    aria-label="Previous image"
                    on:click={previousImage}
                  >
                    ‹
                  </button>
                </div>
                <div class="absolute inset-y-0 right-2 hidden sm:flex items-center">
                  <button
                    type="button"
                    class="flex h-9 w-9 items-center justify-center rounded-full bg-white/80 text-gray-700 shadow hover:bg-white"
                    aria-label="Next image"
                    on:click={nextImage}
                  >
                    ›
                  </button>
                </div>

                <div class="absolute bottom-2 left-0 right-0 flex items-center justify-center">
                  <span class="rounded-full bg-black/60 px-2 py-1 text-xs text-white">
                    {activeImageIndex + 1}/{imageItems.length}
                  </span>
                </div>
              </div>
            </div>
            <div class="mt-2 flex items-center justify-center gap-1">
              {#each imageItems as _, index}
                <button
                  type="button"
                  class={`h-2.5 w-2.5 rounded-full transition-colors ${
                    index === activeImageIndex ? 'bg-gray-900' : 'bg-gray-300'
                  }`}
                  aria-label={`Go to image ${index + 1} of ${imageItems.length}`}
                  aria-current={index === activeImageIndex ? 'true' : 'false'}
                  on:click={() => goToImage(index)}
                ></button>
              {/each}
            </div>
          </div>
        {/if}
        {#if activeImageLink && (!isActiveImageInternal || activeImageFailed)}
          <a
            href={activeImageLink.url}
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 text-sm break-all"
          >
            <span>🔗</span>
            <span class="underline">{activeImageLink.url}</span>
          </a>
        {/if}
        {#if primaryLink && metadata && !primaryLinkIsImage}
          <a
            href={primaryLink.url}
            target="_blank"
            rel="noopener noreferrer"
            class="mt-3 block rounded-lg border border-gray-200 overflow-hidden hover:border-gray-300 transition-colors"
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
        {/if}
      {:else if !isEditing && primaryLink && metadata}
        <a
          href={primaryLink.url}
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
      {:else if !isEditing && primaryLink}
        <a
          href={primaryLink.url}
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 text-sm break-all"
        >
          <span>🔗</span>
          <span class="underline">{primaryLink.url}</span>
        </a>
      {/if}

      <div class="mt-3 flex flex-wrap items-center gap-2">
        <div class="inline-flex items-center gap-1 rounded-full border border-gray-200 bg-white px-2 py-1 text-xs text-gray-600">
          <span>💬</span>
          <span>{post.commentCount || 0}</span>
        </div>
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

{#if isImageLightboxOpen && lightboxImageUrl}
  <div class="fixed inset-0 z-50 flex items-center justify-center px-4 py-6">
    <button
      type="button"
      class="absolute inset-0 bg-black/70"
      aria-label="Close image"
      on:click={closeImageLightbox}
    ></button>
    <div
      class="relative z-10 max-h-full max-w-full"
      role="dialog"
      aria-modal="true"
      aria-label="Full size image"
    >
      <button
        type="button"
        class="absolute -top-3 -right-3 flex h-8 w-8 items-center justify-center rounded-full bg-white text-gray-700 shadow-md hover:bg-gray-100"
        aria-label="Close image"
        on:click={closeImageLightbox}
      >
        ✕
      </button>
      <img
        src={lightboxImageUrl}
        alt={lightboxAltText}
        class="max-h-[85vh] w-auto max-w-[95vw] rounded-lg object-contain bg-white shadow-lg"
        style="touch-action: pinch-zoom;"
      />
    </div>
  </div>
{/if}
