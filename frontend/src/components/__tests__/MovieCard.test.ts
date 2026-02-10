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
  rottenTomatoesScore?: number | string;
  rotten_tomatoes_score?: number | string;
  metacriticScore?: number | string;
  metacritic_score?: number | string;
  imdbId?: string;
  imdb_id?: string;
  rottenTomatoesUrl?: string;
  rotten_tomatoes_url?: string;
  trailerKey?: string;
  tmdbId?: number;
  tmdbMediaType?: 'movie' | 'tv';
  seasons?: Array<{
    seasonNumber: number;
    episodeCount?: number;
    airDate?: string;
    name?: string;
    poster?: string;
  }>;
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
  tmdbId: 157336,
  tmdbMediaType: 'movie',
};

const seriesWithSeasons: MovieMetadata = {
  title: 'Breaking Bad',
  overview: 'A chemistry teacher turns to making meth.',
  poster: 'https://example.com/breaking-bad.jpg',
  runtime: 47,
  genres: ['Drama', 'Crime', 'Thriller'],
  releaseDate: '2008-01-20',
  tmdbRating: 8.9,
  tmdbId: 1396,
  tmdbMediaType: 'tv',
  seasons: [
    {
      seasonNumber: 0,
      episodeCount: 4,
      airDate: '2009-02-17',
      name: 'Specials',
      poster: 'https://example.com/specials.jpg',
    },
    {
      seasonNumber: 1,
      episodeCount: 7,
      airDate: '2008-01-20',
      name: 'Season 1',
      poster: 'https://example.com/s1.jpg',
    },
    {
      seasonNumber: 2,
      episodeCount: 13,
      airDate: '2009-03-08',
      name: 'Season 2',
      poster: 'https://example.com/s2.jpg',
    },
  ],
};

afterEach(() => {
  cleanup();
});

