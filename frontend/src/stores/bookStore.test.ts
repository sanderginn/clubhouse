import { beforeEach, describe, expect, it, vi } from 'vitest';
import { get } from 'svelte/store';

const apiGetBookshelfCategories = vi.hoisted(() => vi.fn());
const apiAddToBookshelf = vi.hoisted(() => vi.fn());
const apiRemoveFromBookshelf = vi.hoisted(() => vi.fn());
const apiCreateBookshelfCategory = vi.hoisted(() => vi.fn());
const apiUpdateBookshelfCategory = vi.hoisted(() => vi.fn());
const apiDeleteBookshelfCategory = vi.hoisted(() => vi.fn());
const apiLogRead = vi.hoisted(() => vi.fn());
const apiRemoveReadLog = vi.hoisted(() => vi.fn());
const apiUpdateReadRating = vi.hoisted(() => vi.fn());
const apiGetMyBookshelf = vi.hoisted(() => vi.fn());
const apiGetAllBookshelfItems = vi.hoisted(() => vi.fn());
const apiGetReadHistory = vi.hoisted(() => vi.fn());
const apiGetPostReadLogs = vi.hoisted(() => vi.fn());

vi.mock('../services/api', () => ({
  api: {
    getBookshelfCategories: apiGetBookshelfCategories,
    addToBookshelf: apiAddToBookshelf,
    removeFromBookshelf: apiRemoveFromBookshelf,
    createBookshelfCategory: apiCreateBookshelfCategory,
    updateBookshelfCategory: apiUpdateBookshelfCategory,
    deleteBookshelfCategory: apiDeleteBookshelfCategory,
    logRead: apiLogRead,
    removeReadLog: apiRemoveReadLog,
    updateReadRating: apiUpdateReadRating,
    getMyBookshelf: apiGetMyBookshelf,
    getAllBookshelfItems: apiGetAllBookshelfItems,
    getReadHistory: apiGetReadHistory,
    getPostReadLogs: apiGetPostReadLogs,
  },
}));

const {
  bookStore,
  bookshelfCategories,
  myBookshelf,
  allBookshelf,
  readHistory,
  bookStoreMeta,
} = await import('./bookStore');
const { authStore } = await import('./authStore');
const { postStore } = await import('./postStore');

const createDeferred = <T>() => {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
};

beforeEach(() => {
  bookStore.reset();
  postStore.reset();
  authStore.setUser({
    id: 'user-1',
    username: 'reader',
    email: 'reader@example.com',
    isAdmin: false,
    totpEnabled: false,
  });

  apiGetBookshelfCategories.mockReset();
  apiAddToBookshelf.mockReset();
  apiRemoveFromBookshelf.mockReset();
  apiCreateBookshelfCategory.mockReset();
  apiUpdateBookshelfCategory.mockReset();
  apiDeleteBookshelfCategory.mockReset();
  apiLogRead.mockReset();
  apiRemoveReadLog.mockReset();
  apiUpdateReadRating.mockReset();
  apiGetMyBookshelf.mockReset();
  apiGetAllBookshelfItems.mockReset();
  apiGetReadHistory.mockReset();
  apiGetPostReadLogs.mockReset();

  apiGetBookshelfCategories.mockResolvedValue({ categories: [] });
  apiAddToBookshelf.mockResolvedValue(undefined);
  apiRemoveFromBookshelf.mockResolvedValue(undefined);
  apiCreateBookshelfCategory.mockResolvedValue({
    category: { id: 'cat-1', name: 'Favorites', position: 0 },
  });
  apiUpdateBookshelfCategory.mockResolvedValue({
    category: { id: 'cat-1', name: 'Top Picks', position: 1 },
  });
  apiDeleteBookshelfCategory.mockResolvedValue(undefined);
  apiLogRead.mockResolvedValue({
    readLog: {
      id: 'read-1',
      userId: 'user-1',
      postId: 'post-1',
      rating: 4,
      createdAt: '2026-01-01T00:00:00Z',
    },
  });
  apiRemoveReadLog.mockResolvedValue(undefined);
  apiUpdateReadRating.mockResolvedValue({
    readLog: {
      id: 'read-1',
      userId: 'user-1',
      postId: 'post-1',
      rating: 5,
      createdAt: '2026-01-01T00:00:00Z',
    },
  });
  apiGetMyBookshelf.mockResolvedValue({ bookshelfItems: [], nextCursor: undefined });
  apiGetAllBookshelfItems.mockResolvedValue({ bookshelfItems: [], nextCursor: undefined });
  apiGetReadHistory.mockResolvedValue({ readLogs: [], nextCursor: undefined });
  apiGetPostReadLogs.mockResolvedValue({
    readCount: 1,
    averageRating: 4,
    viewerRead: true,
    viewerRating: 4,
    readers: [],
  });
});

