import { writable } from 'svelte/store';
import type { Link } from './postStore';

export interface Comment {
  id: string;
  userId: string;
  postId: string;
  parentCommentId?: string;
  content: string;
  links?: Link[];
  user?: {
    id: string;
    username: string;
    profilePictureUrl?: string;
  };
  replies?: Comment[];
  reactionCounts?: Record<string, number>;
  createdAt: string;
  updatedAt?: string;
}

export interface CreateCommentRequest {
  postId: string;
  parentCommentId?: string;
  content: string;
  links?: { url: string }[];
}

export interface CommentThreadState {
  comments: Comment[];
  isLoading: boolean;
  error: string | null;
  cursor: string | null;
  hasMore: boolean;
  loaded: boolean;
}

export type CommentStoreState = Record<string, CommentThreadState>;

const defaultThreadState = (): CommentThreadState => ({
  comments: [],
  isLoading: false,
  error: null,
  cursor: null,
  hasMore: true,
  loaded: false,
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

  function ensureThread(state: CommentStoreState, postId: string): CommentThreadState {
    return state[postId] ?? defaultThreadState();
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
      })),
    appendThread: (postId: string, comments: Comment[], cursor: string | null, hasMore: boolean) =>
      updateThread(postId, (thread) => ({
        ...thread,
        comments: [...thread.comments, ...comments],
        cursor,
        hasMore,
        isLoading: false,
        loaded: true,
      })),
    addComment: (postId: string, comment: Comment) =>
      updateThread(postId, (thread) => ({
        ...thread,
        comments: [comment, ...thread.comments],
        loaded: true,
      })),
    addReply: (postId: string, parentCommentId: string, reply: Comment) =>
      updateThread(postId, (thread) => ({
        ...thread,
        comments: thread.comments.map((comment) =>
          comment.id === parentCommentId
            ? {
                ...comment,
                replies: [...(comment.replies ?? []), reply],
              }
            : comment
        ),
        loaded: true,
      })),
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
  };
}

export const commentStore = createCommentStore();
