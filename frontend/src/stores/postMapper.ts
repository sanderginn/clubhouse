import type {
  Post,
  Link,
  LinkMetadata,
  PostImage,
  Highlight,
  RecipeStats,
  MovieStats,
  BookStats,
  EmbedData,
  RecipeMetadata,
  MovieMetadata,
  MovieCastMember,
  MovieSeason,
  PodcastMetadata,
  PodcastHighlightEpisode,
} from './postStore';

export interface ApiUser {
  id: string;
  username: string;
  profile_picture_url?: string;
}

export interface ApiLink {
  id?: string;
  url: string;
  metadata?: Record<string, unknown> | string | null;
  podcast?: unknown;
  highlights?: unknown;
}

export interface ApiPostImage {
  id: string;
  url: string;
  position: number;
  caption?: string | null;
  alt_text?: string | null;
  created_at?: string;
}

export interface ApiPost {
  id: string;
  user_id: string;
  section_id: string;
  content: string;
  links?: ApiLink[];
  images?: ApiPostImage[];
  user?: ApiUser;
  comment_count?: number;
  reaction_counts?: Record<string, number>;
  viewer_reactions?: string[];
  recipe_stats?: ApiRecipeStats | null;
  recipeStats?: ApiRecipeStats | null;
  movie_stats?: ApiMovieStats | null;
  movieStats?: ApiMovieStats | null;
  book_stats?: ApiBookStats | null;
  bookStats?: ApiBookStats | null;
  created_at: string;
  updated_at?: string;
}

export interface ApiRecipeStats {
  save_count?: number | null;
  cook_count?: number | null;
  avg_rating?: number | null;
  average_rating?: number | null;
  saveCount?: number | null;
  cookCount?: number | null;
  avgRating?: number | null;
  averageRating?: number | null;
}

export interface ApiMovieStats {
  watchlist_count?: number | null;
  watch_count?: number | null;
  avg_rating?: number | null;
  average_rating?: number | null;
  viewer_watchlisted?: boolean | null;
  viewer_watched?: boolean | null;
  viewer_rating?: number | null;
  viewer_categories?: string[] | null;
  watchlistCount?: number | null;
  watchCount?: number | null;
  avgRating?: number | null;
  averageRating?: number | null;
  viewerWatchlisted?: boolean | null;
  viewerWatched?: boolean | null;
  viewerRating?: number | null;
  viewerCategories?: string[] | null;
}

export interface ApiBookStats {
  bookshelf_count?: number | null;
  read_count?: number | null;
  avg_rating?: number | null;
  average_rating?: number | null;
  viewer_on_bookshelf?: boolean | null;
  viewer_categories?: string[] | null;
  viewer_read?: boolean | null;
  viewer_rating?: number | null;
  bookshelfCount?: number | null;
  readCount?: number | null;
  avgRating?: number | null;
  averageRating?: number | null;
  viewerOnBookshelf?: boolean | null;
  viewerCategories?: string[] | null;
  viewerRead?: boolean | null;
  viewerRating?: number | null;
}

function normalizeString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function normalizeNumber(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === 'string') {
    const trimmed = value.trim();
    if (trimmed.length === 0) {
      return undefined;
    }
    const parsed = Number(trimmed);
    return Number.isFinite(parsed) ? parsed : undefined;
  }
  return undefined;
}

function parsePercentScore(value: unknown): number | undefined {
  const normalized = normalizeNumber(value);
  if (typeof normalized === 'number') {
    return normalized;
  }

  if (typeof value !== 'string') {
    return undefined;
  }

  const trimmed = value.trim();
  if (trimmed.length === 0) {
    return undefined;
  }

  const match = trimmed.match(/^(-?\d+(?:\.\d+)?)\s*(?:%|\/\s*100)$/i);
  if (!match?.[1]) {
    return undefined;
  }

  const parsed = Number(match[1]);
  return Number.isFinite(parsed) ? parsed : undefined;
}

function normalizePercentScore(...values: unknown[]): number | undefined {
  for (const value of values) {
    const parsed = parsePercentScore(value);
    if (typeof parsed === 'number') {
      return parsed;
    }
  }
  return undefined;
}

