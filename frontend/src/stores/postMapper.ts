import type {
  Post,
  Link,
  LinkMetadata,
  LinkEmbed,
  PostImage,
  Highlight,
  RecipeStats,
} from './postStore';

export interface ApiUser {
  id: string;
  username: string;
  profile_picture_url?: string;
}

export interface ApiLink {
  id?: string;
  url: string;
  metadata?: Record<string, unknown> | string | null;
  highlights?: unknown;
}

export interface ApiPostImage {
  id: string;
  url: string;
  position: number;
  caption?: string | null;
  alt_text?: string | null;
  created_at?: string;
}

export interface ApiPost {
  id: string;
  user_id: string;
  section_id: string;
  content: string;
  links?: ApiLink[];
  images?: ApiPostImage[];
  user?: ApiUser;
  comment_count?: number;
  reaction_counts?: Record<string, number>;
  viewer_reactions?: string[];
  recipe_stats?: ApiRecipeStats | null;
  recipeStats?: ApiRecipeStats | null;
  created_at: string;
  updated_at?: string;
}

export interface ApiRecipeStats {
  save_count?: number | null;
  cook_count?: number | null;
  avg_rating?: number | null;
  average_rating?: number | null;
  saveCount?: number | null;
  cookCount?: number | null;
  avgRating?: number | null;
  averageRating?: number | null;
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

function normalizeRecipeStats(rawStats: unknown): RecipeStats | undefined {
  if (!rawStats || typeof rawStats !== 'object') {
    return undefined;
  }
  const record = rawStats as Record<string, unknown>;
  const saveCount = normalizeNumber(record.save_count ?? record.saveCount) ?? 0;
  const cookCount = normalizeNumber(record.cook_count ?? record.cookCount) ?? 0;
  const averageRating =
    normalizeNumber(
      record.avg_rating ??
        record.avgRating ??
        record.average_rating ??
        record.averageRating
    ) ?? null;

  return {
    saveCount,
    cookCount,
    averageRating,
  };
}

function normalizeEmbed(record: Record<string, unknown>): LinkEmbed | undefined {
  const rawEmbed = record.embed;
  let embedRecord: Record<string, unknown> | null = null;

  if (rawEmbed && typeof rawEmbed === 'object' && !Array.isArray(rawEmbed)) {
    embedRecord = rawEmbed as Record<string, unknown>;
  }

  const embedUrl =
    normalizeString(embedRecord?.embed_url ?? embedRecord?.embedUrl ?? embedRecord?.url) ??
    normalizeString(record.embed_url ?? record.embedUrl);
  if (!embedUrl) {
    return undefined;
  }

  return {
    url: embedUrl,
    provider:
      normalizeString(embedRecord?.provider) ??
      normalizeString(record.embed_provider ?? record.embedProvider),
    type: normalizeString(embedRecord?.type),
    height:
      normalizeNumber(embedRecord?.height) ??
      normalizeNumber(record.embed_height ?? record.embedHeight),
    width:
      normalizeNumber(embedRecord?.width) ??
      normalizeNumber(record.embed_width ?? record.embedWidth),
  };
}

function normalizeHighlights(rawHighlights: unknown): Highlight[] | undefined {
  if (!Array.isArray(rawHighlights)) {
    return undefined;
  }

  const normalized = rawHighlights
    .map((item) => {
      if (!item || typeof item !== 'object') {
        return null;
      }
      const record = item as { timestamp?: unknown; label?: unknown };
      if (typeof record.timestamp !== 'number' || !Number.isFinite(record.timestamp)) {
        return null;
      }
      const label =
        typeof record.label === 'string' && record.label.trim().length > 0
          ? record.label
          : undefined;
      return {
        timestamp: record.timestamp,
        ...(label ? { label } : {}),
      } as Highlight;
    })
    .filter(Boolean) as Highlight[];

  return normalized.length > 0 ? normalized : undefined;
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
  const embedUrl = normalizeString(metadata.embedUrl) ?? normalizeString(metadata.embed_url);
  const embed = normalizeEmbed(metadata);
  const resolvedEmbedUrl = embed?.url ?? embedUrl;
  const type =
    normalizeString(metadata.type) ??
    normalizeString(metadata.og_type) ??
    normalizeString(metadata.ogType);

  const hasMetadata =
    !!provider ||
    !!title ||
    !!description ||
    !!image ||
    !!author ||
    !!duration ||
    !!resolvedEmbedUrl ||
    !!type;
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
    embedUrl: resolvedEmbedUrl,
    embed,
    type,
  };
}

export function mapApiPost(apiPost: ApiPost): Post {
  const images: PostImage[] | undefined = apiPost.images?.map((image) => ({
    id: image.id,
    url: image.url,
    position: image.position,
    caption: image.caption ?? undefined,
    altText: image.alt_text ?? undefined,
    createdAt: image.created_at,
  }));

  return {
    id: apiPost.id,
    userId: apiPost.user_id,
    sectionId: apiPost.section_id,
    content: apiPost.content,
    links: apiPost.links?.map((link): Link => ({
      id: link.id,
      url: link.url,
      metadata: normalizeLinkMetadata(link.metadata, link.url),
      highlights: normalizeHighlights(link.highlights),
    })),
    images,
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
    recipeStats: normalizeRecipeStats(apiPost.recipe_stats ?? apiPost.recipeStats),
    createdAt: apiPost.created_at,
    updatedAt: apiPost.updated_at,
  };
}
