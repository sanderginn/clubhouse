import { describe, it, expect } from 'vitest';
import { mapApiPost } from '../postMapper';

describe('mapApiPost', () => {
  it('maps ids, user, counts, timestamps, and metadata', () => {
    const post = mapApiPost({
      id: 'post-1',
      user_id: 'user-1',
      section_id: 'section-1',
      content: 'hello',
      comment_count: 2,
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-02T00:00:00Z',
      user: {
        id: 'user-1',
        username: 'sander',
        profile_picture_url: 'https://example.com/avatar.png',
      },
      links: [
        {
          id: 'link-1',
          url: 'https://example.com',
          metadata: {
            url: 'https://example.com',
            provider: 'example',
            title: 'Example',
            description: 'Desc',
            image: 'https://example.com/image.png',
            author: 'Author',
            duration: 120,
            embed: {
              embed_url: 'https://open.spotify.com/embed/track/xyz',
              provider: 'spotify',
              height: 152,
            },
          },
        },
      ],
    });

    expect(post.id).toBe('post-1');
    expect(post.user?.username).toBe('sander');
    expect(post.commentCount).toBe(2);
    expect(post.links?.[0].metadata?.embedUrl).toBe('https://open.spotify.com/embed/track/xyz');
    expect(post.links?.[0].metadata?.embed?.provider).toBe('spotify');
    expect(post.links?.[0].metadata?.embed?.height).toBe(152);
    expect(post.links?.[0].metadata?.author).toBe('Author');
    expect(post.links?.[0].metadata?.duration).toBe(120);
  });

  it('maps embed metadata', () => {
    const post = mapApiPost({
      id: 'post-embed',
      user_id: 'user-embed',
      section_id: 'section-embed',
      content: 'hello',
      created_at: '2025-01-01T00:00:00Z',
      links: [
        {
          id: 'link-embed',
          url: 'https://www.youtube.com/watch?v=dQw4w9WgXcQ',
          metadata: {
            embed: {
              type: 'iframe',
              provider: 'youtube',
              embed_url: 'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ',
              width: 560,
              height: 315,
            },
          },
        },
      ],
    });

    expect(post.links?.[0].metadata?.embed?.provider).toBe('youtube');
    expect(post.links?.[0].metadata?.embed?.embedUrl).toBe(
      'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ'
    );
  });

  it('maps recipe metadata', () => {
    const post = mapApiPost({
      id: 'post-recipe',
      user_id: 'user-recipe',
      section_id: 'section-recipe',
      content: 'recipe',
      created_at: '2025-01-01T00:00:00Z',
      links: [
        {
          id: 'link-recipe',
          url: 'https://example.com/recipe',
          metadata: {
            recipe: {
              name: 'Best Pasta',
              ingredients: ['1 cup flour'],
              instructions: ['Mix well'],
              prep_time: 'PT10M',
              nutrition: {
                calories: '200',
                servings: '2',
              },
            },
          },
        },
      ],
    });

    expect(post.links?.[0].metadata?.recipe?.name).toBe('Best Pasta');
    expect(post.links?.[0].metadata?.recipe?.ingredients).toEqual(['1 cup flour']);
    expect(post.links?.[0].metadata?.recipe?.instructions).toEqual(['Mix well']);
    expect(post.links?.[0].metadata?.recipe?.prep_time).toBe('PT10M');
    expect(post.links?.[0].metadata?.recipe?.nutrition?.calories).toBe('200');
  });

  it('maps movie stats from API shape', () => {
    const post = mapApiPost({
      id: 'post-movie-stats',
      user_id: 'user-movie-stats',
      section_id: 'section-movie',
      content: 'Movie stats',
      created_at: '2025-01-01T00:00:00Z',
      movie_stats: {
        watchlist_count: 9,
        watch_count: 4,
        avg_rating: 4.25,
        viewer_watchlisted: true,
        viewer_watched: false,
        viewer_rating: 5,
        viewer_categories: ['Top Picks'],
      },
    });

    expect(post.movieStats?.watchlistCount).toBe(9);
    expect(post.movieStats?.watchCount).toBe(4);
    expect(post.movieStats?.averageRating).toBe(4.25);
    expect(post.movieStats?.viewerWatchlisted).toBe(true);
    expect(post.movieStats?.viewerWatched).toBe(false);
    expect(post.movieStats?.viewerRating).toBe(5);
    expect(post.movieStats?.viewerCategories).toEqual(['Top Picks']);
  });

  it('maps book stats from API shape', () => {
    const post = mapApiPost({
      id: 'post-book-stats',
      user_id: 'user-book-stats',
      section_id: 'section-book',
      content: 'Book stats',
      created_at: '2025-01-01T00:00:00Z',
      book_stats: {
        bookshelf_count: 8,
        read_count: 5,
        average_rating: 4.4,
        viewer_on_bookshelf: true,
        viewer_categories: ['Favorites'],
        viewer_read: true,
        viewer_rating: 5,
      },
    });

    expect(post.bookStats?.bookshelfCount).toBe(8);
    expect(post.bookStats?.readCount).toBe(5);
    expect(post.bookStats?.averageRating).toBe(4.4);
    expect(post.bookStats?.viewerOnBookshelf).toBe(true);
    expect(post.bookStats?.viewerCategories).toEqual(['Favorites']);
    expect(post.bookStats?.viewerRead).toBe(true);
    expect(post.bookStats?.viewerRating).toBe(5);
  });

  it('preserves and normalizes nested movie metadata payload', () => {
    const post = mapApiPost({
      id: 'post-movie-metadata',
      user_id: 'user-movie-metadata',
      section_id: 'section-movie',
      content: 'Movie metadata',
      created_at: '2025-01-01T00:00:00Z',
      links: [
        {
          id: 'link-movie',
          url: 'https://www.imdb.com/title/tt0816692/',
          metadata: {
            movie: {
              title: 'Interstellar',
              overview: 'A team travels through a wormhole.',
              poster_url: 'https://example.com/poster.jpg',
              backdrop_url: 'https://example.com/backdrop.jpg',
              runtime: '169',
              genres: ['Sci-Fi', 'Drama'],
              release_date: '2014-11-07',
              director: 'Christopher Nolan',
              tmdb_rating: 8.6,
              rotten_tomatoes_score: '88',
              metacriticScore: 73,
              trailer_key: 'zSWdZVtXT7E',
              tmdb_id: '157336',
              tmdb_media_type: 'movie',
              seasons: [
                {
                  season_number: 0,
                  episode_count: 2,
                  air_date: '2014-01-01',
                  name: 'Specials',
                  poster_url: 'https://example.com/specials.jpg',
                },
                {
                  season_number: 1,
                  episode_count: '10',
                  air_date: '2015-01-01',
                  name: 'Season 1',
                  poster: 'https://example.com/season-1.jpg',
                },
              ],
              cast: [
                { name: 'Matthew McConaughey', character: 'Cooper' },
                { name: 'Anne Hathaway', character: 'Brand' },
              ],
            },
          },
        },
      ],
    });

    const movie = post.links?.[0].metadata?.movie;
    expect(movie?.title).toBe('Interstellar');
    expect(movie?.overview).toBe('A team travels through a wormhole.');
    expect(movie?.poster).toBe('https://example.com/poster.jpg');
    expect(movie?.backdrop).toBe('https://example.com/backdrop.jpg');
    expect(movie?.runtime).toBe(169);
    expect(movie?.genres).toEqual(['Sci-Fi', 'Drama']);
    expect(movie?.releaseDate).toBe('2014-11-07');
    expect(movie?.director).toBe('Christopher Nolan');
    expect(movie?.tmdbRating).toBe(8.6);
    expect(movie?.rottenTomatoesScore).toBe(88);
    expect(movie?.metacriticScore).toBe(73);
    expect(movie?.trailerKey).toBe('zSWdZVtXT7E');
    expect(movie?.tmdbId).toBe(157336);
    expect(movie?.tmdbMediaType).toBe('movie');
    expect(movie?.seasons).toEqual([
      {
        seasonNumber: 0,
        episodeCount: 2,
        airDate: '2014-01-01',
        name: 'Specials',
        poster: 'https://example.com/specials.jpg',
      },
      {
        seasonNumber: 1,
        episodeCount: 10,
        airDate: '2015-01-01',
        name: 'Season 1',
        poster: 'https://example.com/season-1.jpg',
      },
    ]);
    expect(movie?.cast).toEqual([
      { name: 'Matthew McConaughey', character: 'Cooper' },
      { name: 'Anne Hathaway', character: 'Brand' },
    ]);
  });

  it('normalizes formatted movie score strings in nested metadata', () => {
    const post = mapApiPost({
      id: 'post-movie-formatted-scores',
      user_id: 'user-movie-formatted-scores',
      section_id: 'section-movie',
      content: 'Movie metadata with formatted scores',
      created_at: '2025-01-01T00:00:00Z',
      links: [
        {
          id: 'link-movie-formatted',
          url: 'https://www.imdb.com/title/tt0133093/',
          metadata: {
            movie: {
              title: 'The Matrix',
              rotten_tomatoes_score: '88%',
              metacritic_score: '73/100',
            },
          },
        },
      ],
    });

    const movie = post.links?.[0].metadata?.movie;
    expect(movie?.rottenTomatoesScore).toBe(88);
    expect(movie?.metacriticScore).toBe(73);
  });

  it('maps podcast metadata from link podcast payload', () => {
    const post = mapApiPost({
      id: 'post-podcast-metadata',
      user_id: 'user-podcast-metadata',
      section_id: 'section-podcast',
      content: 'Podcast metadata',
      created_at: '2025-01-01T00:00:00Z',
      links: [
        {
          id: 'link-podcast',
          url: 'https://podcasts.apple.com/us/podcast/example/id123456789',
          metadata: {
            title: 'Example Show',
          },
          podcast: {
            kind: 'show',
            highlight_episodes: [
              {
                title: 'Episode 1',
                url: 'https://example.com/episode-1',
                note: 'Start here',
              },
            ],
          },
        },
      ],
    });

    const podcast = post.links?.[0].metadata?.podcast;
    expect(podcast?.kind).toBe('show');
    expect(podcast?.highlightEpisodes).toEqual([
      {
        title: 'Episode 1',
        url: 'https://example.com/episode-1',
        note: 'Start here',
      },
    ]);
    expect(post.links?.[0].metadata?.title).toBe('Example Show');
  });

  it('maps podcast metadata from nested metadata payload', () => {
    const post = mapApiPost({
      id: 'post-podcast-nested-metadata',
      user_id: 'user-podcast-nested',
      section_id: 'section-podcast',
      content: 'Podcast metadata nested',
      created_at: '2025-01-01T00:00:00Z',
      links: [
        {
          id: 'link-podcast-nested',
          url: 'https://open.spotify.com/show/abc123',
          metadata: {
            podcast: {
              kind: 'episode',
              highlightEpisodes: [
                {
                  title: 'Episode 2',
                  url: 'https://example.com/episode-2',
                },
              ],
            },
          },
        },
      ],
    });
    const podcast = post.links?.[0].metadata?.podcast;
    expect(podcast?.kind).toBe('episode');
    expect(podcast?.highlightEpisodes).toEqual([
      {
        title: 'Episode 2',
        url: 'https://example.com/episode-2',
      },
    ]);
  });

  it('handles missing user and links gracefully', () => {
    const post = mapApiPost({
      id: 'post-2',
      user_id: 'user-2',
      section_id: 'section-2',
      content: 'hello',
      created_at: '2025-01-01T00:00:00Z',
    });

    expect(post.user).toBeUndefined();
    expect(post.links).toBeUndefined();
  });

  it('maps post images', () => {
    const post = mapApiPost({
      id: 'post-6',
      user_id: 'user-6',
      section_id: 'section-6',
      content: 'hello',
      created_at: '2025-01-01T00:00:00Z',
      images: [
        {
          id: 'image-1',
          url: 'https://example.com/one.png',
          position: 0,
          caption: 'First',
          alt_text: 'First image',
          created_at: '2025-01-01T00:00:00Z',
        },
      ],
    });

    expect(post.images?.[0].id).toBe('image-1');
    expect(post.images?.[0].url).toBe('https://example.com/one.png');
    expect(post.images?.[0].caption).toBe('First');
    expect(post.images?.[0].altText).toBe('First image');
  });

  it('falls back to link url when metadata url is missing', () => {
    const post = mapApiPost({
      id: 'post-3',
      user_id: 'user-3',
      section_id: 'section-3',
      content: 'hello',
      created_at: '2025-01-01T00:00:00Z',
      links: [
        {
          id: 'link-3',
          url: 'https://example.com/photo.png',
          metadata: {
            image: 'https://cdn.example.com/photo.png',
          },
        },
      ],
    });

    expect(post.links?.[0].metadata?.url).toBe('https://example.com/photo.png');
    expect(post.links?.[0].metadata?.image).toBe('https://cdn.example.com/photo.png');
  });

  it('falls back to link url when metadata url is empty', () => {
    const post = mapApiPost({
      id: 'post-4',
      user_id: 'user-4',
      section_id: 'section-4',
      content: 'hello',
      created_at: '2025-01-01T00:00:00Z',
      links: [
        {
          id: 'link-4',
          url: 'https://example.com/photo.png',
          metadata: {
            url: '',
            image: 'https://cdn.example.com/photo.png',
          },
        },
      ],
    });

    expect(post.links?.[0].metadata?.url).toBe('https://example.com/photo.png');
    expect(post.links?.[0].metadata?.image).toBe('https://cdn.example.com/photo.png');
  });

  it('normalizes snake_case metadata and JSON strings', () => {
    const rawMetadata = JSON.stringify({
      site_name: 'Example Site',
      title: 'Example Title',
      description: 'Example Description',
      image_url: 'https://cdn.example.com/image.png',
      embed_url: 'https://example.com/embed',
      embed_height: '232',
      duration: '180',
    });

    const post = mapApiPost({
      id: 'post-5',
      user_id: 'user-5',
      section_id: 'section-5',
      content: 'hello',
      created_at: '2025-01-01T00:00:00Z',
      links: [
        {
          id: 'link-5',
          url: 'https://example.com',
          metadata: rawMetadata,
        },
      ],
    });

    expect(post.links?.[0].metadata?.provider).toBe('Example Site');
    expect(post.links?.[0].metadata?.title).toBe('Example Title');
    expect(post.links?.[0].metadata?.description).toBe('Example Description');
    expect(post.links?.[0].metadata?.image).toBe('https://cdn.example.com/image.png');
    expect(post.links?.[0].metadata?.embedUrl).toBe('https://example.com/embed');
    expect(post.links?.[0].metadata?.embed?.height).toBe(232);
    expect(post.links?.[0].metadata?.duration).toBe(180);
  });

  it('normalizes embed objects', () => {
    const post = mapApiPost({
      id: 'post-7',
      user_id: 'user-7',
      section_id: 'section-7',
      content: 'hello',
      created_at: '2025-01-01T00:00:00Z',
      links: [
        {
          id: 'link-7',
          url: 'https://artist.bandcamp.com/album/test',
          metadata: {
            embed: {
              type: 'iframe',
              provider: 'bandcamp',
              embed_url: 'https://bandcamp.com/EmbeddedPlayer/album=123',
              height: 470,
            },
          },
        },
      ],
    });

    expect(post.links?.[0].metadata?.embed?.embedUrl).toBe(
      'https://bandcamp.com/EmbeddedPlayer/album=123'
    );
    expect(post.links?.[0].metadata?.embed?.height).toBe(470);
    expect(post.links?.[0].metadata?.embedUrl).toBe(
      'https://bandcamp.com/EmbeddedPlayer/album=123'
    );
  });

  it('normalizes soundcloud embed metadata', () => {
    const post = mapApiPost({
      id: 'post-7',
      user_id: 'user-7',
      section_id: 'section-7',
      content: 'hello',
      created_at: '2025-01-01T00:00:00Z',
      links: [
        {
          id: 'link-7',
          url: 'https://soundcloud.com/artist/track',
          metadata: {
            provider: 'soundcloud',
            embed: {
              provider: 'soundcloud',
              embed_url: 'https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/tracks/1',
              height: 166,
            },
          },
        },
      ],
    });

    const metadata = post.links?.[0].metadata;
    expect(metadata?.embedUrl).toBe(
      'https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/tracks/1'
    );
    expect(metadata?.embed?.provider).toBe('soundcloud');
    expect(metadata?.embed?.height).toBe(166);
  });
});
