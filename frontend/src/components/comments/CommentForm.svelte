<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { api } from '../../services/api';
  import { commentStore } from '../../stores/commentStore';
  import { postStore } from '../../stores/postStore';
  import { mapApiComment } from '../../stores/commentMapper';
  import type { Comment } from '../../stores/commentStore';
  import { parseHighlightTimestamp } from '../../lib/highlights';
  import MentionTextarea from '../mentions/MentionTextarea.svelte';

  export let postId: string;
  export let allowTimestamp = false;
  export let imageContext:
    | {
        id?: string;
        url: string;
        index: number;
        altText?: string;
      }
    | null = null;
  export let onClearImageContext: (() => void) | null = null;

  const dispatch = createEventDispatcher<{ submit: Comment }>();

  let content = '';
  let timestampInput = '';
  let timestampError: string | null = null;
  let mentionUsernames: string[] = [];
  let isSubmitting = false;
  let error: string | null = null;

  const maxCommentTimestampSeconds = 21600;

  async function handleSubmit() {
    if (!content.trim()) {
      return;
    }

    isSubmitting = true;
    error = null;
    timestampError = null;

    let timestampSeconds: number | undefined;
    if (allowTimestamp && timestampInput.trim()) {
      const parsed = parseHighlightTimestamp(timestampInput);
      if (parsed === null) {
        timestampError = 'Enter a timestamp in mm:ss or hh:mm:ss format.';
        isSubmitting = false;
        return;
      }
      if (parsed > maxCommentTimestampSeconds) {
        timestampError = 'Timestamp is too long.';
        isSubmitting = false;
        return;
      }
      timestampSeconds = parsed;
    }

    try {
      const response = await api.createComment({
        postId,
        imageId: imageContext?.id,
        content: content.trim(),
        timestampSeconds,
        mentionUsernames,
      });
      const comment = mapApiComment(response.comment);
      const skipIncrement = commentStore.consumeSeenComment(postId, comment.id);
      commentStore.addComment(postId, comment);
      if (!skipIncrement) {
        postStore.incrementCommentCount(postId, 1);
      }
      content = '';
      timestampInput = '';
      mentionUsernames = [];
      onClearImageContext?.();
      dispatch('submit', comment);
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to add comment';
    } finally {
      isSubmitting = false;
    }
  }

  function handleKeyDown(event: KeyboardEvent) {
    if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
      handleSubmit();
    }
  }
</script>

<form on:submit|preventDefault={handleSubmit} class="space-y-2">
  {#if imageContext}
    <div class="flex items-center gap-3 rounded-lg border border-blue-200 bg-blue-50 px-3 py-2">
      <img
        src={imageContext.url}
        alt={imageContext.altText ?? `Image ${imageContext.index + 1}`}
        class="h-10 w-10 rounded-md object-cover border border-blue-200 bg-white"
      />
      <div class="flex-1">
        <p class="text-xs font-medium text-blue-700">Replying to image {imageContext.index + 1}</p>
        {#if imageContext.altText}
          <p class="text-xs text-blue-600 truncate">{imageContext.altText}</p>
        {/if}
      </div>
      <button
        type="button"
        class="text-xs text-blue-700 hover:text-blue-900"
        on:click={() => onClearImageContext?.()}
      >
        Clear
      </button>
    </div>
  {/if}
  <MentionTextarea
    id="comment-content"
    bind:value={content}
    bind:mentionUsernames
    on:keydown={(event) => handleKeyDown(event.detail)}
    ariaLabel="Add a comment"
    placeholder="Add a comment..."
    rows={2}
    disabled={isSubmitting}
    className="w-full px-3 py-2 border border-gray-300 rounded-lg resize-none focus:ring-2 focus:ring-primary focus:border-transparent disabled:opacity-50 disabled:bg-gray-100"
  />
  {#if allowTimestamp}
    <div class="space-y-1">
      <label class="text-xs font-medium text-gray-600" for="comment-timestamp">
        Timestamp (mm:ss or hh:mm:ss)
      </label>
      <input
        id="comment-timestamp"
        type="text"
        bind:value={timestampInput}
        placeholder="02:30"
        class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm text-gray-800 focus:border-primary focus:ring-2 focus:ring-primary/30"
        aria-invalid={timestampError ? 'true' : 'false'}
        disabled={isSubmitting}
      />
      {#if timestampError}
        <p class="text-xs text-red-600">{timestampError}</p>
      {/if}
      <p class="text-xs text-gray-500">Optional. Use this to reference a specific moment in the track.</p>
    </div>
  {/if}
  <p class="text-xs text-gray-500">Tip: Use \@ to write a literal @.</p>

  {#if error}
    <div class="p-2 bg-red-50 border border-red-200 rounded-lg">
      <p class="text-sm text-red-600">{error}</p>
    </div>
  {/if}

  <div class="flex items-center justify-between">
    <p class="text-xs text-gray-500">Press ⌘+Enter to post</p>
    <button
      type="submit"
      disabled={isSubmitting || !content.trim()}
      class="px-3 py-1.5 bg-primary text-white text-sm font-medium rounded-lg hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
    >
      {isSubmitting ? 'Posting...' : 'Comment'}
    </button>
  </div>
</form>
