import { get } from 'svelte/store';
import { api } from '../services/api';
import { postStore } from './postStore';
import { activeSection } from './sectionStore';
import { mapApiPost, type ApiPost } from './postMapper';

const FEED_PAGE_SIZE = 20;

interface FeedResponse {
  data?: {
    posts?: ApiPost[];
  };
  posts?: ApiPost[];
  has_more?: boolean;
  next_cursor?: string | null;
  meta?: {
    cursor?: string | null;
    has_more?: boolean;
    hasMore?: boolean;
    next_cursor?: string | null;
    nextCursor?: string | null;
  };
}

function extractFeedMeta(response: FeedResponse): { cursor: string | null; hasMore: boolean } {
  const meta = response.meta ?? {};
  const cursor =
    response.next_cursor ??
    meta.cursor ??
    meta.next_cursor ??
    meta.nextCursor ??
    null;
  const hasMore = response.has_more ?? meta.has_more ?? meta.hasMore ?? false;
  return { cursor, hasMore };
}

function extractFeedPosts(response: FeedResponse): ApiPost[] {
  return response.data?.posts ?? response.posts ?? [];
}

export async function loadFeed(sectionId: string): Promise<void> {
  postStore.reset();
  postStore.setLoading(true);
  postStore.setPaginationError(null);

  try {
    const response = await api.get<FeedResponse>(`/sections/${sectionId}/feed?limit=${FEED_PAGE_SIZE}`);

    const posts = extractFeedPosts(response).map(mapApiPost);
    const { cursor, hasMore } = extractFeedMeta(response);
    postStore.setPosts(posts, cursor, hasMore);
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
  postStore.setPaginationError(null);

  try {
    const response = await api.get<FeedResponse>(
      `/sections/${section.id}/feed?limit=${FEED_PAGE_SIZE}&cursor=${state.cursor}`
    );

    const posts = extractFeedPosts(response).map(mapApiPost);
    const { cursor, hasMore } = extractFeedMeta(response);
    postStore.appendPosts(posts, cursor, hasMore);
  } catch (err) {
    postStore.setPaginationError(
      err instanceof Error ? err.message : 'Failed to load more posts'
    );
  }
}