function normalizeStringArray(value: unknown): string[] | undefined {
  if (typeof value === 'string') {
    const normalized = normalizeString(value);
    return normalized ? [normalized] : undefined;
  }
  if (!Array.isArray(value)) {
    return undefined;
  }
  const normalized = value
    .map((entry) => normalizeString(entry))
    .filter((entry): entry is string => typeof entry === 'string' && entry.length > 0);
  return normalized.length > 0 ? normalized : undefined;
}

function normalizeTMDBMediaType(value: unknown): 'movie' | 'tv' | undefined {
  const normalized = normalizeString(value)?.toLowerCase();
  if (normalized === 'movie') {
    return 'movie';
  }
  if (normalized === 'tv' || normalized === 'series') {
    return 'tv';
  }
  return undefined;
}

function normalizePodcastKind(value: unknown): 'show' | 'episode' | undefined {
  const normalized = normalizeString(value)?.toLowerCase();
  if (normalized === 'show') {
    return 'show';
  }
  if (normalized === 'episode') {
    return 'episode';
  }
  return undefined;
}

function normalizePodcastHighlightEpisodes(rawEpisodes: unknown): PodcastHighlightEpisode[] | undefined {
  if (!Array.isArray(rawEpisodes)) {
    return undefined;
  }

  const episodes = rawEpisodes
    .map((rawEpisode): PodcastHighlightEpisode | null => {
      if (!rawEpisode || typeof rawEpisode !== 'object' || Array.isArray(rawEpisode)) {
        return null;
      }

      const record = rawEpisode as Record<string, unknown>;
      const title = normalizeString(record.title);
      const url = normalizeString(record.url);
      if (!title || !url) {
        return null;
      }

      const note = normalizeString(record.note);
      return {
        title,
        url,
        ...(note ? { note } : {}),
      };
    })
    .filter((episode): episode is PodcastHighlightEpisode => episode !== null);

  return episodes.length > 0 ? episodes : undefined;
}

function normalizePodcastMetadata(rawPodcast: unknown): PodcastMetadata | undefined {
  if (!rawPodcast) {
    return undefined;
  }

  let podcast: Record<string, unknown> | null = null;
  if (typeof rawPodcast === 'string') {
    try {
      const parsed = JSON.parse(rawPodcast) as unknown;
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        podcast = parsed as Record<string, unknown>;
      }
    } catch {
      return undefined;
    }
  } else if (typeof rawPodcast === 'object' && !Array.isArray(rawPodcast)) {
    podcast = rawPodcast as Record<string, unknown>;
  }

  if (!podcast) {
    return undefined;
  }

  const kind = normalizePodcastKind(podcast.kind);
  const highlightEpisodes = normalizePodcastHighlightEpisodes(
    podcast.highlight_episodes ?? podcast.highlightEpisodes
  );

  if (!kind && !highlightEpisodes) {
    return undefined;
  }

  return {
    ...(kind ? { kind } : {}),
    ...(highlightEpisodes ? { highlightEpisodes } : {}),
  };
}

function normalizeMovieSeasons(rawSeasons: unknown): MovieSeason[] | undefined {
  if (!Array.isArray(rawSeasons)) {
    return undefined;
  }

  const seasons = rawSeasons
    .map((season): MovieSeason | null => {
      if (!season || typeof season !== 'object' || Array.isArray(season)) {
        return null;
      }

      const record = season as Record<string, unknown>;
      const seasonNumber = normalizeNumber(record.season_number ?? record.seasonNumber);
      if (typeof seasonNumber !== 'number') {
        return null;
      }

      const episodeCount = normalizeNumber(record.episode_count ?? record.episodeCount);
      const airDate = normalizeString(record.air_date ?? record.airDate);
      const name = normalizeString(record.name);
      const overview = normalizeString(record.overview);
      const poster =
        normalizeString(record.poster) ??
        normalizeString(record.poster_url) ??
        normalizeString(record.posterUrl);

      return {
        seasonNumber: Math.trunc(seasonNumber),
        ...(typeof episodeCount === 'number' ? { episodeCount: Math.trunc(episodeCount) } : {}),
        ...(airDate ? { airDate } : {}),
        ...(name ? { name } : {}),
        ...(overview ? { overview } : {}),
        ...(poster ? { poster } : {}),
      };
    })
    .filter((season): season is MovieSeason => season !== null)
    .sort((a, b) => a.seasonNumber - b.seasonNumber);

  return seasons.length > 0 ? seasons : undefined;
}

