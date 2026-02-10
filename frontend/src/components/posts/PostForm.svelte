<script lang="ts">
  import { createEventDispatcher, onDestroy, tick } from 'svelte';
  import { api } from '../../services/api';
  import { activeSection, postStore, currentUser } from '../../stores';
  import { loadSectionLinks } from '../../stores/sectionLinksFeedStore';
  import type {
    Highlight,
    Link,
    LinkMetadata,
    PodcastHighlightEpisode,
    PodcastMetadataInput,
  } from '../../stores/postStore';
  import LinkPreview from './LinkPreview.svelte';
  import MentionTextarea from '../mentions/MentionTextarea.svelte';
  import HighlightEditor from './HighlightEditor.svelte';
  import RecipeCard from '../recipes/RecipeCard.svelte';

  const dispatch = createEventDispatcher<{
    submit: void;
  }>();

  let content = '';
  let mentionUsernames: string[] = [];
  let isSubmitting = false;
  let error: string | null = null;

  let linkUrl = '';
  let linkMetadata: LinkMetadata | null = null;
  let isLoadingPreview = false;
  let previewError: string | null = null;
  let isLinkInputVisible = false;
  let linkInputValue = '';
  let linkInputError: string | null = null;
  let linkInputRef: HTMLInputElement | null = null;
  let highlights: Highlight[] = [];
  let lastHighlightLinkInput = '';

  let fileInput: HTMLInputElement;
  type UploadItem = {
    id: string;
    file: File;
    progress: number;
    status: 'pending' | 'uploading' | 'done' | 'error';
    error?: string | null;
    url?: string;
    previewUrl?: string | null;
  };

  const MAX_UPLOAD_BYTES = 10 * 1024 * 1024;
  const MAX_UPLOAD_LABEL = '10 MB';
  const MAX_IMAGE_COUNT = 10;

  let selectedFiles: UploadItem[] = [];
  let uploadLimitError: string | null = null;
  let isParsingRecipe = false;
  let parseRecipeError: string | null = null;
  let podcastKind: '' | 'show' | 'episode' = '';
  let podcastKindSelectionRequired = false;
  let podcastHighlightEpisodes: PodcastHighlightEpisode[] = [];
  let podcastEpisodeTitle = '';
  let podcastEpisodeUrl = '';
  let podcastEpisodeNote = '';
  let podcastEpisodeError: string | null = null;

  const PODCAST_KIND_SELECTION_REQUIRED_MESSAGE =
    'Could not determine whether this podcast link is a show or an episode. Please select one and try again.';
  const MAX_PODCAST_HIGHLIGHT_EPISODES = 10;

  const URL_REGEX = /https?:\/\/[^\s<>"{}|\\^`[\]]+/gi;
  $: hasLink = Boolean((linkMetadata && linkMetadata.url) || linkUrl.trim());
  $: isMusicSection = $activeSection?.type === 'music';
  $: isRecipeSection = $activeSection?.type === 'recipe';
  $: isPodcastSection = $activeSection?.type === 'podcast';
  $: showHighlightEditor = isMusicSection && hasLink;
  $: showPodcastHighlightEpisodeEditor = isPodcastSection && hasLink && podcastKind === 'show';
  $: podcastKindBlocked = isPodcastSection && hasLink && podcastKindSelectionRequired && !podcastKind;
  $: linkInputValueNormalized = linkUrl.trim();
  $: if (linkInputValueNormalized !== lastHighlightLinkInput) {
    highlights = [];
    lastHighlightLinkInput = linkInputValueNormalized;
  }
  $: if (!showHighlightEditor && highlights.length > 0) {
    highlights = [];
  }
  $: if (podcastKindSelectionRequired && podcastKind) {
    podcastKindSelectionRequired = false;
    if (error === PODCAST_KIND_SELECTION_REQUIRED_MESSAGE) {
      error = null;
    }
  }
  $: if (podcastKind !== 'show' && (podcastHighlightEpisodes.length > 0 || podcastEpisodeError)) {
    podcastHighlightEpisodes = [];
    podcastEpisodeError = null;
  }
  $: if (
    podcastKind !== 'show' &&
    (podcastEpisodeTitle || podcastEpisodeUrl || podcastEpisodeNote)
  ) {
    podcastEpisodeTitle = '';
    podcastEpisodeUrl = '';
    podcastEpisodeNote = '';
  }
  $: if (
    !isPodcastSection &&
    (podcastKind ||
      podcastKindSelectionRequired ||
      podcastHighlightEpisodes.length > 0 ||
      podcastEpisodeTitle ||
      podcastEpisodeUrl ||
      podcastEpisodeNote ||
      podcastEpisodeError)
  ) {
    podcastKind = '';
    podcastKindSelectionRequired = false;
    podcastHighlightEpisodes = [];
    podcastEpisodeTitle = '';
    podcastEpisodeUrl = '';
    podcastEpisodeNote = '';
    podcastEpisodeError = null;
  }
  $: hasUploads = selectedFiles.some((item) => item.status !== 'error');
  $: canSubmit =
    Boolean($activeSection) && (content.trim().length > 0 || hasLink || hasUploads) && !podcastKindBlocked;

  function createUploadId(): string {
    if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
      return crypto.randomUUID();
    }
    return `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  }

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

  function isLikelyImageFile(file: File): boolean {
    if (file.type && ALLOWED_IMAGE_MIME_TYPES.includes(file.type)) {
      return true;
    }
    return new RegExp(`\\.(${ALLOWED_IMAGE_EXTENSIONS.join('|')})$`, 'i').test(file.name);
  }

  function validateFile(file: File): string | null {
    if (!isLikelyImageFile(file)) {
      return 'Only image files are supported.';
    }
    if (file.size > MAX_UPLOAD_BYTES) {
      return `Images must be ${MAX_UPLOAD_LABEL} or smaller.`;
    }
    return null;
  }

  function updateUpload(id: string, patch: Partial<UploadItem>) {
    selectedFiles = selectedFiles.map((item) => (item.id === id ? { ...item, ...patch } : item));
  }

  function canCreateObjectUrl(): boolean {
    return typeof URL !== 'undefined' && typeof URL.createObjectURL === 'function';
  }

  function createPreviewUrl(file: File, validationError: string | null): string | null {
    if (validationError || !canCreateObjectUrl()) {
      return null;
    }
    return URL.createObjectURL(file);
  }

  function revokePreviewUrl(item: UploadItem) {
    if (!item.previewUrl || typeof URL === 'undefined' || typeof URL.revokeObjectURL !== 'function') {
      return;
    }
    URL.revokeObjectURL(item.previewUrl);
  }

  function extractUrls(text: string): string[] {
    const matches = text.match(URL_REGEX);
    return matches ? [...new Set(matches)] : [];
  }

  async function handleContentChange() {
    const urls = extractUrls(content);
    if (urls.length > 0 && !linkMetadata && !linkUrl && !isLinkInputVisible && !linkInputValue) {
      linkUrl = urls[0];
      await fetchLinkPreview();
    }
  }

  async function fetchLinkPreview() {
    if (!linkUrl.trim()) {
      linkMetadata = null;
      return;
    }

    isLoadingPreview = true;
    previewError = null;

    try {
      const response = await api.previewLink(linkUrl.trim());
      linkMetadata = response.metadata;
    } catch (err) {
      previewError = err instanceof Error ? err.message : 'Failed to load preview';
      linkMetadata = null;
    } finally {
      isLoadingPreview = false;
    }
  }

  async function parseRecipe() {
    if (!linkMetadata?.url || isParsingRecipe) {
      return;
    }

    isParsingRecipe = true;
    parseRecipeError = null;

    try {
      const response = await api.parseRecipe(linkMetadata.url);
      linkMetadata = {
        ...linkMetadata,
        ...response.metadata,
        recipe: response.metadata.recipe ?? linkMetadata.recipe,
      };
    } catch (err) {
      parseRecipeError = err instanceof Error ? err.message : 'Failed to parse recipe';
    } finally {
      isParsingRecipe = false;
    }
  }

  function removeLink() {
    linkUrl = '';
    linkMetadata = null;
    previewError = null;
    parseRecipeError = null;
    isParsingRecipe = false;
    linkInputValue = '';
    linkInputError = null;
    isLinkInputVisible = false;
    highlights = [];
    lastHighlightLinkInput = '';
    podcastKind = '';
    podcastKindSelectionRequired = false;
    podcastHighlightEpisodes = [];
    podcastEpisodeTitle = '';
    podcastEpisodeUrl = '';
    podcastEpisodeNote = '';
    podcastEpisodeError = null;
  }

  function addPodcastHighlightEpisode() {
    if (podcastKind !== 'show') {
      return;
    }
    if (podcastHighlightEpisodes.length >= MAX_PODCAST_HIGHLIGHT_EPISODES) {
      podcastEpisodeError = `You can add up to ${MAX_PODCAST_HIGHLIGHT_EPISODES} highlighted episodes.`;
      return;
    }

    const title = podcastEpisodeTitle.trim();
    let url = podcastEpisodeUrl.trim();
    const note = podcastEpisodeNote.trim();
    if (!title) {
      podcastEpisodeError = 'Episode title is required.';
      return;
    }
    if (!url) {
      podcastEpisodeError = 'Episode URL is required.';
      return;
    }
    if (!/^https?:\/\//i.test(url)) {
      url = `https://${url}`;
    }
    if (!isValidUrl(url)) {
      podcastEpisodeError = 'Episode URL must be a valid http(s) URL.';
      return;
    }

    podcastEpisodeError = null;
    podcastHighlightEpisodes = [
      ...podcastHighlightEpisodes,
      {
        title,
        url,
        ...(note ? { note } : {}),
      },
    ];
    podcastEpisodeTitle = '';
    podcastEpisodeUrl = '';
    podcastEpisodeNote = '';
  }

  function removePodcastHighlightEpisode(index: number) {
    podcastHighlightEpisodes = podcastHighlightEpisodes.filter((_, episodeIndex) => episodeIndex !== index);
    if (podcastHighlightEpisodes.length < MAX_PODCAST_HIGHLIGHT_EPISODES) {
      podcastEpisodeError = null;
    }
  }

  function buildPodcastMetadata(linkValue: string): PodcastMetadataInput | undefined {
    if (!isPodcastSection || !linkValue) {
      return undefined;
    }

    const payload: PodcastMetadataInput = {};
    if (podcastKind) {
      payload.kind = podcastKind;
    }
    if (podcastKind === 'show' && podcastHighlightEpisodes.length > 0) {
      payload.highlightEpisodes = podcastHighlightEpisodes.map((episode) => ({
        title: episode.title.trim(),
        url: episode.url.trim(),
        ...(episode.note?.trim() ? { note: episode.note.trim() } : {}),
      }));
    }
    return payload;
  }

  function isValidUrl(value: string): boolean {
    try {
      const parsed = new URL(value);
      return parsed.protocol === 'http:' || parsed.protocol === 'https:';
    } catch {
      return false;
    }
  }

  async function openLinkInput() {
    if (isSubmitting || linkMetadata || linkUrl) {
      return;
    }
    isLinkInputVisible = true;
    linkInputError = null;
    await tick();
    linkInputRef?.focus();
    linkInputRef?.select();
  }

  function closeLinkInput() {
    isLinkInputVisible = false;
    linkInputError = null;
    linkInputValue = '';
  }

  async function submitLinkInput() {
    let value = linkInputValue.trim();
    if (!value) {
      linkInputError = 'Enter a link URL.';
      return;
    }

    if (!/^https?:\/\//i.test(value)) {
      value = `https://${value}`;
    }

    if (!isValidUrl(value)) {
      linkInputError = 'Enter a valid http(s) URL.';
      return;
    }

    linkInputError = null;
    linkUrl = value;
    linkInputValue = value;
    isLinkInputVisible = false;
    await fetchLinkPreview();
  }

  function handleLinkInputKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter') {
      event.preventDefault();
      submitLinkInput();
    }
  }

  function handleFileSelect(event: Event) {
    const input = event.target as HTMLInputElement;
    if (input.files) {
      uploadLimitError = null;
      const remainingSlots = MAX_IMAGE_COUNT - selectedFiles.length;
      const files = Array.from(input.files);
      const acceptedFiles = remainingSlots > 0 ? files.slice(0, remainingSlots) : [];
      const next = acceptedFiles.map((file) => {
        const validationError = validateFile(file);
        return {
          id: createUploadId(),
          file,
          progress: 0,
          status: validationError ? 'error' : 'pending',
          error: validationError,
          previewUrl: createPreviewUrl(file, validationError),
        } as UploadItem;
      });
      if (files.length > remainingSlots) {
        uploadLimitError = `You can upload up to ${MAX_IMAGE_COUNT} images per post.`;
      }
      selectedFiles = [...selectedFiles, ...next];
    }
    input.value = '';
  }

  function removeFile(index: number) {
    const removed = selectedFiles[index];
    if (removed) {
      revokePreviewUrl(removed);
    }
    selectedFiles = selectedFiles.filter((_, i) => i !== index);
    if (selectedFiles.length < MAX_IMAGE_COUNT) {
      uploadLimitError = null;
    }
  }

  function moveFile(from: number, to: number) {
    if (to < 0 || to >= selectedFiles.length || from === to) {
      return;
    }
    const next = [...selectedFiles];
    const [moved] = next.splice(from, 1);
    next.splice(to, 0, moved);
    selectedFiles = next;
  }

  function formatFileSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }

  async function handleSubmit() {
    const trimmedContent = content.trim();
    const linkValue = (linkMetadata?.url ?? linkUrl).trim();

    if ((!trimmedContent && !hasLink && !hasUploads) || !$activeSection || !$currentUser) {
      return;
    }

    if (podcastKindBlocked) {
      error = PODCAST_KIND_SELECTION_REQUIRED_MESSAGE;
      return;
    }

    if (selectedFiles.some((item) => item.status === 'error')) {
      error = 'Remove invalid files before posting.';
      return;
    }

    isSubmitting = true;
    error = null;

    try {
      const uploadedUrls: string[] = [];

      for (const item of selectedFiles) {
        if (item.status === 'done' && item.url) {
          uploadedUrls.push(item.url);
          continue;
        }
        if (item.status !== 'pending') {
          continue;
        }

        updateUpload(item.id, { status: 'uploading', progress: 0, error: null });

        try {
          const response = await api.uploadImage(item.file, (progress) =>
            updateUpload(item.id, { progress })
          );
          updateUpload(item.id, { status: 'done', progress: 100, url: response.url });
          uploadedUrls.push(response.url);
        } catch (err) {
          updateUpload(item.id, {
            status: 'error',
            error: err instanceof Error ? err.message : 'Upload failed',
          });
        }
      }

      if (selectedFiles.some((item) => item.status === 'error')) {
        error = 'Some uploads failed. Remove the failed files and try again.';
        return;
      }

      const images = uploadedUrls.map((url) => ({ url }));
      const includeHighlights = showHighlightEditor && highlights.length > 0;
      const podcast = buildPodcastMetadata(linkValue);
      const links = linkValue
        ? [
            {
              url: linkValue,
              ...(includeHighlights ? { highlights } : {}),
              ...(podcast ? { podcast } : {}),
            },
          ]
        : [];

      const payload = {
        sectionId: $activeSection.id,
        content: trimmedContent,
        mentionUsernames,
      } as {
        sectionId: string;
        content: string;
        links?: { url: string; highlights?: Highlight[]; podcast?: PodcastMetadataInput }[];
        images?: { url: string }[];
        mentionUsernames?: string[];
      };

      if (links.length > 0) {
        payload.links = links;
      }
      if (images.length > 0) {
        payload.images = images;
      }

      const response = await api.createPost(payload);

      const createdPost =
        linkMetadata && uploadedUrls.length === 0
          ? {
              ...response.post,
              links: mergeLinkMetadata(response.post.links, linkMetadata, highlights),
            }
          : response.post;

      postStore.addPost(createdPost);
      if (isMusicSection && links.length > 0) {
        loadSectionLinks($activeSection.id);
      }

      content = '';
      mentionUsernames = [];
      linkUrl = '';
      linkMetadata = null;
      linkInputValue = '';
      linkInputError = null;
      isLinkInputVisible = false;
      highlights = [];
      lastHighlightLinkInput = '';
      podcastKind = '';
      podcastKindSelectionRequired = false;
      podcastHighlightEpisodes = [];
      podcastEpisodeTitle = '';
      podcastEpisodeUrl = '';
      podcastEpisodeNote = '';
      podcastEpisodeError = null;
      selectedFiles.forEach(revokePreviewUrl);
      selectedFiles = [];
      uploadLimitError = null;

      dispatch('submit');
    } catch (err) {
      const requestError = err as Error & { podcastKindSelectionRequired?: boolean };
      if (requestError.podcastKindSelectionRequired) {
        podcastKindSelectionRequired = true;
      }
      error = err instanceof Error ? err.message : 'Failed to create post';
    } finally {
      isSubmitting = false;
    }
  }

  function mergeLinkMetadata(
    links: Link[] | undefined,
    metadata: LinkMetadata,
    nextHighlights?: Highlight[]
  ): Link[] {
    if (!links || links.length === 0) {
      return [
        {
          url: metadata.url,
          metadata,
          ...(nextHighlights && nextHighlights.length > 0 ? { highlights: nextHighlights } : {}),
        },
      ];
    }
    return links.map((link, index) => {
      if (index !== 0) {
        return link;
      }
      return {
        ...link,
        url: link.url || metadata.url,
        metadata: link.metadata ?? metadata,
      };
    });
  }

  function handleKeyDown(event: KeyboardEvent) {
    if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
      handleSubmit();
    }
  }

  onDestroy(() => {
    selectedFiles.forEach(revokePreviewUrl);
  });
