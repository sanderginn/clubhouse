<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { api } from '../../services/api';
  import { commentStore } from '../../stores/commentStore';
  import { postStore } from '../../stores/postStore';
  import { mapApiComment } from '../../stores/commentMapper';
  import type { Comment } from '../../stores/commentStore';

  export let postId: string;
  export let parentCommentId: string;

  const dispatch = createEventDispatcher<{ submit: Comment; cancel: void }>();

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
        parentCommentId,
        content: content.trim(),
      });
      const reply = mapApiComment(response.comment);
      commentStore.addReply(postId, parentCommentId, reply);
      postStore.incrementCommentCount(postId, 1);
      content = '';
      dispatch('submit', reply);
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to add reply';
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
  <label class="sr-only" for={`reply-${parentCommentId}`}>Reply</label>
  <textarea
    id={`reply-${parentCommentId}`}
    bind:value={content}
    on:keydown={handleKeyDown}
    placeholder="Write a reply..."
    rows="2"
    disabled={isSubmitting}
    class="w-full px-3 py-2 border border-gray-300 rounded-lg resize-none focus:ring-2 focus:ring-primary focus:border-transparent disabled:opacity-50 disabled:bg-gray-100"
  ></textarea>

  {#if error}
    <div class="p-2 bg-red-50 border border-red-200 rounded-lg">
      <p class="text-sm text-red-600">{error}</p>
    </div>
  {/if}

  <div class="flex items-center justify-between">
    <button
      type="button"
      on:click={() => dispatch('cancel')}
      class="text-xs text-gray-500 hover:text-gray-700"
    >
      Cancel
    </button>
    <button
      type="submit"
      disabled={isSubmitting || !content.trim()}
      class="px-3 py-1.5 bg-primary text-white text-sm font-medium rounded-lg hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
    >
      {isSubmitting ? 'Posting...' : 'Reply'}
    </button>
  </div>
</form>