function normalizeRecipeMetadata(rawRecipe: unknown): RecipeMetadata | undefined {
  if (!rawRecipe) {
    return undefined;
  }
  let recipe: Record<string, unknown> | null = null;
  if (typeof rawRecipe === 'string') {
    try {
      const parsed = JSON.parse(rawRecipe) as unknown;
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        recipe = parsed as Record<string, unknown>;
      }
    } catch {
      return undefined;
    }
  } else if (typeof rawRecipe === 'object' && !Array.isArray(rawRecipe)) {
    recipe = rawRecipe as Record<string, unknown>;
  }

  if (!recipe) {
    return undefined;
  }

  const name = normalizeString(recipe.name) ?? normalizeString(recipe.title);
  const description = normalizeString(recipe.description);
  const image =
    normalizeString(recipe.image) ??
    normalizeString(recipe.image_url) ??
    normalizeString(recipe.imageUrl);
  const ingredients =
    normalizeStringArray(recipe.ingredients) ??
    normalizeStringArray(recipe.ingredient) ??
    normalizeStringArray(recipe.recipeIngredient) ??
    normalizeStringArray(recipe.recipe_ingredient);
  const instructions =
    normalizeStringArray(recipe.instructions) ??
    normalizeStringArray(recipe.instruction) ??
    normalizeStringArray(recipe.recipeInstructions) ??
    normalizeStringArray(recipe.recipe_instructions);
  const prepTime = normalizeString(recipe.prep_time ?? recipe.prepTime);
  const cookTime = normalizeString(recipe.cook_time ?? recipe.cookTime);
  const totalTime = normalizeString(recipe.total_time ?? recipe.totalTime);
  const yieldValue = normalizeString(recipe.yield);
  const author = normalizeString(recipe.author);
  const datePublished = normalizeString(recipe.date_published ?? recipe.datePublished);
  const cuisine = normalizeString(recipe.cuisine);
  const category = normalizeString(recipe.category);

  let nutrition: RecipeMetadata['nutrition'] | undefined;
  const rawNutrition =
    (recipe.nutrition ?? recipe.nutrition_info ?? recipe.nutritionInfo) as unknown;
  if (rawNutrition && typeof rawNutrition === 'object' && !Array.isArray(rawNutrition)) {
    const nutritionRecord = rawNutrition as Record<string, unknown>;
    const calories = normalizeString(nutritionRecord.calories ?? nutritionRecord.calorie);
    const servings = normalizeString(nutritionRecord.servings ?? nutritionRecord.serving);
    if (calories || servings) {
      nutrition = {
        ...(calories ? { calories } : {}),
        ...(servings ? { servings } : {}),
      };
    }
  }

  const hasRecipe =
    !!name ||
    !!description ||
    !!image ||
    (ingredients?.length ?? 0) > 0 ||
    (instructions?.length ?? 0) > 0 ||
    !!prepTime ||
    !!cookTime ||
    !!totalTime ||
    !!yieldValue ||
    !!author ||
    !!datePublished ||
    !!cuisine ||
    !!category ||
    !!nutrition;

  if (!hasRecipe) {
    return undefined;
  }

  return {
    ...(name ? { name } : {}),
    ...(description ? { description } : {}),
    ...(image ? { image } : {}),
    ...(ingredients ? { ingredients } : {}),
    ...(instructions ? { instructions } : {}),
    ...(prepTime ? { prep_time: prepTime } : {}),
    ...(cookTime ? { cook_time: cookTime } : {}),
    ...(totalTime ? { total_time: totalTime } : {}),
    ...(yieldValue ? { yield: yieldValue } : {}),
    ...(author ? { author } : {}),
    ...(datePublished ? { date_published: datePublished } : {}),
    ...(cuisine ? { cuisine } : {}),
    ...(category ? { category } : {}),
    ...(nutrition ? { nutrition } : {}),
  };
}

