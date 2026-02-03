import { writable, get } from 'svelte/store';
import type { Link } from './postStore';

export interface Comment {
  id: string;
  userId: string;
  postId: string;
  parentCommentId?: string;
  imageId?: string;
  content: string;
  timestampSeconds?: number;
  timestampDisplay?: string;
  links?: Link[];
  user?: {
    id: string;
    username: string;
    profilePictureUrl?: string;
  };
  replies?: Comment[];
  reactionCounts?: Record<string, number>;
  viewerReactions?: string[];
  createdAt: string;
  updatedAt?: string;
}

export interface CreateCommentRequest {
  postId: string;
  parentCommentId?: string;
  imageId?: string;
  content: string;
  timestampSeconds?: number;
  links?: { url: string }[];
  mentionUsernames?: string[];
}

export interface CommentThreadState {
  comments: Comment[];
  isLoading: boolean;
  error: string | null;
  cursor: string | null;
  hasMore: boolean;
  loaded: boolean;
  seenCommentIds: Set<string>;
}

export type CommentStoreState = Record<string, CommentThreadState>;

const defaultThreadState = (): CommentThreadState => ({
  comments: [],
  isLoading: false,
  error: null,
  cursor: null,
  hasMore: true,
  loaded: false,
  seenCommentIds: new Set(),
});

