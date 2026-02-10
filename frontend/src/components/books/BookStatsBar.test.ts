import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('./BookshelfSaveButton.svelte', async () => ({
  default: (await import('./BookshelfSaveButtonPropsStub.svelte')).default,
}));

vi.mock('./ReadButton.svelte', async () => ({
  default: (await import('./ReadButtonPropsStub.svelte')).default,
}));

const { default: BookStatsBar } = await import('./BookStatsBar.svelte');

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('BookStatsBar', () => {
  it('renders all stats', () => {
    render(BookStatsBar, {
      postId: 'post-1',
      bookStats: {
        bookshelfCount: 3,
        readCount: 5,
        averageRating: 4.2,
        viewerOnBookshelf: true,
        viewerCategories: ['Favorites'],
        viewerRead: true,
        viewerRating: 4,
      },
    });

    expect(screen.getByTestId('book-stats-bar')).toBeInTheDocument();
    expect(screen.getByTestId('book-bookshelf-count')).toHaveTextContent('3');
    expect(screen.getByText('saved')).toBeInTheDocument();
    expect(screen.getByTestId('book-read-count')).toHaveTextContent('5');
    expect(screen.getByText('read')).toBeInTheDocument();
    expect(screen.getByTestId('book-average-rating-value')).toHaveTextContent('4.2');
  });

  it('hides average rating when there are no ratings', () => {
    render(BookStatsBar, {
      postId: 'post-2',
      bookStats: {
        bookshelfCount: 1,
        readCount: 2,
        averageRating: 0,
        viewerOnBookshelf: false,
        viewerCategories: [],
        viewerRead: false,
        viewerRating: null,
      },
    });

    expect(screen.queryByTestId('book-average-rating')).not.toBeInTheDocument();
  });

  it('renders BookshelfSaveButton and ReadButton', () => {
    render(BookStatsBar, {
      postId: 'post-3',
      bookStats: {
        bookshelfCount: 7,
        readCount: 4,
        averageRating: 3.8,
        viewerOnBookshelf: true,
        viewerCategories: ['Classics', 'Favorites'],
        viewerRead: true,
        viewerRating: 5,
      },
    });

    const bookshelfButton = screen.getByTestId('bookshelf-save-button-stub');
    expect(bookshelfButton).toBeInTheDocument();
    expect(bookshelfButton).toHaveAttribute('data-post-id', 'post-3');
    expect(bookshelfButton).toHaveAttribute('data-viewer-on-bookshelf', 'true');

    const readButton = screen.getByTestId('read-button-stub');
    expect(readButton).toBeInTheDocument();
    expect(readButton).toHaveAttribute('data-post-id', 'post-3');
    expect(readButton).toHaveAttribute('data-viewer-read', 'true');
  });

  it('uses mobile-first responsive layout classes', () => {
    render(BookStatsBar, {
      postId: 'post-4',
      bookStats: {
        bookshelfCount: 2,
        readCount: 1,
        averageRating: 4,
        viewerOnBookshelf: false,
        viewerCategories: [],
        viewerRead: false,
        viewerRating: null,
      },
    });

    const container = screen.getByTestId('book-stats-bar');
    expect(container.className).toContain('flex-col');
    expect(container.className).toContain('sm:flex-row');
  });
});