function normalizeMovieMetadata(rawMovie: unknown): MovieMetadata | undefined {
  if (!rawMovie) {
    return undefined;
  }

  let movie: Record<string, unknown> | null = null;
  if (typeof rawMovie === 'string') {
    try {
      const parsed = JSON.parse(rawMovie) as unknown;
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        movie = parsed as Record<string, unknown>;
      }
    } catch {
      return undefined;
    }
  } else if (typeof rawMovie === 'object' && !Array.isArray(rawMovie)) {
    movie = rawMovie as Record<string, unknown>;
  }

  if (!movie) {
    return undefined;
  }

  const title = normalizeString(movie.title) ?? normalizeString(movie.name);
  const overview = normalizeString(movie.overview) ?? normalizeString(movie.description);
  const poster =
    normalizeString(movie.poster) ??
    normalizeString(movie.poster_url) ??
    normalizeString(movie.posterUrl);
  const backdrop =
    normalizeString(movie.backdrop) ??
    normalizeString(movie.backdrop_url) ??
    normalizeString(movie.backdropUrl);
  const runtime = normalizeNumber(movie.runtime);
  const genres = normalizeStringArray(movie.genres);
  const releaseDate = normalizeString(movie.release_date ?? movie.releaseDate);
  const director = normalizeString(movie.director);
  const tmdbRating = normalizeNumber(movie.tmdb_rating ?? movie.tmdbRating);
  const rottenTomatoesScore = normalizePercentScore(
    movie.rotten_tomatoes_score,
    movie.rottenTomatoesScore
  );
  const metacriticScore = normalizePercentScore(movie.metacritic_score, movie.metacriticScore);
  const trailerKey = normalizeString(movie.trailer_key ?? movie.trailerKey);
  const tmdbId = normalizeNumber(movie.tmdb_id ?? movie.tmdbId);
  const tmdbMediaType = normalizeTMDBMediaType(
    movie.tmdb_media_type ?? movie.tmdbMediaType
  );
  const seasons = normalizeMovieSeasons(movie.seasons);

  const cast = Array.isArray(movie.cast)
    ? movie.cast
        .map((castMember) => {
          if (!castMember || typeof castMember !== 'object' || Array.isArray(castMember)) {
            return null;
          }
          const castRecord = castMember as Record<string, unknown>;
          const name = normalizeString(castRecord.name);
          if (!name) {
            return null;
          }
          const character = normalizeString(castRecord.character);
          const normalizedCastMember: MovieCastMember = {
            name,
          };
          if (character) {
            normalizedCastMember.character = character;
          }
          return normalizedCastMember;
        })
        .filter((value): value is MovieCastMember => value !== null)
    : undefined;

  const hasMovieMetadata =
    !!title ||
    !!overview ||
    !!poster ||
    !!backdrop ||
    typeof runtime === 'number' ||
    !!(genres && genres.length > 0) ||
    !!releaseDate ||
    !!director ||
    typeof tmdbRating === 'number' ||
    typeof rottenTomatoesScore === 'number' ||
    typeof metacriticScore === 'number' ||
    !!trailerKey ||
    typeof tmdbId === 'number' ||
    !!tmdbMediaType ||
    !!(seasons && seasons.length > 0) ||
    !!(cast && cast.length > 0);

  if (!hasMovieMetadata) {
    return undefined;
  }

  return {
    ...(title ? { title } : {}),
    ...(overview ? { overview } : {}),
    ...(poster ? { poster } : {}),
    ...(backdrop ? { backdrop } : {}),
    ...(typeof runtime === 'number' ? { runtime } : {}),
    ...(genres ? { genres } : {}),
    ...(releaseDate ? { releaseDate } : {}),
    ...(cast ? { cast } : {}),
    ...(director ? { director } : {}),
    ...(typeof tmdbRating === 'number' ? { tmdbRating } : {}),
    ...(typeof rottenTomatoesScore === 'number' ? { rottenTomatoesScore } : {}),
    ...(typeof metacriticScore === 'number' ? { metacriticScore } : {}),
    ...(trailerKey ? { trailerKey } : {}),
    ...(typeof tmdbId === 'number' ? { tmdbId } : {}),
    ...(tmdbMediaType ? { tmdbMediaType } : {}),
    ...(seasons ? { seasons } : {}),
  };
}