function createCommentStore() {
  const { subscribe, update } = writable<CommentStoreState>({});

  function updateReactionCounts(
    comments: Comment[],
    commentId: string,
    emoji: string,
    delta: number
  ): Comment[] {
    return comments.map((comment) => {
      if (comment.id === commentId) {
        const counts = { ...(comment.reactionCounts ?? {}) };
        const next = (counts[emoji] ?? 0) + delta;
        if (next <= 0) {
          delete counts[emoji];
        } else {
          counts[emoji] = next;
        }
        return {
          ...comment,
          reactionCounts: counts,
        };
      }
      if (comment.replies?.length) {
        return {
          ...comment,
          replies: updateReactionCounts(comment.replies, commentId, emoji, delta),
        };
      }
      return comment;
    });
  }

  function toggleReactions(
    comments: Comment[],
    commentId: string,
    emoji: string
  ): Comment[] {
    return comments.map((comment) => {
      if (comment.id === commentId) {
        const viewerReactions = new Set(comment.viewerReactions ?? []);
        const counts = { ...(comment.reactionCounts ?? {}) };

        if (viewerReactions.has(emoji)) {
          viewerReactions.delete(emoji);
          const next = (counts[emoji] ?? 0) - 1;
          if (next <= 0) delete counts[emoji];
          else counts[emoji] = next;
        } else {
          viewerReactions.add(emoji);
          counts[emoji] = (counts[emoji] ?? 0) + 1;
        }

        return {
          ...comment,
          reactionCounts: counts,
          viewerReactions: Array.from(viewerReactions),
        };
      }
      if (comment.replies?.length) {
        return {
          ...comment,
          replies: toggleReactions(comment.replies, commentId, emoji),
        };
      }
      return comment;
    });
  }

  function ensureThread(state: CommentStoreState, postId: string): CommentThreadState {
    return state[postId] ?? defaultThreadState();
  }

  function hasComment(comments: Comment[], commentId: string): boolean {
    for (const comment of comments) {
      if (comment.id === commentId) {
        return true;
      }
      if (comment.replies?.length && hasComment(comment.replies, commentId)) {
        return true;
      }
    }
    return false;
  }

  function updateThread(
    postId: string,
    updater: (thread: CommentThreadState) => CommentThreadState
  ) {
    update((state) => ({
      ...state,
      [postId]: updater(ensureThread(state, postId)),
    }));
  }

  function pruneSeenIds(comments: Comment[], seenIds: Set<string>): Set<string> {
    const next = new Set(seenIds);
    for (const id of next) {
      if (hasComment(comments, id)) {
        next.delete(id);
      }
    }
    return next;
  }

  function insertReply(
    comments: Comment[],
    parentCommentId: string,
    reply: Comment
  ): { comments: Comment[]; inserted: boolean } {
    let inserted = false;
    const next = comments.map((comment) => {
      if (comment.id === parentCommentId) {
        if (hasComment(comment.replies ?? [], reply.id)) {
          return comment;
        }
        inserted = true;
        return {
          ...comment,
          replies: [...(comment.replies ?? []), reply],
        };
      }
      if (comment.replies?.length) {
        const nested = insertReply(comment.replies, parentCommentId, reply);
        if (nested.inserted) {
          inserted = true;
          return {
            ...comment,
            replies: nested.comments,
          };
        }
      }
      return comment;
    });
    return { comments: next, inserted };
  }

  function updateCommentContent(
    comments: Comment[],
    commentId: string,
    content: string
  ): Comment[] {
    return comments.map((comment) => {
      if (comment.id === commentId) {
        return {
          ...comment,
          content,
        };
      }
      if (comment.replies?.length) {
        return {
          ...comment,
          replies: updateCommentContent(comment.replies, commentId, content),
        };
      }
      return comment;
    });
  }

  function updateCommentById(comments: Comment[], updated: Comment): Comment[] {
    return comments.map((comment) => {
      if (comment.id === updated.id) {
        const mergedReplies =
          updated.replies && updated.replies.length > 0
            ? updated.replies
            : comment.replies ?? [];
        return {
          ...comment,
          ...updated,
          replies: mergedReplies,
        };
      }
      if (comment.replies?.length) {
        return {
          ...comment,
          replies: updateCommentById(comment.replies, updated),
        };
      }
      return comment;
    });
  }

  function collectCommentIds(comment: Comment, ids: Set<string>) {
    ids.add(comment.id);
    if (comment.replies?.length) {
      for (const reply of comment.replies) {
        collectCommentIds(reply, ids);
      }
    }
  }

  function removeCommentById(
    comments: Comment[],
    commentId: string
  ): { comments: Comment[]; removed: boolean; removedIds: Set<string> } {
    let removed = false;
    const removedIds = new Set<string>();
    const next = comments.flatMap((comment) => {
      if (comment.id === commentId) {
        removed = true;
        collectCommentIds(comment, removedIds);
        return [];
      }
      if (comment.replies?.length) {
        const nested = removeCommentById(comment.replies, commentId);
        if (nested.removed) {
          removed = true;
          for (const id of nested.removedIds) {
            removedIds.add(id);
          }
          return [
            {
              ...comment,
              replies: nested.comments,
            },
          ];
        }
      }
      return [comment];
    });
    return { comments: next, removed, removedIds };
  }

  function updateProfilePictures(
    comments: Comment[],
    userId: string,
    profilePictureUrl?: string
  ): Comment[] {
    return comments.map((comment) => {
      const nextReplies = comment.replies?.length
        ? updateProfilePictures(comment.replies, userId, profilePictureUrl)
        : comment.replies;
      let nextUser = comment.user;
      if (comment.user?.id === userId) {
        nextUser = {
          ...comment.user,
          profilePictureUrl,
        };
      }
      if (nextReplies !== comment.replies || nextUser !== comment.user) {
        return {
          ...comment,
          user: nextUser,
          replies: nextReplies,
        };
      }
      return comment;
    });
  }

  return {
    subscribe,
    setThread: (postId: string, comments: Comment[], cursor: string | null, hasMore: boolean) =>
      updateThread(postId, (thread) => ({
        ...thread,
        comments,
        cursor,
        hasMore,
        isLoading: false,
        error: null,
        loaded: true,
        seenCommentIds: pruneSeenIds(comments, thread.seenCommentIds),
      })),
    appendThread: (postId: string, comments: Comment[], cursor: string | null, hasMore: boolean) =>
      updateThread(postId, (thread) => ({
        ...thread,
        comments: [
          ...thread.comments,
          ...comments.filter((comment) => !hasComment(thread.comments, comment.id)),
        ],
        cursor,
        hasMore,
        isLoading: false,
        loaded: true,
        seenCommentIds: pruneSeenIds(
          [...thread.comments, ...comments],
          thread.seenCommentIds
        ),
      })),
    addComment: (postId: string, comment: Comment) =>
      updateThread(postId, (thread) => {
        const seenCommentIds = new Set(thread.seenCommentIds);
        if (hasComment(thread.comments, comment.id)) {
          return { ...thread, loaded: true, seenCommentIds };
        }
        seenCommentIds.delete(comment.id);
        return {
          ...thread,
          comments: [comment, ...thread.comments],
          loaded: true,
          seenCommentIds,
        };
      }),
    markSeenComment: (postId: string, commentId: string) =>
      updateThread(postId, (thread) => {
        const seenCommentIds = new Set(thread.seenCommentIds);
        seenCommentIds.add(commentId);
        return {
          ...thread,
          seenCommentIds,
        };
      }),
    consumeSeenComment: (postId: string, commentId: string): boolean => {
      const state = get({ subscribe });
      const thread = state[postId];
      if (!thread?.seenCommentIds?.has(commentId)) {
        return false;
      }
      updateThread(postId, (current) => {
        const seenCommentIds = new Set(current.seenCommentIds);
        seenCommentIds.delete(commentId);
        return {
          ...current,
          seenCommentIds,
        };
      });
      return true;
    },
    addReply: (postId: string, parentCommentId: string, reply: Comment) =>
      updateThread(postId, (thread) => {
        const seenCommentIds = new Set(thread.seenCommentIds);
        const { comments, inserted } = insertReply(thread.comments, parentCommentId, reply);
        const shouldDeleteSeen =
          inserted && !hasComment(thread.comments, reply.id) && seenCommentIds.has(reply.id);
        if (shouldDeleteSeen) {
          seenCommentIds.delete(reply.id);
        }
        return {
          ...thread,
          comments,
          loaded: true,
          seenCommentIds,
        };
      }),
    setLoading: (postId: string, isLoading: boolean) =>
      updateThread(postId, (thread) => ({
        ...thread,
        isLoading,
      })),
    setError: (postId: string, error: string | null) =>
      updateThread(postId, (thread) => ({
        ...thread,
        error,
        isLoading: false,
      })),
    resetThread: (postId: string) =>
      update((state) => {
        const { [postId]: _, ...rest } = state;
        return rest;
      }),
    updateReactionCount: (postId: string, commentId: string, emoji: string, delta: number) =>
      updateThread(postId, (thread) => ({
        ...thread,
        comments: updateReactionCounts(thread.comments, commentId, emoji, delta),
      })),
    toggleReaction: (postId: string, commentId: string, emoji: string) =>
      updateThread(postId, (thread) => ({
        ...thread,
        comments: toggleReactions(thread.comments, commentId, emoji),
      })),
    updateContent: (postId: string, commentId: string, content: string) =>
      updateThread(postId, (thread) => ({
        ...thread,
        comments: updateCommentContent(thread.comments, commentId, content),
      })),
    updateComment: (postId: string, comment: Comment) =>
      updateThread(postId, (thread) => ({
        ...thread,
        comments: updateCommentById(thread.comments, comment),
      })),
    removeComment: (postId: string, commentId: string): number => {
      let removedCount = 0;
      updateThread(postId, (thread) => {
        const result = removeCommentById(thread.comments, commentId);
        removedCount = result.removed ? result.removedIds.size : 0;
        if (!result.removed) {
          return thread;
        }
        const seenCommentIds = new Set(thread.seenCommentIds);
        for (const id of result.removedIds) {
          seenCommentIds.delete(id);
        }
        return {
          ...thread,
          comments: result.comments,
          seenCommentIds,
        };
      });
      return removedCount;
    },
    updateUserProfilePicture: (userId: string, profilePictureUrl?: string) =>
      update((state) => {
        const nextState: CommentStoreState = {};
        for (const [postId, thread] of Object.entries(state)) {
          nextState[postId] = {
            ...thread,
            comments: updateProfilePictures(thread.comments, userId, profilePictureUrl),
          };
        }
        return nextState;
      }),
  };
}

export const commentStore = createCommentStore();
