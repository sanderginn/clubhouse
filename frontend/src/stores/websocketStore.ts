import { writable, get } from 'svelte/store';
import { activeSection } from './sectionStore';
import { isAuthenticated, currentUser } from './authStore';
import { postStore, type Post } from './postStore';
import { commentStore, type Comment } from './commentStore';
import { handleRealtimeNotification } from './notificationStore';
import {
  handleCookLogCreatedEvent,
  handleCookLogRemovedEvent,
  handleCookLogUpdatedEvent,
  handleRecipeCategoryCreatedEvent,
  handleRecipeCategoryDeletedEvent,
  handleRecipeCategoryUpdatedEvent,
  handleRecipeSavedEvent,
  handleRecipeUnsavedEvent,
} from './recipeStore';
import { api } from '../services/api';
import { mapApiComment } from './commentMapper';
import { mapApiPost, type ApiPost } from './postMapper';
import { logError, logInfo, logWarn } from '../lib/observability/logger';
import { recordWebsocketConnect } from '../lib/observability/performance';

type WebSocketStatus = 'disconnected' | 'connecting' | 'connected' | 'error';

interface WebSocketError {
  message: string;
  timestamp: Date;
  reconnectAttempt: number;
}

interface WsEvent<T = unknown> {
  type: string;
  data: T;
  timestamp: string;
}

interface WsPostEvent {
  post: ApiPost;
}

interface WsCommentEvent {
  comment: {
    id: string;
    post_id: string;
  };
}

interface WsReactionEvent {
  post_id?: string;
  comment_id?: string;
  user_id: string;
  emoji: string;
}

interface WsSubscriptionPayload {
  sectionIds: string[];
}

interface WsOutgoingMessage<T = unknown> {
  type: 'subscribe' | 'unsubscribe';
  data: T;
}

function hasComment(comments: Comment[] | undefined, commentId: string): boolean {
  if (!comments) {
    return false;
  }
  for (const comment of comments) {
    if (comment.id === commentId) {
      return true;
    }
    if (hasComment(comment.replies, commentId)) {
      return true;
    }
  }
  return false;
}

const status = writable<WebSocketStatus>('disconnected');
const lastError = writable<WebSocketError | null>(null);

let socket: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let reconnectAttempts = 0;
const maxReconnectAttempts = 10;
let initialized = false;
let lastAuthState = false;
let currentSectionId: string | null = null;
let authUnsub: (() => void) | null = null;
let sectionUnsub: (() => void) | null = null;
let intentionalClose = false;

function getWebSocketUrl(): string {
  const url = new URL('/api/v1/ws', window.location.href);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  return url.toString();
}

function sendMessage<T>(message: WsOutgoingMessage<T>) {
  if (!socket || socket.readyState !== WebSocket.OPEN) {
    return;
  }
  socket.send(JSON.stringify(message));
}

function subscribeToSection(sectionId: string) {
  sendMessage<WsSubscriptionPayload>({
    type: 'subscribe',
    data: { sectionIds: [sectionId] },
  });
}

function unsubscribeFromSection(sectionId: string) {
  sendMessage<WsSubscriptionPayload>({
    type: 'unsubscribe',
    data: { sectionIds: [sectionId] },
  });
}

function handleSectionChange(sectionId: string | null) {
  if (sectionId === currentSectionId) {
    return;
  }

  if (currentSectionId) {
    unsubscribeFromSection(currentSectionId);
  }

  currentSectionId = sectionId;

  if (currentSectionId) {
    subscribeToSection(currentSectionId);
  }
}

