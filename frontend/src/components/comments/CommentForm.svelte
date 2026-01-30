<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { api } from '../../services/api';
  import { commentStore } from '../../stores/commentStore';
  import { postStore } from '../../stores/postStore';
  import { mapApiComment } from '../../stores/commentMapper';
  import type { Comment } from '../../stores/commentStore';
  import MentionTextarea from '../mentions/MentionTextarea.svelte';

  export let postId: string;

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
        content: content.trim(),
      });
      const comment = mapApiComment(response.comment);
      const skipIncrement = commentStore.consumeSeenComment(postId, comment.id);
      commentStore.addComment(postId, comment);
      if (!skipIncrement) {
        postStore.incrementCommentCount(postId, 1);
      }
      content = '';
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
