import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createRequire } from 'module';

const require = createRequire(import.meta.url);
const { writable, get } = require('svelte/store') as typeof import('svelte/store');

class MockWebSocket {
  static instances: MockWebSocket[] = [];
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;

  url: string;
  readyState = MockWebSocket.CONNECTING;
  send = vi.fn();
  close = vi.fn(() => {
    this.readyState = MockWebSocket.CLOSED;
    this.emit('close');
  });

  private listeners: Record<string, Array<(event: any) => void>> = {};

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
  }

  addEventListener(type: string, handler: (event: any) => void) {
    this.listeners[type] = this.listeners[type] || [];
    this.listeners[type].push(handler);
  }

  emit(type: string, event: any = {}) {
    (this.listeners[type] || []).forEach((handler) => handler(event));
  }

  open() {
    this.readyState = MockWebSocket.OPEN;
    this.emit('open');
  }
}

const storeRefs = {
  activeSection: writable<{ id: string } | null>(null),
  isAuthenticated: writable(false),
  postStore: {
    upsertPost: vi.fn(),
    incrementCommentCount: vi.fn(),
    updateReactionCount: vi.fn(),
  },
  commentState: writable<Record<string, { comments: Array<{ id: string; replies?: any[] }> }>>({}),
  commentStore: {} as {
    subscribe: ReturnType<typeof writable>['subscribe'];
    addComment: ReturnType<typeof vi.fn>;
    addReply: ReturnType<typeof vi.fn>;
    setState: (value: Record<string, { comments: Array<{ id: string; replies?: any[] }> }>) => void;
  },
  api: {
    getComment: vi.fn(),
  },
  mapApiComment: vi.fn(),
  mapApiPost: vi.fn(),
};

storeRefs.commentStore = {
  subscribe: storeRefs.commentState.subscribe,
  addComment: vi.fn(),
  addReply: vi.fn(),
  setState: (value: Record<string, { comments: Array<{ id: string; replies?: any[] }> }>) =>
    storeRefs.commentState.set(value),
};

vi.mock('../sectionStore', () => ({
  activeSection: storeRefs.activeSection,
}));

vi.mock('../authStore', () => ({
  isAuthenticated: storeRefs.isAuthenticated,
}));

vi.mock('../postStore', () => ({
  postStore: storeRefs.postStore,
}));

vi.mock('../commentStore', () => ({
  commentStore: storeRefs.commentStore,
}));

vi.mock('../../services/api', () => ({
  api: storeRefs.api,
}));

vi.mock('../commentMapper', () => ({
  mapApiComment: (comment: unknown) => storeRefs.mapApiComment(comment),
}));

vi.mock('../postMapper', () => ({
  mapApiPost: (post: unknown) => storeRefs.mapApiPost(post),
}));

beforeEach(() => {
  MockWebSocket.instances = [];
  storeRefs.activeSection.set(null);
  storeRefs.isAuthenticated.set(false);
  storeRefs.postStore.upsertPost.mockReset();
  storeRefs.postStore.incrementCommentCount.mockReset();
  storeRefs.postStore.updateReactionCount.mockReset();
  storeRefs.commentStore.addComment.mockReset();
  storeRefs.commentStore.addReply.mockReset();
  storeRefs.api.getComment.mockReset();
  storeRefs.mapApiComment.mockReset();
  storeRefs.mapApiPost.mockReset();
  storeRefs.commentStore.setState({});

  (globalThis as any).WebSocket = MockWebSocket;
});

afterEach(() => {
  vi.useRealTimers();
  vi.resetModules();
});

