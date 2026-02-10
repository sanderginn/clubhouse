import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { api, type BookshelfItem } from '../../services/api';
import { authStore } from '../../stores/authStore';
import {
  allBookshelf,
  bookStore,
  bookStoreMeta,
  bookshelfCategories,
  myBookshelf,
} from '../../stores/bookStore';
import type { Post } from '../../stores/postStore';
import { postStore } from '../../stores/postStore';
import Bookshelf from './Bookshelf.svelte';

function createBookshelfItem(
  id: string,
  userId: string,
  postId: string,
  categoryId?: string,
  createdAt = '2026-01-01T00:00:00Z'
): BookshelfItem {
  return {
    id,
    userId,
    postId,
    categoryId,
    createdAt,
  };
}

function createBookPost(id: string, title: string, userId = 'author-1'): Post {
  return {
    id,
    userId,
    sectionId: 'section-books',
    content: title,
    createdAt: '2026-01-01T00:00:00Z',
    user: {
      id: userId,
      username: `user-${userId}`,
    },
    links: [
      {
        url: `https://example.com/${id}`,
        metadata: {
          url: `https://example.com/${id}`,
          title,
          type: 'book',
          author: 'Octavia Butler',
          description: `${title} description`,
          book: {
            title,
            authors: ['Octavia Butler'],
            cover_url: `https://example.com/${id}.jpg`,
          },
        } as never,
      },
    ],
  };
}

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

  bookshelfCategories.set([
    { id: 'cat-1', name: 'Favorites', position: 1 },
    { id: 'cat-2', name: 'Classics', position: 2 },
  ]);

  myBookshelf.set(
    new Map([
      [
        'Favorites',
        [createBookshelfItem('my-1', 'user-1', 'post-1', 'cat-1', '2026-01-02T00:00:00Z')],
      ],
      ['Classics', [createBookshelfItem('my-2', 'user-1', 'post-2', 'cat-2', '2026-01-03T00:00:00Z')]],
      ['Uncategorized', [createBookshelfItem('my-3', 'user-1', 'post-4', undefined)]],
    ])
  );

  allBookshelf.set(
    new Map([
      ['Favorites', [createBookshelfItem('all-1', 'user-2', 'post-3', 'cat-1')]],
    ])
  );

  postStore.setPosts(
    [
      createBookPost('post-1', 'Parable of the Sower'),
      createBookPost('post-2', 'Kindred'),
      createBookPost('post-3', 'The Left Hand of Darkness'),
      createBookPost('post-4', 'The Dispossessed'),
    ],
    null,
    false
  );

  bookStoreMeta.set({
    loading: {
      categories: false,
      myBookshelf: false,
      allBookshelf: false,
      readHistory: false,
    },
    cursors: {
      myBookshelf: null,
      allBookshelf: null,
      readHistory: null,
    },
    error: null,
  });

  vi.spyOn(bookStore, 'loadBookshelfCategories').mockResolvedValue();
  vi.spyOn(bookStore, 'loadMyBookshelf').mockResolvedValue(undefined);
  vi.spyOn(bookStore, 'loadAllBookshelf').mockResolvedValue(undefined);
  vi.spyOn(bookStore, 'createCategory').mockResolvedValue();
  vi.spyOn(bookStore, 'updateCategory').mockResolvedValue();
  vi.spyOn(bookStore, 'deleteCategory').mockResolvedValue();
  vi.spyOn(api, 'reorderBookshelfCategories').mockResolvedValue();
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  bookStore.reset();
  postStore.reset();
});