function normalizeRecipeStats(rawStats: unknown): RecipeStats | undefined {
  if (!rawStats || typeof rawStats !== 'object') {
    return undefined;
  }
  const record = rawStats as Record<string, unknown>;
  const saveCount = normalizeNumber(record.save_count ?? record.saveCount) ?? 0;
  const cookCount = normalizeNumber(record.cook_count ?? record.cookCount) ?? 0;
  const averageRating =
    normalizeNumber(
      record.avg_rating ??
        record.avgRating ??
        record.average_rating ??
        record.averageRating
    ) ?? null;

  return {
    saveCount,
    cookCount,
    averageRating,
  };
}

function normalizeMovieStats(rawStats: unknown): MovieStats | undefined {
  if (!rawStats || typeof rawStats !== 'object') {
    return undefined;
  }

  const record = rawStats as Record<string, unknown>;
  const watchlistCount = normalizeNumber(record.watchlist_count ?? record.watchlistCount) ?? 0;
  const watchCount = normalizeNumber(record.watch_count ?? record.watchCount) ?? 0;
  const averageRating =
    normalizeNumber(
      record.avg_rating ??
        record.avgRating ??
        record.average_rating ??
        record.averageRating
    ) ?? null;

  const viewerWatchlisted =
    typeof record.viewer_watchlisted === 'boolean'
      ? record.viewer_watchlisted
      : typeof record.viewerWatchlisted === 'boolean'
        ? record.viewerWatchlisted
        : undefined;
  const viewerWatched =
    typeof record.viewer_watched === 'boolean'
      ? record.viewer_watched
      : typeof record.viewerWatched === 'boolean'
        ? record.viewerWatched
        : undefined;
  const viewerRating = normalizeNumber(record.viewer_rating ?? record.viewerRating) ?? undefined;
  const viewerCategories = normalizeStringArray(
    record.viewer_categories ?? record.viewerCategories
  );

  return {
    watchlistCount,
    watchCount,
    averageRating,
    ...(typeof viewerWatchlisted === 'boolean' ? { viewerWatchlisted } : {}),
    ...(typeof viewerWatched === 'boolean' ? { viewerWatched } : {}),
    ...(typeof viewerRating === 'number' ? { viewerRating } : {}),
    ...(viewerCategories ? { viewerCategories } : {}),
  };
}

function normalizeBookStats(rawStats: unknown): BookStats | undefined {
  if (!rawStats || typeof rawStats !== 'object') {
    return undefined;
  }

  const record = rawStats as Record<string, unknown>;
  const bookshelfCount = normalizeNumber(record.bookshelf_count ?? record.bookshelfCount) ?? 0;
  const readCount = normalizeNumber(record.read_count ?? record.readCount) ?? 0;
  const averageRating =
    normalizeNumber(
      record.avg_rating ??
        record.avgRating ??
        record.average_rating ??
        record.averageRating
    ) ?? null;
  const viewerOnBookshelf =
    typeof record.viewer_on_bookshelf === 'boolean'
      ? record.viewer_on_bookshelf
      : typeof record.viewerOnBookshelf === 'boolean'
        ? record.viewerOnBookshelf
        : undefined;
  const viewerCategories = normalizeStringArray(
    record.viewer_categories ?? record.viewerCategories
  );
  const viewerRead =
    typeof record.viewer_read === 'boolean'
      ? record.viewer_read
      : typeof record.viewerRead === 'boolean'
        ? record.viewerRead
        : undefined;
  const viewerRating = normalizeNumber(record.viewer_rating ?? record.viewerRating) ?? undefined;

  return {
    bookshelfCount,
    readCount,
    averageRating,
    ...(typeof viewerOnBookshelf === 'boolean' ? { viewerOnBookshelf } : {}),
    ...(viewerCategories ? { viewerCategories } : {}),
    ...(typeof viewerRead === 'boolean' ? { viewerRead } : {}),
    ...(typeof viewerRating === 'number' ? { viewerRating } : {}),
  };
}