describe('MovieCard', () => {
  it('renders collapsed movie metadata with key stats and badges', () => {
    render(MovieCard, { movie: fullMovie });

    expect(screen.getByTestId('movie-title')).toHaveTextContent('Interstellar (2014)');
    expect(screen.getByTestId('movie-meta-line')).toHaveTextContent('★ 8.6· 2h 49m');
    expect(screen.getByTestId('movie-rating-tmdb')).toHaveTextContent('★ 8.6');
    expect(screen.queryByTestId('movie-rating-rotten-tomatoes')).not.toBeInTheDocument();
    expect(screen.queryByTestId('movie-rating-metacritic')).not.toBeInTheDocument();
    expect(screen.getByTestId('movie-director')).toHaveTextContent('Dir: Christopher Nolan');
    expect(screen.getByTestId('movie-genres')).toHaveTextContent('Sci-Fi');
    expect(screen.getByTestId('movie-genres')).toHaveTextContent('Drama');
    expect(screen.getByTestId('movie-genres')).toHaveTextContent('Adventure');
    expect(screen.getByTestId('movie-genres')).toHaveTextContent('+1');
    const tmdbLink = screen.getByTestId('movie-tmdb-link');
    expect(tmdbLink).toHaveAttribute('href', 'https://www.themoviedb.org/movie/157336');
    expect(tmdbLink).toHaveClass(
      'rounded-md',
      'border',
      'bg-slate-50',
      'text-sm',
      'focus-visible:outline'
    );
    expect(screen.getByRole('link', { name: /view interstellar on tmdb/i })).toBe(tmdbLink);
    expect(screen.queryByTestId('movie-expanded-content')).not.toBeInTheDocument();
  });

  it('renders TMDB, Rotten Tomatoes, and Metacritic ratings when provided', () => {
    render(MovieCard, {
      movie: {
        ...fullMovie,
        rottenTomatoesScore: 88,
        metacriticScore: 73,
      },
    });

    expect(screen.getByTestId('movie-meta-line')).toHaveTextContent('★ 8.6· 🍅 88%· MC 73· 2h 49m');
    expect(screen.getByTestId('movie-rating-rotten-tomatoes')).toHaveClass(
      'text-emerald-800',
      'bg-emerald-100'
    );
    expect(screen.getByTestId('movie-rating-metacritic')).toHaveTextContent('MC 73');
  });

  it('applies RT warning and rotten color states by score thresholds', async () => {
    const { rerender } = render(MovieCard, {
      movie: {
        ...fullMovie,
        tmdbRating: undefined,
        rottenTomatoesScore: 52,
      },
    });

    expect(screen.getByTestId('movie-rating-rotten-tomatoes')).toHaveClass(
      'text-amber-800',
      'bg-amber-100'
    );
    expect(screen.queryByTestId('movie-rating-fallback')).not.toBeInTheDocument();

    await rerender({
      movie: {
        ...fullMovie,
        tmdbRating: undefined,
        rottenTomatoesScore: 31,
      },
    });

    expect(screen.getByTestId('movie-rating-rotten-tomatoes')).toHaveClass(
      'text-rose-800',
      'bg-rose-100'
    );
  });

  it('normalizes snake_case formatted score strings for RT and Metacritic', () => {
    render(MovieCard, {
      movie: {
        ...fullMovie,
        rotten_tomatoes_score: '91%',
        metacritic_score: '74/100',
      },
    });

    expect(screen.getByTestId('movie-rating-rotten-tomatoes')).toHaveTextContent('🍅 91%');
    expect(screen.getByTestId('movie-rating-rotten-tomatoes')).toHaveClass(
      'text-emerald-800',
      'bg-emerald-100'
    );
    expect(screen.getByTestId('movie-rating-metacritic')).toHaveTextContent('MC 74');
  });

  it('shows RT score when expanded is true', () => {
    render(MovieCard, {
      movie: {
        ...fullMovie,
        rottenTomatoesScore: 88,
      },
      expanded: true,
    });

    expect(screen.getByTestId('movie-rating-rotten-tomatoes')).toHaveTextContent('🍅 88%');
    expect(screen.getByTestId('movie-expanded-content')).toBeInTheDocument();
  });

  it('renders season summary and expanded season list for series metadata', async () => {
    render(MovieCard, {
      movie: seriesWithSeasons,
      expanded: true,
    });

    expect(screen.getByTestId('movie-meta-line')).toHaveTextContent('★ 8.9· 47m· 3 Seasons');
    expect(screen.getByTestId('movie-seasons-section')).toBeInTheDocument();
    expect(screen.getByTestId('movie-seasons-list')).toBeInTheDocument();
    expect(screen.getByTestId('movie-season-number-0')).toHaveTextContent('S0');
    expect(screen.getByTestId('movie-season-name-1')).toHaveTextContent('Season 1');
    expect(screen.getByTestId('movie-season-details-2')).toHaveTextContent('13 episodes · 2009');

    const seasonsToggle = screen.getByTestId('movie-seasons-toggle');
    await fireEvent.click(seasonsToggle);
    expect(screen.queryByTestId('movie-seasons-list')).not.toBeInTheDocument();
    expect(seasonsToggle).toHaveTextContent('Show 3 Seasons');
  });

  it('shows season poster fallback when a season thumbnail fails to load', async () => {
    render(MovieCard, {
      movie: seriesWithSeasons,
      expanded: true,
    });

    const seasonPoster = screen.getByTestId('movie-season-poster-1');
    await fireEvent.error(seasonPoster);

    expect(screen.getByTestId('movie-season-poster-fallback-1')).toBeInTheDocument();
  });

  it('renders a secure TMDB link for TV metadata', () => {
    render(MovieCard, {
      movie: {
        ...fullMovie,
        tmdbId: 1399,
        tmdbMediaType: 'tv',
      },
    });

    const tmdbLink = screen.getByTestId('movie-tmdb-link');
    expect(tmdbLink).toHaveAttribute('href', 'https://www.themoviedb.org/tv/1399');
    expect(tmdbLink).toHaveAttribute('target', '_blank');
    expect(tmdbLink).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('renders TMDB, IMDb, and Rotten Tomatoes links when available', () => {
    render(MovieCard, {
      movie: {
        ...fullMovie,
        imdbId: 'tt0816692',
        rottenTomatoesUrl: 'https://www.rottentomatoes.com/m/interstellar_2014',
      },
    });

    const tmdbLink = screen.getByTestId('movie-tmdb-link');
    const imdbLink = screen.getByTestId('movie-imdb-link');
    const rtLink = screen.getByTestId('movie-rotten-tomatoes-link');

    expect(tmdbLink).toHaveAttribute('href', 'https://www.themoviedb.org/movie/157336');
    expect(imdbLink).toHaveAttribute('href', 'https://www.imdb.com/title/tt0816692');
    expect(rtLink).toHaveAttribute(
      'href',
      'https://www.rottentomatoes.com/m/interstellar_2014'
    );
    expect(imdbLink).toHaveAttribute('target', '_blank');
    expect(rtLink).toHaveAttribute('target', '_blank');
    expect(imdbLink).toHaveAttribute('rel', 'noopener noreferrer');
    expect(rtLink).toHaveAttribute('rel', 'noopener noreferrer');
    expect(imdbLink).toHaveClass(
      'rounded-md',
      'border',
      'bg-slate-50',
      'text-sm',
      'focus-visible:outline'
    );
    expect(rtLink).toHaveClass(
      'rounded-md',
      'border',
      'bg-slate-50',
      'text-sm',
      'focus-visible:outline'
    );
  });

  it('does not render TMDB link when tmdb id is missing', () => {
    render(MovieCard, {
      movie: {
        ...fullMovie,
        tmdbId: undefined,
      },
    });

    expect(screen.queryByTestId('movie-tmdb-link')).not.toBeInTheDocument();
  });

  it('omits IMDb and Rotten Tomatoes links when metadata is unavailable', () => {
    render(MovieCard, {
      movie: {
        ...fullMovie,
        imdbId: 'invalid-id',
        rottenTomatoesUrl: '',
      },
    });

    expect(screen.queryByTestId('movie-imdb-link')).not.toBeInTheDocument();
    expect(screen.queryByTestId('movie-rotten-tomatoes-link')).not.toBeInTheDocument();
  });

  it('renders fallback content for minimal metadata', () => {
    render(MovieCard, { movie: { title: 'Untitled' } });

    expect(screen.getByTestId('movie-meta-line')).toHaveTextContent('No rating');
    expect(screen.getByTestId('movie-rating-fallback')).toHaveTextContent('No rating');
    expect(screen.queryByTestId('movie-rating-tmdb')).not.toBeInTheDocument();
    expect(screen.getByTestId('movie-director')).toHaveTextContent('Dir: Unknown');
    expect(screen.getByTestId('movie-poster-fallback')).toBeInTheDocument();
    expect(screen.queryByTestId('movie-genres')).not.toBeInTheDocument();
  });

  it('does not render season information for movie metadata', async () => {
    render(MovieCard, {
      movie: {
        ...fullMovie,
        seasons: [
          {
            seasonNumber: 1,
            episodeCount: 10,
            airDate: '2014-01-01',
          },
        ],
      },
      expanded: true,
    });

    expect(screen.getByTestId('movie-meta-line')).toHaveTextContent('★ 8.6· 2h 49m');
    expect(screen.queryByTestId('movie-seasons-section')).not.toBeInTheDocument();
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
