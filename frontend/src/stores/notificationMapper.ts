import type { Notification } from './notificationStore';

export interface ApiNotificationUser {
  id: string;
  username: string;
  profile_picture_url?: string | null;
}

export interface ApiNotification {
  id: string;
  user_id?: string;
  type: string;
  related_post_id?: string | null;
  related_comment_id?: string | null;
  related_user_id?: string | null;
  related_user?: ApiNotificationUser | null;
  content_excerpt?: string | null;
  read_at?: string | null;
  created_at?: string;
}

function readString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function mapRelatedUser(raw: unknown): Notification['relatedUser'] | undefined {
  if (!raw || typeof raw !== 'object') {
    return undefined;
  }

  const record = raw as Record<string, unknown>;
  const id = readString(record.id);
  const username = readString(record.username);
  if (!id || !username) {
    return undefined;
  }
  const profilePictureUrl =
    readString(record.profile_picture_url ?? record.profilePictureUrl) ?? undefined;

  return {
    id,
    username,
    profilePictureUrl,
  };
}

export function mapApiNotification(apiNotification: ApiNotification): Notification {
  return {
    id: apiNotification.id,
    userId: apiNotification.user_id,
    type: apiNotification.type,
    relatedPostId: apiNotification.related_post_id ?? null,
    relatedCommentId: apiNotification.related_comment_id ?? null,
    relatedUserId: apiNotification.related_user_id ?? null,
    relatedUser: apiNotification.related_user
      ? {
          id: apiNotification.related_user.id,
          username: apiNotification.related_user.username,
          profilePictureUrl: apiNotification.related_user.profile_picture_url ?? undefined,
        }
      : undefined,
    contentExcerpt: apiNotification.content_excerpt ?? null,
    readAt: apiNotification.read_at ?? null,
    createdAt: apiNotification.created_at ?? new Date().toISOString(),
  };
}

export function mapNotificationPayload(payload: unknown): Notification | null {
  if (!payload || typeof payload !== 'object') {
    return null;
  }

  const outer = payload as Record<string, unknown>;
  const raw = outer.notification ?? payload;
  if (!raw || typeof raw !== 'object') {
    return null;
  }

  const record = raw as Record<string, unknown>;
  const id = readString(record.id);
  const type = readString(record.type);
  if (!id || !type) {
    return null;
  }

  const createdAt =
    readString(record.created_at ?? record.createdAt) ?? new Date().toISOString();

  const relatedUser = mapRelatedUser(record.related_user ?? record.relatedUser);

  return {
    id,
    userId: readString(record.user_id ?? record.userId),
    type,
    relatedPostId: readString(record.related_post_id ?? record.relatedPostId) ?? null,
    relatedCommentId: readString(record.related_comment_id ?? record.relatedCommentId) ?? null,
    relatedUserId: readString(record.related_user_id ?? record.relatedUserId) ?? null,
    relatedUser,
    contentExcerpt: readString(record.content_excerpt ?? record.contentExcerpt) ?? null,
    readAt: readString(record.read_at ?? record.readAt) ?? null,
    createdAt,
  };
}