function normalizeEmbedData(rawEmbed: unknown): EmbedData | undefined {
  if (!rawEmbed || typeof rawEmbed !== 'object' || Array.isArray(rawEmbed)) {
    return undefined;
  }
  const record = rawEmbed as Record<string, unknown>;
  const embedUrl =
    normalizeString(record.embedUrl) ??
    normalizeString(record.embed_url) ??
    normalizeString(record.url);
  if (!embedUrl) {
    return undefined;
  }
  const type = normalizeString(record.type);
  const provider = normalizeString(record.provider);
  const width =
    normalizeNumber(record.width) ?? normalizeNumber(record.embed_width ?? record.embedWidth);
  const height =
    normalizeNumber(record.height) ?? normalizeNumber(record.embed_height ?? record.embedHeight);
  return {
    type,
    provider,
    embedUrl,
    width,
    height,
  };
}

function mergePodcastIntoMetadata(rawMetadata: unknown, rawPodcast: unknown): unknown {
  if (!rawPodcast) {
    return rawMetadata;
  }

  if (rawMetadata && typeof rawMetadata === 'object' && !Array.isArray(rawMetadata)) {
    return {
      ...(rawMetadata as Record<string, unknown>),
      podcast: rawPodcast,
    };
  }

  if (typeof rawMetadata === 'string') {
    try {
      const parsed = JSON.parse(rawMetadata) as unknown;
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        return {
          ...(parsed as Record<string, unknown>),
          podcast: rawPodcast,
        };
      }
    } catch {
      // Fall through and preserve podcast metadata even if metadata string is invalid.
    }
  }

  return { podcast: rawPodcast };
}

function normalizeHighlights(rawHighlights: unknown): Highlight[] | undefined {
  if (!Array.isArray(rawHighlights)) {
    return undefined;
  }

  const normalized = rawHighlights
    .map((item) => {
      if (!item || typeof item !== 'object') {
        return null;
      }
      const record = item as {
        id?: unknown;
        timestamp?: unknown;
        label?: unknown;
        heart_count?: unknown;
        heartCount?: unknown;
        viewer_reacted?: unknown;
        viewerReacted?: unknown;
      };
      if (typeof record.timestamp !== 'number' || !Number.isFinite(record.timestamp)) {
        return null;
      }
      const label =
        typeof record.label === 'string' && record.label.trim().length > 0
          ? record.label
          : undefined;
      const id = typeof record.id === 'string' && record.id.trim().length > 0 ? record.id : undefined;
      const heartCount =
        normalizeNumber(record.heart_count ?? record.heartCount) ?? undefined;
      const viewerReacted =
        typeof record.viewer_reacted === 'boolean'
          ? record.viewer_reacted
          : typeof record.viewerReacted === 'boolean'
            ? record.viewerReacted
            : undefined;
      return {
        timestamp: record.timestamp,
        ...(label ? { label } : {}),
        ...(id ? { id } : {}),
        ...(typeof heartCount === 'number' ? { heartCount } : {}),
        ...(typeof viewerReacted === 'boolean' ? { viewerReacted } : {}),
      } as Highlight;
    })
    .filter(Boolean) as Highlight[];

  return normalized.length > 0 ? normalized : undefined;
}

