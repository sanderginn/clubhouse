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
  reaction_counts?: Record<string, number>;
  viewer_reactions?: string[];
  created_at: string;
  updated_at?: string;
}

export function mapApiPost(apiPost: ApiPost): Post {
  return {
    id: apiPost.id,
    userId: apiPost.user_id,
    sectionId: apiPost.section_id,
    content: apiPost.content,
    links: apiPost.links?.map((l): Link => {
      const metadata = l.metadata;
      const hasMetadata = metadata && Object.keys(metadata).length > 0;

      return {
        id: l.id,
        url: l.url,
        metadata: hasMetadata
          ? {
              url: metadata.url ?? l.url,
              provider: metadata.provider,
              title: metadata.title,
              description: metadata.description,
              image: metadata.image,
              author: metadata.author,
              duration: metadata.duration,
              embedUrl: metadata.embedUrl,
            }
          : undefined,
      };
    }),
    user: apiPost.user
      ? {
          id: apiPost.user.id,
          username: apiPost.user.username,
          profilePictureUrl: apiPost.user.profile_picture_url,
        }
      : undefined,
    commentCount: apiPost.comment_count,
    reactionCounts: apiPost.reaction_counts ?? undefined,
    viewerReactions: apiPost.viewer_reactions,
    createdAt: apiPost.created_at,
    updatedAt: apiPost.updated_at,
  };
}
