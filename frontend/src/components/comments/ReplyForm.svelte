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
  export let parentCommentId: string;
  export let allowTimestamp = false;

  const dispatch = createEventDispatcher<{ submit: Comment; cancel: void }>();

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
        parentCommentId,
        content: content.trim(),
        timestampSeconds,
        mentionUsernames,
      });
      const reply = mapApiComment(response.comment);
      const skipIncrement = commentStore.consumeSeenComment(postId, reply.id);
      commentStore.addReply(postId, parentCommentId, reply);
      if (!skipIncrement) {
        postStore.incrementCommentCount(postId, 1);
      }
      content = '';
      timestampInput = '';
      mentionUsernames = [];
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
  <MentionTextarea
    id={`reply-${parentCommentId}`}
    name={`reply-${parentCommentId}`}
    bind:value={content}
    bind:mentionUsernames
    on:keydown={(event) => handleKeyDown(event.detail)}
    ariaLabel="Write a reply"
    placeholder="Write a reply..."
    rows={2}
    disabled={isSubmitting}
    className="w-full px-3 py-2 border border-gray-300 rounded-lg resize-none focus:ring-2 focus:ring-primary focus:border-transparent disabled:opacity-50 disabled:bg-gray-100"
  />
  {#if allowTimestamp}
    <div class="space-y-1">
      <label class="text-xs font-medium text-gray-600" for={`reply-timestamp-${parentCommentId}`}>
        Timestamp (mm:ss or hh:mm:ss)
      </label>
      <input
        id={`reply-timestamp-${parentCommentId}`}
        name={`reply-timestamp-${parentCommentId}`}
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
      <p class="text-xs text-gray-500">Optional. Reference a specific moment in the track.</p>
    </div>
  {/if}
  <p class="text-xs text-gray-500">Tip: Use \@ to write a literal @.</p>

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