export function normalizeLinkMetadata(
  rawMetadata: unknown,
  linkUrl: string
): LinkMetadata | undefined {
  if (!rawMetadata) {
    return undefined;
  }

  let metadata: Record<string, unknown> | null = null;
  if (typeof rawMetadata === 'string') {
    try {
      const parsed = JSON.parse(rawMetadata) as unknown;
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        metadata = parsed as Record<string, unknown>;
      }
    } catch {
      return undefined;
    }
  } else if (typeof rawMetadata === 'object' && !Array.isArray(rawMetadata)) {
    metadata = rawMetadata as Record<string, unknown>;
  }

  if (!metadata) {
    return undefined;
  }

  const url = normalizeString(metadata.url) ?? linkUrl;
  const provider =
    normalizeString(metadata.provider) ??
    normalizeString(metadata.site_name) ??
    normalizeString(metadata.siteName);
  const title = normalizeString(metadata.title) ?? normalizeString(metadata.name);
  const description =
    normalizeString(metadata.description) ?? normalizeString(metadata.summary);
  const image =
    normalizeString(metadata.image) ??
    normalizeString(metadata.image_url) ??
    normalizeString(metadata.imageUrl);
  const author = normalizeString(metadata.author) ?? normalizeString(metadata.artist);
  const duration = normalizeNumber(metadata.duration);
  const embedUrl = normalizeString(metadata.embedUrl) ?? normalizeString(metadata.embed_url);
  const embed =
    normalizeEmbedData(metadata.embed) ??
    (embedUrl
      ? {
          embedUrl,
          provider: normalizeString(metadata.embed_provider ?? metadata.embedProvider),
          type: normalizeString(metadata.embed_type ?? metadata.embedType),
          width: normalizeNumber(metadata.embed_width ?? metadata.embedWidth),
          height: normalizeNumber(metadata.embed_height ?? metadata.embedHeight),
        }
      : undefined);
  const resolvedEmbedUrl = embed?.embedUrl ?? embedUrl;
  const type =
    normalizeString(metadata.type) ??
    normalizeString(metadata.og_type) ??
    normalizeString(metadata.ogType);
  const recipe = normalizeRecipeMetadata(metadata.recipe);
  const movie = normalizeMovieMetadata(metadata.movie);
  const podcast = normalizePodcastMetadata(metadata.podcast);

  const hasMetadata =
    !!provider ||
    !!title ||
    !!description ||
    !!image ||
    !!author ||
    !!duration ||
    !!resolvedEmbedUrl ||
    !!embed ||
    !!type ||
    !!recipe ||
    !!movie ||
    !!podcast;
  if (!hasMetadata) {
    return undefined;
  }

  return {
    url,
    provider,
    title,
    description,
    image,
    author,
    duration,
    embedUrl: resolvedEmbedUrl,
    embed,
    type,
    ...(recipe ? { recipe } : {}),
    ...(movie ? { movie } : {}),
    ...(podcast ? { podcast } : {}),
  };
}

export function mapApiPost(apiPost: ApiPost): Post {
  const images: PostImage[] | undefined = apiPost.images?.map((image) => ({
    id: image.id,
    url: image.url,
    position: image.position,
    caption: image.caption ?? undefined,
    altText: image.alt_text ?? undefined,
    createdAt: image.created_at,
  }));

  return {
    id: apiPost.id,
    userId: apiPost.user_id,
    sectionId: apiPost.section_id,
    content: apiPost.content,
    links: apiPost.links?.map((link): Link => ({
      id: link.id,
      url: link.url,
      metadata: normalizeLinkMetadata(
        mergePodcastIntoMetadata(link.metadata, link.podcast),
        link.url
      ),
      highlights: normalizeHighlights(link.highlights),
    })),
    images,
    user: apiPost.user
      ? {
          id: apiPost.user.id,
          username: apiPost.user.username,
          profilePictureUrl: apiPost.user.profile_picture_url,
        }
      : undefined,
    commentCount: apiPost.comment_count,
    reactionCounts: apiPost.reaction_counts ?? undefined,
    viewerReactions: apiPost.viewer_reactions,
    recipeStats: normalizeRecipeStats(apiPost.recipe_stats ?? apiPost.recipeStats),
    movieStats: normalizeMovieStats(apiPost.movie_stats ?? apiPost.movieStats),
    bookStats: normalizeBookStats(apiPost.book_stats ?? apiPost.bookStats),
    createdAt: apiPost.created_at,
    updatedAt: apiPost.updated_at,
  };
}