describe('Bookshelf', () => {
  it('switches between My Books and All Books tabs', async () => {
    render(Bookshelf);

    expect(screen.getByTestId('bookshelf-tab-my')).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByTestId('bookshelf-my-item-post-1')).toBeInTheDocument();

    await fireEvent.click(screen.getByTestId('bookshelf-tab-all'));

    expect(screen.getByTestId('bookshelf-tab-all')).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByTestId('bookshelf-all-item-post-3')).toBeInTheDocument();
  });

  it('renders category sidebar with All, custom categories, and Uncategorized', () => {
    render(Bookshelf);

    expect(screen.getByTestId('bookshelf-category-panel')).toBeInTheDocument();
    expect(screen.getByTestId('bookshelf-category-__all__')).toBeInTheDocument();
    expect(screen.getByTestId('bookshelf-category-Favorites')).toBeInTheDocument();
    expect(screen.getByTestId('bookshelf-category-Classics')).toBeInTheDocument();
    expect(screen.getByTestId('bookshelf-category-Uncategorized')).toBeInTheDocument();
  });

  it('filters My Books by selected category', async () => {
    render(Bookshelf);

    await fireEvent.click(screen.getByTestId('bookshelf-category-Classics'));

    expect(screen.getByTestId('bookshelf-my-item-post-2')).toBeInTheDocument();
    expect(screen.queryByTestId('bookshelf-my-item-post-1')).not.toBeInTheDocument();
  });

  it('loads more items in All Books pagination', async () => {
    allBookshelf.set(new Map());
    postStore.setPosts(
      [createBookPost('post-10', 'Dune'), createBookPost('post-11', 'Hyperion')],
      null,
      false
    );

    const loadAllSpy = vi.spyOn(bookStore, 'loadAllBookshelf').mockImplementation(async (_category, cursor) => {
      if (!cursor) {
        allBookshelf.set(
          new Map([
            ['Favorites', [createBookshelfItem('all-10', 'user-2', 'post-10', 'cat-1')]],
          ])
        );
        bookStoreMeta.update((state) => ({
          ...state,
          cursors: {
            ...state.cursors,
            allBookshelf: 'cursor-2',
          },
        }));
        return 'cursor-2';
      }

      allBookshelf.update((current) => {
        const next = new Map(current);
        const existing = next.get('Favorites') ?? [];
        next.set('Favorites', [...existing, createBookshelfItem('all-11', 'user-3', 'post-11', 'cat-1')]);
        return next;
      });
      bookStoreMeta.update((state) => ({
        ...state,
        cursors: {
          ...state.cursors,
          allBookshelf: null,
        },
      }));
      return undefined;
    });

    render(Bookshelf);

    await fireEvent.click(screen.getByTestId('bookshelf-tab-all'));
    await screen.findByTestId('bookshelf-all-item-post-10');

    await fireEvent.click(screen.getByTestId('bookshelf-all-load-more'));

    await waitFor(() => {
      expect(screen.getByTestId('bookshelf-all-item-post-11')).toBeInTheDocument();
    });

    expect(loadAllSpy).toHaveBeenNthCalledWith(1, undefined, undefined);
    expect(loadAllSpy).toHaveBeenNthCalledWith(2, undefined, 'cursor-2');
  });

  it('shows both empty states for no books and empty selected category', async () => {
    myBookshelf.set(new Map());

    const { unmount } = render(Bookshelf);

    expect(screen.getByText('No books saved yet')).toBeInTheDocument();

    unmount();

    bookshelfCategories.set([
      { id: 'cat-1', name: 'Favorites', position: 1 },
      { id: 'cat-2', name: 'Classics', position: 2 },
    ]);
    myBookshelf.set(
      new Map([
        ['Favorites', [createBookshelfItem('my-1', 'user-1', 'post-1', 'cat-1')]],
      ])
    );

    render(Bookshelf);

    await fireEvent.click(screen.getByTestId('bookshelf-category-Classics'));

    expect(screen.getByText('No books in this category')).toBeInTheDocument();
  });

  it('supports category create, edit, and delete actions', async () => {
    const createSpy = vi.spyOn(bookStore, 'createCategory').mockResolvedValue();
    const updateSpy = vi.spyOn(bookStore, 'updateCategory').mockResolvedValue();
    const deleteSpy = vi.spyOn(bookStore, 'deleteCategory').mockResolvedValue();

    render(Bookshelf);

    await fireEvent.click(screen.getByTestId('bookshelf-category-create'));
    await fireEvent.input(screen.getByTestId('bookshelf-category-create-input'), {
      target: { value: 'To Re-read' },
    });
    await fireEvent.click(screen.getByTestId('bookshelf-category-create-save'));

    await waitFor(() => {
      expect(createSpy).toHaveBeenCalledWith('To Re-read');
    });
    await screen.findByTestId('bookshelf-category-create');

    await fireEvent.click(screen.getByTestId('bookshelf-category-edit-cat-1'));
    await fireEvent.input(await screen.findByTestId('bookshelf-category-edit-input'), {
      target: { value: 'Best Of' },
    });
    await fireEvent.click(screen.getByTestId('bookshelf-category-edit-save'));

    await waitFor(() => {
      expect(updateSpy).toHaveBeenCalledWith('cat-1', 'Best Of', 1);
    });
    await waitFor(() => {
      expect(screen.queryByTestId('bookshelf-category-edit-input')).not.toBeInTheDocument();
    });

    await fireEvent.click(screen.getByTestId('bookshelf-category-delete-cat-1'));
    await fireEvent.click(screen.getByTestId('bookshelf-category-delete-confirm-button'));

    await waitFor(() => {
      expect(deleteSpy).toHaveBeenCalledWith('cat-1');
    });
  });

  it('reorders custom categories with drag and drop', async () => {
    const reorderSpy = vi.spyOn(api, 'reorderBookshelfCategories').mockResolvedValue();

    render(Bookshelf);

    const classicsRow = screen.getByTestId('bookshelf-custom-category-cat-2');
    const favoritesRow = screen.getByTestId('bookshelf-custom-category-cat-1');

    await fireEvent.dragStart(classicsRow);
    await fireEvent.dragOver(favoritesRow);
    await fireEvent.drop(favoritesRow);

    await waitFor(() => {
      expect(reorderSpy).toHaveBeenCalledWith(['cat-2', 'cat-1']);
    });
  });
});