</script>

<form on:submit|preventDefault={handleSubmit} class="space-y-4">
  <div>
    <label for="post-content" class="sr-only">Post content</label>
    <MentionTextarea
      id="post-content"
      name="post-content"
      bind:value={content}
      bind:mentionUsernames
      on:input={handleContentChange}
      on:keydown={(event) => handleKeyDown(event.detail)}
      placeholder={$activeSection
        ? `Share something in ${$activeSection.name}...`
        : 'Share something...'}
      rows={3}
      disabled={isSubmitting}
      ariaLabel="Post content"
      className="w-full px-4 py-3 border border-gray-300 rounded-lg resize-none focus:ring-2 focus:ring-primary focus:border-transparent disabled:opacity-50 disabled:bg-gray-100"
    />
    <p class="mt-2 text-xs text-gray-500">Tip: Use \@ to write a literal @.</p>
  </div>

  {#if linkMetadata}
    {#if isRecipeSection}
      <LinkPreview metadata={linkMetadata} onRemove={removeLink}>
        <div slot="footer" class="flex flex-wrap items-center gap-3">
          <button
            type="button"
            class="inline-flex items-center gap-2 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-700 hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-60"
            on:click={parseRecipe}
            disabled={isParsingRecipe}
          >
            {#if isParsingRecipe}
              <svg class="h-3.5 w-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
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
              Parsing...
            {:else}
              {linkMetadata.recipe ? 'Re-parse recipe' : 'Parse recipe'}
            {/if}
          </button>
          {#if parseRecipeError}
            <span class="text-xs text-red-600">{parseRecipeError}</span>
          {/if}
        </div>
      </LinkPreview>
      {#if linkMetadata.recipe}
        <RecipeCard
          recipe={linkMetadata.recipe}
          fallbackImage={linkMetadata.image}
          fallbackTitle={linkMetadata.title}
        />
      {/if}
    {:else}
      <LinkPreview metadata={linkMetadata} onRemove={removeLink} />
    {/if}
  {:else if isLoadingPreview}
    <div class="flex items-center gap-2 p-3 bg-gray-50 border border-gray-200 rounded-lg">
      <svg class="w-5 h-5 text-gray-400 animate-spin" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
        />
      </svg>
      <span class="text-sm text-gray-500">Loading link preview...</span>
    </div>
  {:else if previewError}
    <div class="flex items-center justify-between p-3 bg-red-50 border border-red-200 rounded-lg">
      <span class="text-sm text-red-600">{previewError}</span>
      <button
        type="button"
        on:click={removeLink}
        class="text-sm text-red-600 hover:text-red-800 font-medium"
      >
        Dismiss
      </button>
    </div>
  {/if}

  {#if showHighlightEditor}
    <div class="space-y-3 rounded-lg border border-gray-200 bg-gray-50 p-3">
      <div class="text-sm font-medium text-gray-700">Highlights</div>
      <HighlightEditor bind:highlights disabled={isSubmitting} />
    </div>
  {/if}

  {#if isPodcastSection && hasLink}
    <div class="space-y-3 rounded-lg border border-gray-200 bg-gray-50 p-3">
      <div class="space-y-1">
        <label for="podcast-kind" class="text-sm font-medium text-gray-700">Podcast kind</label>
        <select
          id="podcast-kind"
          bind:value={podcastKind}
          disabled={isSubmitting}
          class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-transparent focus:ring-2 focus:ring-primary disabled:cursor-not-allowed disabled:bg-gray-100"
          aria-label="Podcast kind"
        >
          <option value="">Auto-detect from link</option>
          <option value="show">Show</option>
          <option value="episode">Episode</option>
        </select>
        <p class="text-xs text-gray-500">Choose a kind manually only if auto-detection fails.</p>
        {#if podcastKindSelectionRequired}
          <p class="text-xs text-red-600">{PODCAST_KIND_SELECTION_REQUIRED_MESSAGE}</p>
        {/if}
      </div>

      {#if showPodcastHighlightEpisodeEditor}
        <div class="space-y-3 rounded-lg border border-gray-200 bg-white p-3">
          <div class="text-sm font-medium text-gray-700">Highlighted episodes</div>
          <div class="grid gap-2 sm:grid-cols-2">
            <div class="space-y-1">
              <label for="podcast-episode-title" class="text-xs font-medium text-gray-600">Title</label>
              <input
                id="podcast-episode-title"
                type="text"
                bind:value={podcastEpisodeTitle}
                disabled={isSubmitting}
                placeholder="Episode title"
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-transparent focus:ring-2 focus:ring-primary disabled:bg-gray-100"
                aria-label="Highlight episode title"
              />
            </div>
            <div class="space-y-1">
              <label for="podcast-episode-url" class="text-xs font-medium text-gray-600">URL</label>
              <input
                id="podcast-episode-url"
                type="url"
                bind:value={podcastEpisodeUrl}
                disabled={isSubmitting}
                placeholder="https://..."
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-transparent focus:ring-2 focus:ring-primary disabled:bg-gray-100"
                aria-label="Highlight episode url"
              />
            </div>
          </div>
          <div class="space-y-1">
            <label for="podcast-episode-note" class="text-xs font-medium text-gray-600">
              Note (optional)
            </label>
            <input
              id="podcast-episode-note"
              type="text"
              bind:value={podcastEpisodeNote}
              disabled={isSubmitting}
              placeholder="Why this episode stands out"
              class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-transparent focus:ring-2 focus:ring-primary disabled:bg-gray-100"
              aria-label="Highlight episode note"
            />
          </div>
          <button
            type="button"
            on:click={addPodcastHighlightEpisode}
            disabled={isSubmitting}
            class="inline-flex items-center rounded-lg border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50"
          >
            Add highlighted episode
          </button>
          {#if podcastEpisodeError}
            <p class="text-xs text-red-600">{podcastEpisodeError}</p>
          {/if}

          {#if podcastHighlightEpisodes.length > 0}
            <ul class="space-y-2">
              {#each podcastHighlightEpisodes as episode, index}
                <li class="rounded-lg border border-gray-200 bg-gray-50 p-2">
                  <div class="flex items-start justify-between gap-2">
                    <div class="min-w-0">
                      <p class="truncate text-sm font-medium text-gray-700">{episode.title}</p>
                      <p class="truncate text-xs text-gray-500">{episode.url}</p>
                      {#if episode.note}
                        <p class="mt-1 text-xs text-gray-600">{episode.note}</p>
                      {/if}
                    </div>
                    <button
                      type="button"
                      on:click={() => removePodcastHighlightEpisode(index)}
                      disabled={isSubmitting}
                      class="rounded-md p-1 text-gray-400 hover:bg-white hover:text-gray-600 disabled:opacity-40"
                      aria-label={`Remove highlighted episode ${index + 1}`}
                    >
                      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M6 18L18 6M6 6l12 12"
                        />
                      </svg>
                    </button>
                  </div>
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      {/if}
    </div>
  {/if}

  {#if isLinkInputVisible && !linkMetadata}
    <div class="space-y-2">
      <div class="flex flex-col sm:flex-row gap-2">
        <div class="flex-1">
          <label for="post-link" class="sr-only">Link URL</label>
          <input
            id="post-link"
            type="url"
            bind:this={linkInputRef}
            bind:value={linkInputValue}
            on:keydown={handleLinkInputKeydown}
            placeholder="Paste a link (https://...)"
            disabled={isSubmitting || isLoadingPreview}
            class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary focus:border-transparent disabled:opacity-50 disabled:bg-gray-100"
          />
        </div>
        <button
          type="button"
          on:click={submitLinkInput}
          disabled={isSubmitting || isLoadingPreview}
          class="px-3 py-2 bg-primary text-white font-medium rounded-lg hover:bg-secondary transition-colors disabled:opacity-50"
        >
          Add link
        </button>
        <button
          type="button"
          on:click={closeLinkInput}
          disabled={isSubmitting || isLoadingPreview}
          class="px-3 py-2 border border-gray-200 text-gray-600 font-medium rounded-lg hover:bg-gray-50 transition-colors disabled:opacity-50"
        >
          Cancel
        </button>
      </div>
      {#if linkInputError}
        <p class="text-xs text-red-600">{linkInputError}</p>
      {:else}
        <p class="text-xs text-gray-500">We’ll fetch a preview after you add the link.</p>
      {/if}
    </div>
  {/if}

  {#if selectedFiles.length > 0}
    <div class="space-y-2">
      <div class="flex items-center justify-between text-xs text-gray-500">
        <span>{selectedFiles.length} of {MAX_IMAGE_COUNT} images selected</span>
        <span>Reorder to choose the primary image</span>
      </div>
      <div class="grid gap-3 sm:grid-cols-2">
        {#each selectedFiles as item, index}
          <div class="relative p-3 border border-gray-200 rounded-lg bg-gray-50">
            <div class="flex items-start gap-3">
              <div class="relative w-20 h-20 rounded-md overflow-hidden bg-gray-200 flex-shrink-0">
                {#if item.previewUrl}
                  <img
                    src={item.previewUrl}
                    alt={`Selected image ${index + 1}: ${item.file.name}`}
                    class="w-full h-full object-cover"
                  />
                {:else}
                  <div class="w-full h-full flex items-center justify-center text-gray-400 text-xs">
                    No preview
                  </div>
                {/if}
                {#if index === 0}
                  <span
                    class="absolute top-1 left-1 text-[10px] uppercase tracking-wide bg-primary text-white px-1.5 py-0.5 rounded"
                  >
                    Primary
                  </span>
                {/if}
              </div>
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2">
                  <span class="text-sm text-gray-700 font-medium truncate" data-testid="upload-filename">
                    {item.file.name}
                  </span>
                  <span class="text-xs text-gray-500 flex-shrink-0">
                    {formatFileSize(item.file.size)}
                  </span>
                </div>
                <div class="mt-1 flex items-center gap-2 text-xs">
                  {#if item.status === 'uploading'}
                    <span class="text-gray-500">{item.progress}%</span>
                  {:else if item.status === 'done'}
                    <span class="text-green-600">Uploaded</span>
                  {:else if item.status === 'error'}
                    <span class="text-red-600">Error</span>
                  {:else}
                    <span class="text-gray-400">Ready</span>
                  {/if}
                </div>
                {#if item.status === 'uploading'}
                  <div class="mt-2 h-1 w-full bg-gray-200 rounded">
                    <div
                      class="h-1 bg-primary rounded"
                      style={`width: ${item.progress}%`}
                    ></div>
                  </div>
                {:else if item.status === 'error' && item.error}
                  <p class="mt-2 text-xs text-red-600">{item.error}</p>
                {/if}
              </div>
              <div class="flex flex-col gap-1">
                <button
                  type="button"
                  on:click={() => moveFile(index, index - 1)}
                  class="p-2 text-gray-400 hover:text-gray-600 hover:bg-white rounded-md disabled:opacity-40"
                  disabled={index === 0 || item.status === 'uploading'}
                  aria-label="Move image up"
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M5 15l7-7 7 7"
                    />
                  </svg>
                </button>
                <button
                  type="button"
                  on:click={() => moveFile(index, index + 1)}
                  class="p-2 text-gray-400 hover:text-gray-600 hover:bg-white rounded-md disabled:opacity-40"
                  disabled={index === selectedFiles.length - 1 || item.status === 'uploading'}
                  aria-label="Move image down"
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M19 9l-7 7-7-7"
                    />
                  </svg>
                </button>
                <button
                  type="button"
                  on:click={() => removeFile(index)}
                  class="p-2 text-gray-400 hover:text-gray-600 hover:bg-white rounded-md disabled:opacity-40"
                  disabled={item.status === 'uploading'}
                  aria-label="Remove file"
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M6 18L18 6M6 6l12 12"
                    />
                  </svg>
                </button>
              </div>
            </div>
          </div>
        {/each}
      </div>
    </div>
  {/if}

  {#if uploadLimitError}
    <div class="p-3 bg-amber-50 border border-amber-200 rounded-lg">
      <p class="text-sm text-amber-700">{uploadLimitError}</p>
    </div>
  {/if}

  {#if error}
    <div class="p-3 bg-red-50 border border-red-200 rounded-lg">
      <p class="text-sm text-red-600">{error}</p>
    </div>
  {/if}

  <div class="flex items-center justify-between">
    <div class="flex items-center gap-2">
      <input
        type="file"
        bind:this={fileInput}
        on:change={handleFileSelect}
        multiple
        accept={ACCEPTED_IMAGE_TYPES}
        class="hidden"
      />
      <button
        type="button"
        on:click={() => fileInput.click()}
        disabled={isSubmitting || selectedFiles.length >= MAX_IMAGE_COUNT}
        class="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-lg transition-colors disabled:opacity-50"
        aria-label="Attach file"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
          />
        </svg>
      </button>
      <button
        type="button"
        on:click={openLinkInput}
        disabled={isSubmitting || isLoadingPreview || !!linkMetadata || !!linkUrl}
        class="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-lg transition-colors disabled:opacity-50"
        aria-label="Add link"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"
          />
        </svg>
      </button>
    </div>

    <button
      type="submit"
      disabled={!canSubmit || isSubmitting}
      class="px-4 py-2 bg-primary text-white font-medium rounded-lg hover:bg-secondary transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
    >
      {#if isSubmitting}
        <svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
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
        <span>Posting...</span>
      {:else}
        <span>Post</span>
      {/if}
    </button>
  </div>

  <p class="text-xs text-gray-500">
    Press <kbd class="px-1.5 py-0.5 bg-gray-100 border border-gray-200 rounded text-xs">⌘</kbd>
    + <kbd class="px-1.5 py-0.5 bg-gray-100 border border-gray-200 rounded text-xs">Enter</kbd>
    to post
  </p>
</form>
