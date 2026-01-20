import type { Link } from './postStore';
import type { ApiLink, ApiUser } from './postMapper';
import type { Comment } from './commentStore';

export interface ApiComment {
  id: string;
  user_id: string;
  post_id: string;
  parent_comment_id?: string | null;
  content: string;
  links?: ApiLink[];
  user?: ApiUser;
  replies?: ApiComment[];
  created_at: string;
  updated_at?: string;
}

export function mapApiComment(apiComment: ApiComment): Comment {
  return {
    id: apiComment.id,
    userId: apiComment.user_id,
    postId: apiComment.post_id,
    parentCommentId: apiComment.parent_comment_id ?? undefined,
    content: apiComment.content,
    links: apiComment.links?.map((link): Link => ({
      id: link.id,
      url: link.url,
      metadata: link.metadata
        ? {
            url: link.metadata.url ?? link.url,
            provider: link.metadata.provider,
            title: link.metadata.title,
            description: link.metadata.description,
            image: link.metadata.image,
            author: link.metadata.author,
            duration: link.metadata.duration,
            embedUrl: link.metadata.embedUrl,
          }
        : undefined,
    })),
    user: apiComment.user
      ? {
          id: apiComment.user.id,
          username: apiComment.user.username,
          profilePictureUrl: apiComment.user.profile_picture_url,
        }
      : undefined,
    replies: apiComment.replies?.map(mapApiComment) ?? [],
    createdAt: apiComment.created_at,
    updatedAt: apiComment.updated_at,
  };
}