function scheduleReconnect() {
  if (reconnectTimer || !lastAuthState) {
    return;
  }

  if (reconnectAttempts >= maxReconnectAttempts) {
    const error: WebSocketError = {
      message: `Failed to reconnect after ${maxReconnectAttempts} attempts`,
      timestamp: new Date(),
      reconnectAttempt: reconnectAttempts,
    };
    lastError.set(error);
    status.set('error');
    logError('WebSocket max reconnection attempts reached', {
      reconnectAttempts,
      message: error.message,
    });
    return;
  }

  const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 10000);
  reconnectAttempts += 1;
  logInfo('WebSocket scheduling reconnection', { attempt: reconnectAttempts, delayMs: delay });
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    connect();
  }, delay);
}

function connect() {
  if (!lastAuthState || socket || get(status) === 'connecting') {
    return;
  }

  intentionalClose = false;
  const wsUrl = getWebSocketUrl();
  const connectStart = typeof performance !== 'undefined' ? performance.now() : null;
  let connectRecorded = false;
  logInfo('WebSocket connecting', {
    url: wsUrl,
    attempt: reconnectAttempts + 1,
  });
  status.set('connecting');

  const socketRef = new WebSocket(wsUrl);
  socket = socketRef;

  socketRef.addEventListener('open', () => {
    logInfo('WebSocket connected');
    if (connectStart !== null && !connectRecorded) {
      recordWebsocketConnect('success', performance.now() - connectStart);
      connectRecorded = true;
    }
    reconnectAttempts = 0;
    lastError.set(null);
    status.set('connected');
    if (currentSectionId) {
      subscribeToSection(currentSectionId);
    }
  });

  socketRef.addEventListener('message', (event) => {
    if (typeof event.data !== 'string') {
      return;
    }

    let parsed: WsEvent;
    try {
      parsed = JSON.parse(event.data) as WsEvent;
    } catch {
      return;
    }

    switch (parsed.type) {
      case 'new_post': {
        const payload = parsed.data as WsPostEvent;
        if (payload?.post) {
          const post: Post = mapApiPost(payload.post);
          postStore.upsertPost(post);
        }
        break;
      }
      case 'new_comment': {
        const payload = parsed.data as WsCommentEvent;
        if (payload?.comment?.post_id && payload?.comment?.id) {
          const postId = payload.comment.post_id;
          const commentId = payload.comment.id;
          const thread = get(commentStore)[postId];
          const exists = thread ? hasComment(thread.comments, commentId) : false;

          if (!exists) {
            commentStore.markSeenComment(postId, commentId);
            postStore.incrementCommentCount(postId, 1);
          }

          if (thread && !exists) {
            api
              .getComment(commentId)
              .then((response) => {
                const comment = mapApiComment(response.comment);
                if (comment.parentCommentId) {
                  commentStore.addReply(postId, comment.parentCommentId, comment);
                } else {
                  commentStore.addComment(postId, comment);
                }
              })
              .catch((error) => {
                logWarn('WebSocket comment fetch failed', { postId, commentId, error });
              });
          }
        }
        break;
      }
      case 'reaction_added': {
        const payload = parsed.data as WsReactionEvent;
        if (!payload?.emoji) {
          break;
        }
        const userId = get(currentUser)?.id;
        if (userId && payload.user_id === userId) {
          break;
        }
        if (payload.comment_id && payload.post_id) {
          commentStore.updateReactionCount(payload.post_id, payload.comment_id, payload.emoji, 1);
          break;
        }
        if (payload.post_id) {
          postStore.updateReactionCount(payload.post_id, payload.emoji, 1);
        }
        break;
      }
      case 'reaction_removed': {
        const payload = parsed.data as WsReactionEvent;
        if (!payload?.emoji) {
          break;
        }
        const userId = get(currentUser)?.id;
        if (userId && payload.user_id === userId) {
          break;
        }
        if (payload.comment_id && payload.post_id) {
          commentStore.updateReactionCount(payload.post_id, payload.comment_id, payload.emoji, -1);
          break;
        }
        if (payload.post_id) {
          postStore.updateReactionCount(payload.post_id, payload.emoji, -1);
        }
        break;
      }
      case 'notification': {
        handleRealtimeNotification(parsed.data);
        break;
      }
      case 'recipe_saved': {
        handleRecipeSavedEvent(parsed.data);
        break;
      }
      case 'recipe_unsaved': {
        handleRecipeUnsavedEvent(parsed.data);
        break;
      }
      case 'cook_log_created': {
        handleCookLogCreatedEvent(parsed.data);
        break;
      }
      case 'cook_log_updated': {
        handleCookLogUpdatedEvent(parsed.data);
        break;
      }
      case 'cook_log_removed': {
        handleCookLogRemovedEvent(parsed.data);
        break;
      }
      case 'recipe_category_created': {
        handleRecipeCategoryCreatedEvent(parsed.data);
        break;
      }
      case 'recipe_category_updated': {
        handleRecipeCategoryUpdatedEvent(parsed.data);
        break;
      }
      case 'recipe_category_deleted': {
        handleRecipeCategoryDeletedEvent(parsed.data);
        break;
      }
      default:
        break;
    }
  });

  socketRef.addEventListener('close', (event) => {
    if (socketRef !== socket) {
      return;
    }
    logInfo('WebSocket closed', {
      code: event.code,
      reason: event.reason || 'none',
    });
    if (connectStart !== null && !connectRecorded) {
      recordWebsocketConnect('closed', performance.now() - connectStart);
      connectRecorded = true;
    }
    socket = null;
    status.set('disconnected');

    const closedBecauseReplaced = event.code === 4000 || event.reason === 'replaced';
    const closedIntentionally = intentionalClose;
    intentionalClose = false;

    if (closedIntentionally || closedBecauseReplaced) {
      lastError.set(null);
      if (closedBecauseReplaced) {
        logInfo('WebSocket replaced by newer session');
      } else {
        logInfo('WebSocket closed intentionally');
      }
      return;
    }

    const error: WebSocketError = {
      message: event.reason || `Connection closed with code ${event.code}`,
      timestamp: new Date(),
      reconnectAttempt: reconnectAttempts,
    };
    lastError.set(error);

    scheduleReconnect();
  });

  socketRef.addEventListener('error', (event) => {
    logError('WebSocket connection error', { event });
    if (connectStart !== null && !connectRecorded) {
      recordWebsocketConnect('error', performance.now() - connectStart);
      connectRecorded = true;
    }
    const error: WebSocketError = {
      message: 'WebSocket connection error - check authentication and network connectivity',
      timestamp: new Date(),
      reconnectAttempt: reconnectAttempts,
    };
    lastError.set(error);
    socketRef.close();
  });
}

