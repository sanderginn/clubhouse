import { get } from 'svelte/store';
import { api } from '../services/api';
import { postStore, type Post, type Link } from './postStore';
import { activeSection } from './sectionStore';

interface ApiUser {
  id: string;
  username: string;
  profile_picture_url?: string;
}

interface ApiLink {
  id?: string;
  url: string;
  metadata?: {
    url?: string;
    provider?: string;
    title?: string;
    description?: string;
    image?: string;
    author?: string;
    duration?: number;
    embedUrl?: string;
  };
}

interface ApiPost {
  id: string;
  user_id: string;
  section_id: string;
  content: string;
  links?: ApiLink[];
  user?: ApiUser;
  comment_count?: number;
  created_at: string;
  updated_at?: string;
}

interface FeedResponse {
  posts: ApiPost[];
  has_more: boolean;
  next_cursor?: string;
}

function mapApiPost(apiPost: ApiPost): Post {
  return {
    id: apiPost.id,
    userId: apiPost.user_id,
    sectionId: apiPost.section_id,
    content: apiPost.content,
    links: apiPost.links?.map((l): Link => ({
      id: l.id,
      url: l.url,
      metadata: l.metadata?.url
        ? {
            url: l.metadata.url,
            provider: l.metadata.provider,
            title: l.metadata.title,
            description: l.metadata.description,
            image: l.metadata.image,
            author: l.metadata.author,
            duration: l.metadata.duration,
            embedUrl: l.metadata.embedUrl,
          }
        : undefined,
    })),
    user: apiPost.user
      ? {
          id: apiPost.user.id,
          username: apiPost.user.username,
          profilePictureUrl: apiPost.user.profile_picture_url,
        }
      : undefined,
    commentCount: apiPost.comment_count,
    createdAt: apiPost.created_at,
    updatedAt: apiPost.updated_at,
  };
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