describe('websocketStore', () => {
  it('init is idempotent and cleanup resets status', async () => {
    storeRefs.isAuthenticated.set(true);
    const { websocketStore } = await import('../websocketStore');

    websocketStore.init();
    websocketStore.init();

    expect(MockWebSocket.instances).toHaveLength(1);

    websocketStore.cleanup();
    websocketStore.cleanup();
    const status = get(websocketStore.status);
    expect(status).toBe('disconnected');
  });

  it('connects on auth and disconnects on logout', async () => {
    const { websocketStore } = await import('../websocketStore');

    websocketStore.init();
    expect(MockWebSocket.instances).toHaveLength(0);

    storeRefs.isAuthenticated.set(true);
    expect(MockWebSocket.instances).toHaveLength(1);

    const socket = MockWebSocket.instances[0];
    socket.open();

    storeRefs.isAuthenticated.set(false);
    expect(socket.close).toHaveBeenCalled();
  });

  it('subscribes/unsubscribes on section changes', async () => {
    storeRefs.isAuthenticated.set(true);
    const { websocketStore } = await import('../websocketStore');

    websocketStore.init();
    const socket = MockWebSocket.instances[0];
    socket.open();

    storeRefs.activeSection.set({ id: 'section-1' });
    expect(socket.send).toHaveBeenCalledWith(
      JSON.stringify({ type: 'subscribe', data: { sectionIds: ['section-1'] } })
    );

    storeRefs.activeSection.set({ id: 'section-2' });
    expect(socket.send).toHaveBeenCalledWith(
      JSON.stringify({ type: 'unsubscribe', data: { sectionIds: ['section-1'] } })
    );
    expect(socket.send).toHaveBeenCalledWith(
      JSON.stringify({ type: 'subscribe', data: { sectionIds: ['section-2'] } })
    );
  });

  it('handles new_post event', async () => {
    storeRefs.isAuthenticated.set(true);
    storeRefs.mapApiPost.mockReturnValue({ id: 'post-1' });

    const { websocketStore } = await import('../websocketStore');

    websocketStore.init();
    const socket = MockWebSocket.instances[0];
    socket.open();

    socket.emit('message', {
      data: JSON.stringify({
        type: 'new_post',
        data: { post: { id: 'post-1' } },
        timestamp: 'now',
      }),
    });

    expect(storeRefs.postStore.upsertPost).toHaveBeenCalledWith({ id: 'post-1' });
  });

  it('handles new_comment and avoids double count when already present', async () => {
    storeRefs.isAuthenticated.set(true);
    storeRefs.mapApiComment.mockReturnValue({
      id: 'comment-1',
      postId: 'post-1',
      userId: 'user-1',
      content: 'Hello',
      createdAt: 'now',
    });
    storeRefs.api.getComment.mockResolvedValue({ comment: { id: 'comment-1' } });

    const { websocketStore } = await import('../websocketStore');
    websocketStore.init();

    const socket = MockWebSocket.instances[0];
    socket.open();

    storeRefs.commentStore.setState({ 'post-1': { comments: [] } } as any);

    socket.emit('message', {
      data: JSON.stringify({
        type: 'new_comment',
        data: { comment: { id: 'comment-1', post_id: 'post-1' } },
        timestamp: 'now',
      }),
    });

    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(storeRefs.postStore.incrementCommentCount).toHaveBeenCalledWith('post-1', 1);
    expect(storeRefs.api.getComment).toHaveBeenCalledWith('comment-1');
    expect(storeRefs.commentStore.addComment).toHaveBeenCalled();

    storeRefs.postStore.incrementCommentCount.mockClear();
    storeRefs.api.getComment.mockClear();

    storeRefs.commentStore.setState({ 'post-1': { comments: [{ id: 'comment-1' }] } } as any);

    socket.emit('message', {
      data: JSON.stringify({
        type: 'new_comment',
        data: { comment: { id: 'comment-1', post_id: 'post-1' } },
        timestamp: 'now',
      }),
    });

    expect(storeRefs.postStore.incrementCommentCount).not.toHaveBeenCalled();
    expect(storeRefs.api.getComment).not.toHaveBeenCalled();
  });

  it('handles reaction events', async () => {
    storeRefs.isAuthenticated.set(true);
    const { websocketStore } = await import('../websocketStore');
    websocketStore.init();

    const socket = MockWebSocket.instances[0];
    socket.open();

    socket.emit('message', {
      data: JSON.stringify({
        type: 'reaction_added',
        data: { post_id: 'post-1', emoji: '🔥', user_id: 'user-1' },
        timestamp: 'now',
      }),
    });

    socket.emit('message', {
      data: JSON.stringify({
        type: 'reaction_removed',
        data: { post_id: 'post-1', emoji: '🔥', user_id: 'user-1' },
        timestamp: 'now',
      }),
    });

    expect(storeRefs.postStore.updateReactionCount).toHaveBeenCalledWith('post-1', '🔥', 1);
    expect(storeRefs.postStore.updateReactionCount).toHaveBeenCalledWith('post-1', '🔥', -1);
  });

  it('schedules reconnect on close when authed', async () => {
    vi.useFakeTimers();
    storeRefs.isAuthenticated.set(true);

    const { websocketStore } = await import('../websocketStore');
    websocketStore.init();

    const socket = MockWebSocket.instances[0];
    socket.open();
    socket.emit('close');

    const initialCount = MockWebSocket.instances.length;
    expect(initialCount).toBeGreaterThan(0);

    vi.advanceTimersByTime(1000);
    expect(MockWebSocket.instances.length).toBeGreaterThan(initialCount);
  });
});