function disconnect() {
  logInfo('WebSocket disconnecting');
  intentionalClose = true;
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
  reconnectAttempts = 0;
  lastError.set(null);
  if (socket) {
    socket.close(1000, 'client_disconnect');
    socket = null;
  }
  status.set('disconnected');
}

function init() {
  if (initialized) {
    return;
  }
  initialized = true;

  lastAuthState = get(isAuthenticated);
  authUnsub = isAuthenticated.subscribe((authed) => {
    lastAuthState = authed;
    if (authed) {
      connect();
    } else {
      disconnect();
    }
  });

  sectionUnsub = activeSection.subscribe((section) => {
    handleSectionChange(section?.id ?? null);
  });

  if (lastAuthState) {
    connect();
  }
}

function cleanup() {
  disconnect();
  authUnsub?.();
  sectionUnsub?.();
  authUnsub = null;
  sectionUnsub = null;
  initialized = false;
}

function retry() {
  logInfo('WebSocket manual retry requested');
  reconnectAttempts = 0;
  lastError.set(null);
  if (lastAuthState && !socket) {
    connect();
  }
}

export const websocketStore = {
  init,
  cleanup,
  retry,
  status,
  lastError,
};

export const websocketStatus = {
  subscribe: status.subscribe,
};

export const websocketError = {
  subscribe: lastError.subscribe,
};
