import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it } from 'vitest';

const { default: BookCard } = await import('./BookCard.svelte');

type BookData = {
  title?: string;
  authors?: string[];
  description?: string;
  cover_url?: string;
  page_count?: number;
  genres?: string[];
  publish_date?: string;
  goodreads_url?: string;
  open_library_key?: string;
};

const fullBookData: BookData = {
  title: 'Neuromancer',
  authors: ['William Gibson', 'Jane Doe'],
  description:
    'A cyberpunk classic that helped define the genre and follows a washed-up hacker pulled into a final dangerous heist.',
  cover_url: 'https://covers.openlibrary.org/b/id/12345-L.jpg',
  page_count: 271,
  genres: ['Science Fiction', 'Cyberpunk', 'Noir', 'Thriller'],
  publish_date: '1984-07-01',
  goodreads_url: 'https://www.goodreads.com/book/show/22328-neuromancer',
  open_library_key: '/works/OL45883W',
};

afterEach(() => {
  cleanup();
});

describe('BookCard', () => {
  it('renders title, authors, and description', () => {
    render(BookCard, {
      bookData: fullBookData,
      threadHref: '/sections/books/posts/post-123',
    });

    expect(screen.getByTestId('book-title')).toHaveTextContent('Neuromancer');
    expect(screen.getByTestId('book-thread-link')).toHaveAttribute('href', '/sections/books/posts/post-123');
    expect(screen.getByTestId('book-authors')).toHaveTextContent('William Gibson, Jane Doe');
    expect(screen.getByTestId('book-description')).toHaveTextContent('A cyberpunk classic');
    expect(screen.getByTestId('book-details')).toHaveTextContent('271 pages · 1984');
  });

  it('displays the cover image when provided', () => {
    render(BookCard, { bookData: fullBookData });

    const cover = screen.getByTestId('book-cover');
    expect(cover).toHaveAttribute('src', 'https://covers.openlibrary.org/b/id/12345-L.jpg');
    expect(cover).toHaveAttribute('alt', 'Neuromancer cover');
  });

  it('shows Goodreads button when Goodreads URL is available', () => {
    render(BookCard, { bookData: fullBookData });

    const goodreadsLink = screen.getByTestId('book-goodreads-link');
    expect(goodreadsLink).toHaveAttribute(
      'href',
      'https://www.goodreads.com/book/show/22328-neuromancer'
    );
    expect(goodreadsLink).toHaveTextContent('View on Goodreads');
  });

  it('does not render unsafe or mislabeled external links', () => {
    render(BookCard, {
      bookData: {
        ...fullBookData,
        goodreads_url: 'javascript:alert(1)',
        open_library_key: 'https://evil.example/works/OL45883W',
      },
    });

    expect(screen.queryByTestId('book-goodreads-link')).not.toBeInTheDocument();
    expect(screen.queryByTestId('book-open-library-link')).not.toBeInTheDocument();
  });

  it('uses compact description truncation class in compact mode', () => {
    render(BookCard, {
      bookData: fullBookData,
      compact: true,
    });

    expect(screen.getByTestId('book-description')).toHaveClass('line-clamp-2');
  });

  it('handles missing optional fields gracefully', () => {
    render(BookCard, {
      bookData: {
        title: 'Title only',
      },
    });

    expect(screen.getByTestId('book-title')).toHaveTextContent('Title only');
    expect(screen.getByTestId('book-authors')).toHaveTextContent('Unknown author');
    expect(screen.getByTestId('book-cover-fallback')).toBeInTheDocument();
    expect(screen.queryByTestId('book-details')).not.toBeInTheDocument();
    expect(screen.queryByTestId('book-description')).not.toBeInTheDocument();
    expect(screen.queryByTestId('book-goodreads-link')).not.toBeInTheDocument();
    expect(screen.queryByTestId('book-open-library-link')).not.toBeInTheDocument();
  });
});
