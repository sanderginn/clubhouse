import type { Post, Link, LinkMetadata } from './postStore';

export interface ApiUser {
  id: string;
  username: string;
  profile_picture_url?: string;
}

export interface ApiLink {
  id?: string;
  url: string;
  metadata?: Record<string, unknown> | string | null;
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

function normalizeString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function normalizeNumber(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === 'string') {
    const trimmed = value.trim();
    if (trimmed.length === 0) {
      return undefined;
    }
    const parsed = Number(trimmed);
    return Number.isFinite(parsed) ? parsed : undefined;
  }
  return undefined;
}

export function normalizeLinkMetadata(
  rawMetadata: unknown,
  linkUrl: string
): LinkMetadata | undefined {
  if (!rawMetadata) {
    return undefined;
  }

  let metadata: Record<string, unknown> | null = null;
  if (typeof rawMetadata === 'string') {
    try {
      const parsed = JSON.parse(rawMetadata) as unknown;
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        metadata = parsed as Record<string, unknown>;
      }
    } catch {
      return undefined;
    }
  } else if (typeof rawMetadata === 'object' && !Array.isArray(rawMetadata)) {
    metadata = rawMetadata as Record<string, unknown>;
  }

  if (!metadata) {
    return undefined;
  }

  const url = normalizeString(metadata.url) ?? linkUrl;
  const provider =
    normalizeString(metadata.provider) ??
    normalizeString(metadata.site_name) ??
    normalizeString(metadata.siteName);
  const title = normalizeString(metadata.title) ?? normalizeString(metadata.name);
  const description =
    normalizeString(metadata.description) ?? normalizeString(metadata.summary);
  const image =
    normalizeString(metadata.image) ??
    normalizeString(metadata.image_url) ??
    normalizeString(metadata.imageUrl);
  const author = normalizeString(metadata.author) ?? normalizeString(metadata.artist);
  const duration = normalizeNumber(metadata.duration);
  const embedUrl =
    normalizeString(metadata.embedUrl) ?? normalizeString(metadata.embed_url);

  const hasMetadata =
    !!provider || !!title || !!description || !!image || !!author || !!duration || !!embedUrl;
  if (!hasMetadata) {
    return undefined;
  }

  return {
    url,
    provider,
    title,
    description,
    image,
    author,
    duration,
    embedUrl,
  };
}

export function mapApiPost(apiPost: ApiPost): Post {
  return {
    id: apiPost.id,
    userId: apiPost.user_id,
    sectionId: apiPost.section_id,
    content: apiPost.content,
    links: apiPost.links?.map((link): Link => ({
      id: link.id,
      url: link.url,
      metadata: normalizeLinkMetadata(link.metadata, link.url),
    })),
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
