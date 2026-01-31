<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { api } from '../../services/api';
  import { commentStore } from '../../stores/commentStore';
  import { postStore } from '../../stores/postStore';
  import { mapApiComment } from '../../stores/commentMapper';
  import type { Comment } from '../../stores/commentStore';
  import MentionTextarea from '../mentions/MentionTextarea.svelte';

  export let postId: string;
  export let imageContext:
    | {
        id: string;
        url: string;
        index: number;
        altText?: string;
      }
    | null = null;
  export let onClearImageContext: (() => void) | null = null;

  const dispatch = createEventDispatcher<{ submit: Comment }>();

  let content = '';
  let isSubmitting = false;
  let error: string | null = null;

  async function handleSubmit() {
    if (!content.trim()) {
      return;
    }

    isSubmitting = true;
    error = null;

    try {
      const response = await api.createComment({
        postId,
        imageId: imageContext?.id,
        content: content.trim(),
      });
      const comment = mapApiComment(response.comment);
      const skipIncrement = commentStore.consumeSeenComment(postId, comment.id);
      commentStore.addComment(postId, comment);
      if (!skipIncrement) {
        postStore.incrementCommentCount(postId, 1);
      }
      content = '';
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
    on:keydown={(event) => handleKeyDown(event.detail)}
    ariaLabel="Add a comment"
    placeholder="Add a comment..."
    rows={2}
    disabled={isSubmitting}
    className="w-full px-3 py-2 border border-gray-300 rounded-lg resize-none focus:ring-2 focus:ring-primary focus:border-transparent disabled:opacity-50 disabled:bg-gray-100"
  />
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
