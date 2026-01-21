<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { api } from '../../services/api';
  import { activeSection, postStore, currentUser } from '../../stores';
  import type { Link, LinkMetadata } from '../../stores/postStore';
  import LinkPreview from './LinkPreview.svelte';

  const dispatch = createEventDispatcher<{
    submit: void;
  }>();

  let content = '';
  let isSubmitting = false;
  let error: string | null = null;

  let linkUrl = '';
  let linkMetadata: LinkMetadata | null = null;
  let isLoadingPreview = false;
  let previewError: string | null = null;

  let fileInput: HTMLInputElement;
  let selectedFiles: File[] = [];

  const URL_REGEX = /https?:\/\/[^\s<>"{}|\\^`[\]]+/gi;

  function extractUrls(text: string): string[] {
    const matches = text.match(URL_REGEX);
    return matches ? [...new Set(matches)] : [];
  }

  async function handleContentChange() {
    const urls = extractUrls(content);
    if (urls.length > 0 && !linkMetadata && !linkUrl) {
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

  function removeLink() {
    linkUrl = '';
    linkMetadata = null;
    previewError = null;
  }

  function handleFileSelect(event: Event) {
    const input = event.target as HTMLInputElement;
    if (input.files) {
      selectedFiles = [...selectedFiles, ...Array.from(input.files)];
    }
    input.value = '';
  }

  function removeFile(index: number) {
    selectedFiles = selectedFiles.filter((_, i) => i !== index);
  }

  function formatFileSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }

  async function handleSubmit() {
    if (!content.trim() || !$activeSection || !$currentUser) {
      return;
    }

    isSubmitting = true;
    error = null;

    try {
      const links = linkMetadata ? [{ url: linkMetadata.url }] : [];

      const response = await api.createPost({
        sectionId: $activeSection.id,
        content: content.trim(),
        links: links.length > 0 ? links : undefined,
      });

      const createdPost = linkMetadata
        ? {
            ...response.post,
            links: mergeLinkMetadata(response.post.links, linkMetadata),
          }
        : response.post;

      postStore.addPost(createdPost);

      content = '';
      linkUrl = '';
      linkMetadata = null;
      selectedFiles = [];

      dispatch('submit');
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to create post';
    } finally {
      isSubmitting = false;
    }
  }

  function mergeLinkMetadata(links: Link[] | undefined, metadata: LinkMetadata): Link[] {
    if (!links || links.length === 0) {
      return [{ url: metadata.url, metadata }];
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
</script>

<form on:submit|preventDefault={handleSubmit} class="space-y-4">
  <div>
    <label for="post-content" class="sr-only">Post content</label>
    <textarea
      id="post-content"
      bind:value={content}
      on:input={handleContentChange}
      on:keydown={handleKeyDown}
      placeholder={$activeSection
        ? `Share something in ${$activeSection.name}...`
        : 'Share something...'}
      rows="3"
      disabled={isSubmitting}
      class="w-full px-4 py-3 border border-gray-300 rounded-lg resize-none focus:ring-2 focus:ring-primary focus:border-transparent disabled:opacity-50 disabled:bg-gray-100"
    />
  </div>

  {#if linkMetadata}
    <LinkPreview metadata={linkMetadata} onRemove={removeLink} />
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

  {#if selectedFiles.length > 0}
    <div class="space-y-2">
      {#each selectedFiles as file, index}
        <div
          class="flex items-center justify-between p-2 bg-gray-50 border border-gray-200 rounded"
        >
          <div class="flex items-center gap-2 min-w-0">
            <svg
              class="w-5 h-5 text-gray-400 flex-shrink-0"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"
              />
            </svg>
            <span class="text-sm text-gray-700 truncate">{file.name}</span>
            <span class="text-xs text-gray-500 flex-shrink-0">
              ({formatFileSize(file.size)})
            </span>
          </div>
          <button
            type="button"
            on:click={() => removeFile(index)}
            class="p-1 text-gray-400 hover:text-gray-600"
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
      {/each}
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
        accept="image/*"
        class="hidden"
      />
      <button
        type="button"
        on:click={() => fileInput.click()}
        disabled={isSubmitting}
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
        on:click={() => {
          const url = prompt('Enter a link URL:');
          if (url) {
            linkUrl = url;
            fetchLinkPreview();
          }
        }}
        disabled={isSubmitting || !!linkMetadata}
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
      disabled={!content.trim() || isSubmitting || !$activeSection}
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
