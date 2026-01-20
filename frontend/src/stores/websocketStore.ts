import { writable, get } from 'svelte/store';
import { activeSection } from './sectionStore';
import { isAuthenticated } from './authStore';
import { postStore, type Post } from './postStore';
import { mapApiPost, type ApiPost } from './postMapper';

type WebSocketStatus = 'disconnected' | 'connecting' | 'connected';

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

const status = writable<WebSocketStatus>('disconnected');

let socket: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let reconnectAttempts = 0;
let initialized = false;
let lastAuthState = false;
let currentSectionId: string | null = null;
let authUnsub: (() => void) | null = null;
let sectionUnsub: (() => void) | null = null;

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
  const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 10000);
  reconnectAttempts += 1;
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    connect();
  }, delay);
}

function connect() {
  if (!lastAuthState || socket || get(status) === 'connecting') {
    return;
  }

  status.set('connecting');

  socket = new WebSocket(getWebSocketUrl());

  socket.addEventListener('open', () => {
    reconnectAttempts = 0;
    status.set('connected');
    if (currentSectionId) {
      subscribeToSection(currentSectionId);
    }
  });

  socket.addEventListener('message', (event) => {
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
        if (payload?.comment?.post_id) {
          postStore.incrementCommentCount(payload.comment.post_id, 1);
        }
        break;
      }
      case 'reaction_added': {
        const payload = parsed.data as WsReactionEvent;
        if (payload?.post_id && payload.emoji) {
          postStore.updateReactionCount(payload.post_id, payload.emoji, 1);
        }
        break;
      }
      case 'reaction_removed': {
        const payload = parsed.data as WsReactionEvent;
        if (payload?.post_id && payload.emoji) {
          postStore.updateReactionCount(payload.post_id, payload.emoji, -1);
        }
        break;
      }
      default:
        break;
    }
  });

  socket.addEventListener('close', () => {
    socket = null;
    status.set('disconnected');
    scheduleReconnect();
  });

  socket.addEventListener('error', () => {
    socket?.close();
  });
}

function disconnect() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
  reconnectAttempts = 0;
  if (socket) {
    socket.close();
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

export const websocketStore = {
  init,
  cleanup,
  status,
};

export const websocketStatus = {
  subscribe: status.subscribe,
};