describe('bookStore', () => {
  it('initializes and resets all writable state', async () => {
    bookshelfCategories.set([{ id: 'cat-1', name: 'Favorites', position: 0 }]);
    myBookshelf.set(
      new Map([
        [
          'Favorites',
          [{ id: 'item-1', userId: 'user-1', postId: 'post-1', categoryId: 'cat-1', createdAt: 'now' }],
        ],
      ])
    );
    allBookshelf.set(new Map([['Favorites', []]]));
    readHistory.set([{ id: 'read-1', userId: 'user-1', postId: 'post-1', createdAt: 'now' }]);

    bookStore.reset();
    await bookStore.loadBookshelfCategories();

    expect(get(bookshelfCategories)).toEqual([]);
    expect(get(myBookshelf).size).toBe(0);
    expect(get(allBookshelf).size).toBe(0);
    expect(get(readHistory)).toEqual([]);
    expect(get(bookStoreMeta).error).toBeNull();
  });

  it('optimistically adds and removes bookshelf entries', async () => {
    postStore.setPosts(
      [
        {
          id: 'post-1',
          userId: 'user-1',
          sectionId: 'section-books',
          content: 'Book 1',
          createdAt: '2026-01-01T00:00:00Z',
          bookStats: {
            bookshelfCount: 0,
            readCount: 0,
            averageRating: null,
            viewerOnBookshelf: false,
            viewerCategories: [],
            viewerRead: false,
            viewerRating: null,
          },
        },
      ],
      null,
      false
    );

    const addDeferred = createDeferred<void>();
    apiAddToBookshelf.mockReturnValueOnce(addDeferred.promise);
    const addPromise = bookStore.addToBookshelf('post-1', ['Favorites']);

    let grouped = get(myBookshelf);
    expect(grouped.get('Favorites')).toHaveLength(1);
    let post = get(postStore).posts[0];
    expect(post.bookStats?.viewerOnBookshelf).toBe(true);

    addDeferred.resolve(undefined);
    await addPromise;

    const removeDeferred = createDeferred<void>();
    apiRemoveFromBookshelf.mockReturnValueOnce(removeDeferred.promise);
    const removePromise = bookStore.removeFromBookshelf('post-1');

    grouped = get(myBookshelf);
    expect(grouped.get('Favorites')).toBeUndefined();
    post = get(postStore).posts[0];
    expect(post.bookStats?.viewerOnBookshelf).toBe(false);

    removeDeferred.resolve(undefined);
    await removePromise;
  });

  it('optimistically logs reads and removes them', async () => {
    postStore.setPosts(
      [
        {
          id: 'post-1',
          userId: 'user-1',
          sectionId: 'section-books',
          content: 'Book 1',
          createdAt: '2026-01-01T00:00:00Z',
          bookStats: {
            bookshelfCount: 0,
            readCount: 0,
            averageRating: null,
            viewerOnBookshelf: false,
            viewerCategories: [],
            viewerRead: false,
            viewerRating: null,
          },
        },
      ],
      null,
      false
    );

    const logDeferred = createDeferred<{ readLog: { id: string; userId: string; postId: string; rating: number; createdAt: string } }>();
    apiLogRead.mockReturnValueOnce(logDeferred.promise);
    const logPromise = bookStore.logRead('post-1', 5);

    let logs = get(readHistory);
    expect(logs).toHaveLength(1);
    expect(logs[0].postId).toBe('post-1');
    let post = get(postStore).posts[0];
    expect(post.bookStats?.viewerRead).toBe(true);
    expect(post.bookStats?.viewerRating).toBe(5);

    logDeferred.resolve({
      readLog: {
        id: 'read-1',
        userId: 'user-1',
        postId: 'post-1',
        rating: 5,
        createdAt: '2026-01-01T00:00:00Z',
      },
    });
    await logPromise;

    const removeDeferred = createDeferred<void>();
    apiRemoveReadLog.mockReturnValueOnce(removeDeferred.promise);
    const removePromise = bookStore.removeRead('post-1');

    logs = get(readHistory);
    expect(logs).toHaveLength(0);
    post = get(postStore).posts[0];
    expect(post.bookStats?.viewerRead).toBe(false);
    expect(post.bookStats?.viewerRating).toBeNull();

    removeDeferred.resolve(undefined);
    await removePromise;
  });

  it('category CRUD updates categories and grouped bookshelf keys', async () => {
    bookshelfCategories.set([{ id: 'cat-1', name: 'Favorites', position: 0 }]);
    myBookshelf.set(
      new Map([
        [
          'Favorites',
          [
            {
              id: 'item-1',
              userId: 'user-1',
              postId: 'post-1',
              categoryId: 'cat-1',
              createdAt: '2026-01-01T00:00:00Z',
            },
          ],
        ],
      ])
    );

    await bookStore.createCategory('Favorites');
    expect(get(bookshelfCategories)).toHaveLength(1);

    await bookStore.updateCategory('cat-1', 'Top Picks', 1);
    let grouped = get(myBookshelf);
    expect(grouped.get('Favorites')).toBeUndefined();
    expect(grouped.get('Top Picks')).toHaveLength(1);

    await bookStore.deleteCategory('cat-1');
    grouped = get(myBookshelf);
    expect(get(bookshelfCategories)).toHaveLength(0);
    expect(grouped.get('Top Picks')).toBeUndefined();
    expect(grouped.get('Uncategorized')).toHaveLength(1);
  });

  it('rolls back optimistic state on API error', async () => {
    readHistory.set([
      {
        id: 'read-1',
        userId: 'user-1',
        postId: 'post-1',
        rating: 4,
        createdAt: '2026-01-01T00:00:00Z',
      },
    ]);
    postStore.setPosts(
      [
        {
          id: 'post-1',
          userId: 'user-1',
          sectionId: 'section-books',
          content: 'Book 1',
          createdAt: '2026-01-01T00:00:00Z',
          bookStats: {
            bookshelfCount: 0,
            readCount: 1,
            averageRating: 4,
            viewerOnBookshelf: false,
            viewerCategories: [],
            viewerRead: true,
            viewerRating: 4,
          },
        },
      ],
      null,
      false
    );

    apiRemoveReadLog.mockRejectedValueOnce(new Error('remove failed'));
    await bookStore.removeRead('post-1');

    const logs = get(readHistory);
    const post = get(postStore).posts[0];
    expect(logs).toHaveLength(1);
    expect(logs[0].postId).toBe('post-1');
    expect(post.bookStats?.viewerRead).toBe(true);
    expect(post.bookStats?.readCount).toBe(1);
    expect(get(bookStoreMeta).error).toBe('remove failed');
  });

  it('loads paginated bookshelf and read history data', async () => {
    bookshelfCategories.set([{ id: 'cat-1', name: 'Favorites', position: 0 }]);
    apiGetMyBookshelf
      .mockResolvedValueOnce({
        bookshelfItems: [
          {
            id: 'item-2',
            userId: 'user-1',
            postId: 'post-2',
            categoryId: 'cat-1',
            createdAt: '2026-01-02T00:00:00Z',
          },
        ],
        nextCursor: 'cursor-1',
      })
      .mockResolvedValueOnce({
        bookshelfItems: [
          {
            id: 'item-1',
            userId: 'user-1',
            postId: 'post-1',
            categoryId: 'cat-1',
            createdAt: '2026-01-01T00:00:00Z',
          },
        ],
        nextCursor: 'cursor-2',
      });
    apiGetAllBookshelfItems
      .mockResolvedValueOnce({
        bookshelfItems: [
          {
            id: 'all-2',
            userId: 'user-2',
            postId: 'post-4',
            categoryId: 'cat-1',
            createdAt: '2026-01-04T00:00:00Z',
          },
        ],
        nextCursor: 'all-cursor-1',
      })
      .mockResolvedValueOnce({
        bookshelfItems: [
          {
            id: 'all-1',
            userId: 'user-2',
            postId: 'post-3',
            categoryId: 'cat-1',
            createdAt: '2026-01-03T00:00:00Z',
          },
        ],
        nextCursor: 'all-cursor-2',
      });
    apiGetReadHistory
      .mockResolvedValueOnce({
        readLogs: [
          {
            id: 'read-2',
            userId: 'user-1',
            postId: 'post-2',
            rating: 5,
            createdAt: '2026-01-02T00:00:00Z',
          },
        ],
        nextCursor: 'read-cursor-1',
      })
      .mockResolvedValueOnce({
        readLogs: [
          {
            id: 'read-1',
            userId: 'user-1',
            postId: 'post-1',
            rating: 4,
            createdAt: '2026-01-01T00:00:00Z',
          },
        ],
        nextCursor: 'read-cursor-2',
      });

    const firstMyCursor = await bookStore.loadMyBookshelf(undefined);
    const secondMyCursor = await bookStore.loadMyBookshelf(undefined, firstMyCursor);
    const firstAllCursor = await bookStore.loadAllBookshelf('Favorites');
    const secondAllCursor = await bookStore.loadAllBookshelf('Favorites', firstAllCursor);
    const firstReadCursor = await bookStore.loadReadHistory();
    const secondReadCursor = await bookStore.loadReadHistory(firstReadCursor);

    expect(apiGetMyBookshelf).toHaveBeenNthCalledWith(1, undefined, undefined, 20);
    expect(apiGetMyBookshelf).toHaveBeenNthCalledWith(2, undefined, 'cursor-1', 20);
    expect(apiGetAllBookshelfItems).toHaveBeenNthCalledWith(1, 'Favorites', undefined, 20);
    expect(apiGetAllBookshelfItems).toHaveBeenNthCalledWith(2, 'Favorites', 'all-cursor-1', 20);
    expect(apiGetReadHistory).toHaveBeenNthCalledWith(1, undefined, 20);
    expect(apiGetReadHistory).toHaveBeenNthCalledWith(2, 'read-cursor-1', 20);

    const myGrouped = get(myBookshelf);
    const allGrouped = get(allBookshelf);
    const myFavorites = myGrouped.get('Favorites') ?? [];
    const allFavorites = allGrouped.get('Favorites') ?? [];
    const history = get(readHistory);
    expect(myGrouped.get('Favorites')).toHaveLength(2);
    expect(allGrouped.get('Favorites')).toHaveLength(2);
    expect(history).toHaveLength(2);
    expect(myFavorites.map((item) => item.postId)).toEqual(['post-2', 'post-1']);
    expect(allFavorites.map((item) => item.postId)).toEqual(['post-4', 'post-3']);
    expect(history.map((log) => log.postId)).toEqual(['post-2', 'post-1']);

    expect(secondMyCursor).toBe('cursor-2');
    expect(secondAllCursor).toBe('all-cursor-2');
    expect(secondReadCursor).toBe('read-cursor-2');
    expect(get(bookStoreMeta).cursors.readHistory).toBe('read-cursor-2');
  });
});
