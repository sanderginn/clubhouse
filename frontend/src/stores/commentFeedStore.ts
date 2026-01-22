import { get } from 'svelte/store';
import { api } from '../services/api';
import { commentStore } from './commentStore';
import { mapApiComment } from './commentMapper';

export async function loadThreadComments(postId: string): Promise<void> {
  commentStore.setLoading(postId, true);

  try {
    const response = await api.getThreadComments(postId, 50);
    const comments = (response.comments ?? []).map(mapApiComment);
    commentStore.setThread(postId, comments, response.meta?.cursor ?? null, !!response.meta?.has_more);
  } catch (err) {
    commentStore.setError(postId, err instanceof Error ? err.message : 'Failed to load comments');
  }
}

export async function loadMoreThreadComments(postId: string): Promise<void> {
  const state = get(commentStore);
  const thread = state[postId];
  if (!thread || thread.isLoading || !thread.hasMore || !thread.cursor) {
    return;
  }

  commentStore.setLoading(postId, true);

  try {
    const response = await api.getThreadComments(postId, 50, thread.cursor);
    const comments = (response.comments ?? []).map(mapApiComment);
    commentStore.appendThread(
      postId,
      comments,
      response.meta?.cursor ?? null,
      !!response.meta?.has_more
    );
  } catch (err) {
    commentStore.setError(postId, err instanceof Error ? err.message : 'Failed to load more comments');
  }
}
