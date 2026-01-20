import type { Post, Link } from './postStore';

export interface ApiUser {
  id: string;
  username: string;
  profile_picture_url?: string;
}

export interface ApiLink {
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

export interface ApiPost {
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

export function mapApiPost(apiPost: ApiPost): Post {
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
