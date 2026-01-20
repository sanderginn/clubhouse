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
}

export type CommentStoreState = Record<string, CommentThreadState>;

const defaultThreadState = (): CommentThreadState => ({
  comments: [],
  isLoading: false,
  error: null,
  cursor: null,
  hasMore: true,
});

function createCommentStore() {
  const { subscribe, update } = writable<CommentStoreState>({});

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
      })),
    appendThread: (postId: string, comments: Comment[], cursor: string | null, hasMore: boolean) =>
      updateThread(postId, (thread) => ({
        ...thread,
        comments: [...thread.comments, ...comments],
        cursor,
        hasMore,
        isLoading: false,
      })),
    addComment: (postId: string, comment: Comment) =>
      updateThread(postId, (thread) => ({
        ...thread,
        comments: [comment, ...thread.comments],
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
  };
}

export const commentStore = createCommentStore();
