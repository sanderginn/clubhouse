import type { Link } from './postStore';
import { normalizeLinkMetadata, type ApiLink, type ApiUser } from './postMapper';
import type { Comment } from './commentStore';

export interface ApiComment {
  id: string;
  user_id: string;
  post_id: string;
  parent_comment_id?: string | null;
  image_id?: string | null;
  content: string;
  links?: ApiLink[];
  user?: ApiUser;
  replies?: ApiComment[];
  reaction_counts?: Record<string, number>;
  viewer_reactions?: string[];
  created_at: string;
  updated_at?: string;
}

export function mapApiComment(apiComment: ApiComment): Comment {
  return {
    id: apiComment.id,
    userId: apiComment.user_id,
    postId: apiComment.post_id,
    parentCommentId: apiComment.parent_comment_id ?? undefined,
    imageId: apiComment.image_id ?? undefined,
    content: apiComment.content,
    links: apiComment.links?.map((link): Link => ({
      id: link.id,
      url: link.url,
      metadata: normalizeLinkMetadata(link.metadata, link.url),
    })),
    user: apiComment.user
      ? {
          id: apiComment.user.id,
          username: apiComment.user.username,
          profilePictureUrl: apiComment.user.profile_picture_url,
        }
      : undefined,
    replies: apiComment.replies?.map(mapApiComment) ?? [],
    reactionCounts: apiComment.reaction_counts ?? undefined,
    viewerReactions: apiComment.viewer_reactions,
    createdAt: apiComment.created_at,
    updatedAt: apiComment.updated_at,
  };
}
