import { get } from 'svelte/store';
import { api } from '../services/api';
import { postStore } from './postStore';
import { activeSection } from './sectionStore';
import { mapApiPost, type ApiPost } from './postMapper';

interface FeedResponse {
  posts: ApiPost[];
  has_more: boolean;
  next_cursor?: string;
}

export async function loadFeed(sectionId: string): Promise<void> {
  postStore.setLoading(true);
  postStore.reset();

  try {
    const response = await api.get<FeedResponse>(
      `/sections/${sectionId}/feed?limit=20`
    );

    const posts = (response.posts || []).map(mapApiPost);
    postStore.setPosts(posts, response.next_cursor || null, response.has_more);
  } catch (err) {
    postStore.setError(err instanceof Error ? err.message : 'Failed to load feed');
  }
}

export async function loadMorePosts(): Promise<void> {
  const state = get(postStore);
  const section = get(activeSection);

  if (state.isLoading || !state.hasMore || !state.cursor || !section) {
    return;
  }

  postStore.setLoading(true);

  try {
    const response = await api.get<FeedResponse>(
      `/sections/${section.id}/feed?limit=20&cursor=${state.cursor}`
    );

    const posts = (response.posts || []).map(mapApiPost);
    postStore.appendPosts(posts, response.next_cursor || null, response.has_more);
  } catch (err) {
    postStore.setError(err instanceof Error ? err.message : 'Failed to load more posts');
  }
}
