import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it } from 'vitest';

const { default: MovieCard } = await import('../movies/MovieCard.svelte');

type MovieMetadata = {
  title: string;
  overview?: string;
  poster?: string;
  backdrop?: string;
  runtime?: number;
  genres?: string[];
  releaseDate?: string;
  cast?: { name: string; character: string }[];
  director?: string;
  tmdbRating?: number;
  trailerKey?: string;
};

const fullMovie: MovieMetadata = {
  title: 'Interstellar',
  overview: "A team travels through a wormhole to secure humanity's future.",
  poster: 'https://example.com/interstellar.jpg',
  backdrop: 'https://example.com/interstellar-backdrop.jpg',
  runtime: 169,
  genres: ['Sci-Fi', 'Drama', 'Adventure', 'Mystery'],
  releaseDate: '2014-11-07',
  cast: [
    { name: 'Matthew McConaughey', character: 'Cooper' },
    { name: 'Anne Hathaway', character: 'Brand' },
    { name: 'Jessica Chastain', character: 'Murph' },
    { name: 'Michael Caine', character: 'Professor Brand' },
    { name: 'Matt Damon', character: 'Mann' },
    { name: 'Casey Affleck', character: 'Tom' },
  ],
  director: 'Christopher Nolan',
  tmdbRating: 8.6,
  trailerKey: 'zSWdZVtXT7E',
};

afterEach(() => {
  cleanup();
});

describe('MovieCard', () => {
  it('renders collapsed movie metadata with key stats and badges', () => {
    render(MovieCard, { movie: fullMovie });

    expect(screen.getByTestId('movie-title')).toHaveTextContent('Interstellar (2014)');
    expect(screen.getByTestId('movie-meta-line')).toHaveTextContent('★ 8.6 · 2h 49m');
    expect(screen.getByTestId('movie-director')).toHaveTextContent('Dir: Christopher Nolan');
    expect(screen.getByTestId('movie-genres')).toHaveTextContent('Sci-Fi');
    expect(screen.getByTestId('movie-genres')).toHaveTextContent('Drama');
    expect(screen.getByTestId('movie-genres')).toHaveTextContent('Adventure');
    expect(screen.getByTestId('movie-genres')).toHaveTextContent('+1');
    expect(screen.queryByTestId('movie-expanded-content')).not.toBeInTheDocument();
  });

  it('renders fallback content for minimal metadata', () => {
    render(MovieCard, { movie: { title: 'Untitled' } });

    expect(screen.getByTestId('movie-meta-line')).toHaveTextContent('★ N/A');
    expect(screen.getByTestId('movie-director')).toHaveTextContent('Dir: Unknown');
    expect(screen.getByTestId('movie-poster-fallback')).toBeInTheDocument();
    expect(screen.queryByTestId('movie-genres')).not.toBeInTheDocument();
  });

  it('shows poster fallback when poster loading fails', async () => {
    render(MovieCard, {
      movie: {
        title: 'Poster test',
        poster: 'https://example.com/missing.jpg',
      },
    });

    const poster = screen.getByTestId('movie-poster');
    await fireEvent.error(poster);

    expect(screen.getByTestId('movie-poster-fallback')).toBeInTheDocument();
  });

  it('opens trailer modal and supports keyboard close + focus return', async () => {
    render(MovieCard, { movie: fullMovie, expanded: true });

    const openButton = screen.getByTestId('movie-trailer-button');
    openButton.focus();
    await fireEvent.click(openButton);

    const dialog = screen.getByRole('dialog', { name: 'Trailer for Interstellar' });
    const closeButton = screen.getByTestId('movie-trailer-close');

    expect(dialog).toBeInTheDocument();
    expect(screen.getByTestId('youtube-embed-frame')).toBeInTheDocument();
    expect(closeButton).toHaveFocus();

    await fireEvent.keyDown(dialog, { key: 'Tab', shiftKey: true });
    expect(closeButton).toHaveFocus();

    await fireEvent.keyDown(window, { key: 'Escape' });
    expect(
      screen.queryByRole('dialog', { name: 'Trailer for Interstellar' })
    ).not.toBeInTheDocument();
    expect(openButton).toHaveFocus();
  });

  it('toggles expanded content via the details button', async () => {
    render(MovieCard, { movie: fullMovie });

    const toggle = screen.getByTestId('movie-expand-toggle');
    expect(screen.queryByTestId('movie-expanded-content')).not.toBeInTheDocument();

    await fireEvent.click(toggle);
    expect(screen.getByTestId('movie-expanded-content')).toBeInTheDocument();
    expect(screen.getByTestId('movie-overview')).toHaveTextContent(fullMovie.overview || '');
    expect(screen.getByTestId('movie-cast-list')).toHaveTextContent('Matthew McConaughey');
    expect(screen.getByTestId('movie-cast-list')).not.toHaveTextContent('Casey Affleck');

    await fireEvent.click(toggle);
    expect(screen.queryByTestId('movie-expanded-content')).not.toBeInTheDocument();
  });
});
