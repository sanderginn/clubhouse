<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { fade } from 'svelte/transition';
  import type { Link, LinkMetadata, Post } from '../stores/postStore';
  import { postStore, currentUser, isAdmin, activeView } from '../stores';
  import { api } from '../services/api';
  import CommentThread from './comments/CommentThread.svelte';
  import EditedBadge from './EditedBadge.svelte';
  import ReactionBar from './reactions/ReactionBar.svelte';
  import RelativeTime from './RelativeTime.svelte';
  import HighlightDisplay from './posts/HighlightDisplay.svelte';
  import { buildProfileHref, handleProfileNavigation } from '../services/profileNavigation';
  import { buildThreadHref } from '../services/routeNavigation';
  import LinkifiedText from './LinkifiedText.svelte';
  import MentionTextarea from './mentions/MentionTextarea.svelte';
  import LinkPreview from './posts/LinkPreview.svelte';
  import { getImageLinkUrl, isInternalUploadUrl, stripInternalUploadUrls } from '../services/linkUtils';
  import { sections } from '../stores/sectionStore';
  import { getSectionSlugById } from '../services/sectionSlug';
  import { logError } from '../lib/observability/logger';
  import { isYouTubeUrl, isSpotifyUrl, isSoundCloudUrl, parseYouTubeUrl, parseSpotifyUrl, fetchSoundCloudEmbed } from '$lib/embeds/urlParsers';
  import { recordComponentRender } from '../lib/observability/performance';
  import { lockBodyScroll, unlockBodyScroll } from '../lib/scrollLock';
  import RecipeCard from './recipes/RecipeCard.svelte';
  import RecipeStatsBar from './recipes/RecipeStatsBar.svelte';
  import BookCard from './books/BookCard.svelte';
  import BookStatsBar from './books/BookStatsBar.svelte';
  import QuoteList from './books/QuoteList.svelte';
  import MovieCard from './movies/MovieCard.svelte';
  import MovieStatsBar from './movies/MovieStatsBar.svelte';
  import PodcastSaveButton from './podcasts/PodcastSaveButton.svelte';
  import BandcampEmbed from '../lib/components/embeds/BandcampEmbed.svelte';
  import SoundCloudEmbed from '../lib/components/embeds/SoundCloudEmbed.svelte';
  import SpotifyEmbed from '../lib/components/embeds/SpotifyEmbed.svelte';
  import YouTubeEmbed from '../lib/components/embeds/YouTubeEmbed.svelte';
  import type { EmbedController } from '../lib/embeds/controller';

  export let post: Post;
  export let highlightCommentId: string | null = null;
  export let highlightCommentIds: string[] = [];
  export let showSectionPill: boolean = false;
  export let showSectionLabel: boolean = false;
  export let profileUserId: string | null = null;
  export let highlightQuery: string = '';

  type ImageItem = {
    id?: string;
    url: string;
    title: string;
    altText?: string;
    link?: Link;
  };
  type SoundCloudEmbedData = {
    embedUrl: string;
    height?: number;
  };
  type MovieCastMember = {
    name: string;
    character?: string;
  };
  type MovieSeason = {
    seasonNumber?: number;
    season_number?: number;
    episodeCount?: number;
    episode_count?: number;
    airDate?: string;
    air_date?: string;
    name?: string;
    overview?: string;
    poster?: string;
    poster_url?: string;
    posterUrl?: string;
  };
  type MovieMetadata = {
    title?: string;
    overview?: string;
    poster?: string;
    backdrop?: string;
    runtime?: number;
    genres?: string[];
    releaseDate?: string;
    release_date?: string;
    cast?: MovieCastMember[];
    director?: string;
    tmdbRating?: number;
    tmdb_rating?: number;
    rottenTomatoesScore?: number | string;
    rotten_tomatoes_score?: number | string;
    metacriticScore?: number | string;
    metacritic_score?: number | string;
    imdbId?: string;
    imdb_id?: string;
    rottenTomatoesUrl?: string;
    rotten_tomatoes_url?: string;
    trailerKey?: string;
    trailer_key?: string;
    tmdbId?: number;
    tmdb_id?: number;
    tmdbMediaType?: string;
    tmdb_media_type?: string;
    seasons?: MovieSeason[];
  };
  type MovieCardSeason = {
    seasonNumber: number;
    episodeCount?: number;
    airDate?: string;
    name?: string;
    overview?: string;
    poster?: string;
  };
  type MovieCardMetadata = {
    title: string;
    overview?: string;
    poster?: string;
    backdrop?: string;
    runtime?: number;
    genres?: string[];
    releaseDate?: string;
    cast?: Array<{ name: string; character: string }>;
    director?: string;
    tmdbRating?: number;
    rottenTomatoesScore?: number;
    metacriticScore?: number;
    imdbId?: string;
    rottenTomatoesUrl?: string;
    trailerKey?: string;
    tmdbId?: number;
    tmdbMediaType?: 'movie' | 'tv';
    seasons?: MovieCardSeason[];
  };
  type LinkMetadataWithMovie = LinkMetadata & {
    movie?: MovieMetadata;
  };
  type BookMetadata = {
    title?: string;
    authors?: string[];
    description?: string;
    coverUrl?: string;
    cover_url?: string;
    pageCount?: number;
    page_count?: number;
    genres?: string[];
    publishDate?: string;
    publish_date?: string;
    openLibraryKey?: string;
    open_library_key?: string;
    goodreadsUrl?: string;
    goodreads_url?: string;
  };
  type LinkMetadataWithBook = LinkMetadata & {
    book_data?: BookMetadata | string;
    bookData?: BookMetadata | string;
    book?: BookMetadata | string;
  };
  type PodcastHighlightEpisodeMetadata = {
    title?: string;
    url?: string;
    note?: string;
  };
  type PodcastMetadata = {
    kind?: 'show' | 'episode' | string;
    highlightEpisodes?: PodcastHighlightEpisodeMetadata[];
    highlight_episodes?: PodcastHighlightEpisodeMetadata[];
  };
  type PodcastCardMetadata = {
    kind: 'show' | 'episode';
    highlightEpisodes: Array<{
      title: string;
      url: string;
      note?: string;
    }>;
  };
  type PostWithSectionType = Post & {
    section?: {
      type?: string;
    };
  };

  let soundCloudEmbedFromFrontend: SoundCloudEmbedData | null = null;

  onMount(async () => {
    const url = primaryLink?.url;
    if (url && isSoundCloudUrl(url) && !metadata?.embed?.embedUrl) {
      soundCloudEmbedFromFrontend = await fetchSoundCloudEmbed(url);
    }
  });

  $: userReactions = new Set(post.viewerReactions ?? []);
  $: sectionSlug = getSectionSlugById($sections, post.sectionId) ?? post.sectionId;
  $: sectionInfo = $sections.find((s) => s.id === post.sectionId) ?? null;
  function normalizeSectionType(value: string | null | undefined): string | null {
    if (typeof value !== 'string') {
      return null;
    }
    const normalized = value.trim().toLowerCase();
    if (normalized === 'books') {
      return 'book';
    }
    return normalized || null;
  }

  $: sectionType = normalizeSectionType(
    sectionInfo?.type ?? ((post as PostWithSectionType).section?.type ?? null)
  );
  $: recipeStats = post.recipeStats ?? post.recipe_stats ?? null;
  $: bookStats = post.bookStats ?? post.book_stats ?? null;
  $: movieStats = post.movieStats ?? post.movie_stats ?? null;
  $: isBookSection = sectionType === 'book';
  $: isMovieSection = sectionType === 'movie' || sectionType === 'series';
  $: isPodcastSection = sectionType === 'podcast';
  $: isThreadView = $activeView === 'thread';
  let copiedLink = false;
  let copyTimeout: ReturnType<typeof setTimeout> | null = null;
  let isEditing = false;
  let editContent = '';
  let editMentionUsernames: string[] = [];
  let editError: string | null = null;
  let isSaving = false;
  let editLinkAction: 'keep' | 'remove' | 'replace' = 'keep';
  let editLinkMetadata: LinkMetadata | null = null;
  let editLinkUrl = '';
  let editLinkInputValue = '';
  let editLinkInputError: string | null = null;
  let editLinkPreviewError: string | null = null;
  let isEditLinkInputVisible = false;
  let isEditLinkLoading = false;
  type EditImageState = {
    action: 'keep' | 'remove' | 'replace';
    uploadUrl: string | null;
    uploadError: string | null;
    uploading: boolean;
    progress: number;
  };
  let editImages: EditImageState[] = [];
  let editImageInputs: Array<HTMLInputElement | null> = [];
  let isImageLightboxOpen = false;
  let lightboxImageIndex = 0;
  let isDeleting = false;
  let deleteError: string | null = null;
  let embedController: EmbedController | null = null;
  let imageReplyTarget:
    | {
        id?: string;
        url: string;
        index: number;
        altText?: string;
      }
    | null = null;

  const MAX_UPLOAD_BYTES = 10 * 1024 * 1024;
  const MAX_UPLOAD_LABEL = '10 MB';

  const ALLOWED_IMAGE_MIME_TYPES = [
    'image/jpeg',
    'image/png',
    'image/gif',
    'image/webp',
    'image/bmp',
    'image/avif',
    'image/tiff',
  ];
  const ALLOWED_IMAGE_EXTENSIONS = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp', 'avif', 'tif', 'tiff'];
  const ACCEPTED_IMAGE_TYPES = [
    ...ALLOWED_IMAGE_MIME_TYPES,
    ...ALLOWED_IMAGE_EXTENSIONS.map((ext) => `.${ext}`),
  ].join(',');

  async function copyThreadLink() {
    if (typeof window === 'undefined') return;
    const url = new URL(buildThreadHref(sectionSlug, post.id), window.location.origin).toString();
    let copied = false;

    if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
      try {
        await navigator.clipboard.writeText(url);
        copied = true;
      } catch {
        copied = false;
      }
    }

    if (!copied && typeof document !== 'undefined' && typeof document.execCommand === 'function') {
      const textarea = document.createElement('textarea');
      textarea.value = url;
      textarea.setAttribute('readonly', '');
      textarea.style.position = 'absolute';
      textarea.style.left = '-9999px';
      document.body.appendChild(textarea);
      textarea.select();
      copied = document.execCommand('copy');
      document.body.removeChild(textarea);
    }

    if (copied) {
      copiedLink = true;
      if (copyTimeout) {
        clearTimeout(copyTimeout);
      }
      copyTimeout = setTimeout(() => {
        copiedLink = false;
      }, 2000);
    }
  }

  onDestroy(() => {
    if (copyTimeout) {
      clearTimeout(copyTimeout);
    }
    if (isImageLightboxOpen) {
      unlockBodyScroll();
    }
  });

  function openImageLightbox(index: number) {
    if (imageItems.length === 0) {
      return;
    }
    if (!isImageLightboxOpen) {
      lockBodyScroll();
    }
    const clamped = (index + imageItems.length) % imageItems.length;
    lightboxImageIndex = clamped;
    isImageLightboxOpen = true;
    preloadLightboxAdjacent(clamped);
  }

  function closeImageLightbox() {
    if (!isImageLightboxOpen) {
      return;
    }
    isImageLightboxOpen = false;
    unlockBodyScroll();
  }

  function goToLightboxImage(index: number) {
    if (imageItems.length === 0) {
      return;
    }
    const clamped = (index + imageItems.length) % imageItems.length;
    lightboxImageIndex = clamped;
    preloadLightboxAdjacent(clamped);
  }

  function nextLightboxImage() {
    goToLightboxImage(lightboxImageIndex + 1);
  }

  function previousLightboxImage() {
    goToLightboxImage(lightboxImageIndex - 1);
  }

  function handleLightboxKeydown(event: KeyboardEvent) {
    if (!isImageLightboxOpen) {
      return;
    }
    if (event.key === 'Escape') {
      closeImageLightbox();
    }
    if (imageItems.length <= 1) {
      return;
    }
    if (event.key === 'ArrowLeft') {
      event.preventDefault();
      previousLightboxImage();
    }
    if (event.key === 'ArrowRight') {
      event.preventDefault();
      nextLightboxImage();
    }
  }

  function handleLightboxTouchStart(event: TouchEvent) {
    if (imageItems.length <= 1) {
      return;
    }
    const touch = event.touches[0];
    if (!touch) {
      return;
    }
    lightboxTouchStartX = touch.clientX;
    lightboxTouchStartY = touch.clientY;
    lightboxTouchActive = true;
  }

  function handleLightboxTouchEnd(event: TouchEvent) {
    if (!lightboxTouchActive || imageItems.length <= 1) {
      lightboxTouchActive = false;
      return;
    }
    const touch = event.changedTouches[0];
    if (!touch) {
      lightboxTouchActive = false;
      return;
    }
    const deltaX = touch.clientX - lightboxTouchStartX;
    const deltaY = touch.clientY - lightboxTouchStartY;
    lightboxTouchActive = false;
    if (Math.abs(deltaX) > 40 && Math.abs(deltaX) > Math.abs(deltaY) * 1.5) {
      if (deltaX < 0) {
        nextLightboxImage();
      } else {
        previousLightboxImage();
      }
    }
  }

  function clearImageReplyTarget() {
    imageReplyTarget = null;
  }

  function scrollToCommentForm() {
    if (typeof document === 'undefined') return;
    const form = document.getElementById(`comment-form-${post.id}`);
    form?.scrollIntoView({ behavior: 'smooth', block: 'center' });
  }

  function startImageReply(index: number) {
    const item = imageItems[index];
    if (!item) {
      return;
    }
    imageReplyTarget = {
      id: item.id ?? undefined,
      url: item.url,
      index,
      altText: item.altText ?? item.title,
    };
    scrollToCommentForm();
  }

  function handleImageReferenceNavigate(index: number) {
    if (imageItems.length === 0) return;
    goToImage(index);
    if (typeof document !== 'undefined') {
      const container = document.getElementById(`post-images-${post.id}`);
      container?.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  }

  function buildEditImagesState(): EditImageState[] {
    return imageLinks.map(() => ({
      action: 'keep',
      uploadUrl: null,
      uploadError: null,
      uploading: false,
      progress: 0,
    }));
  }

  function resetEditImages() {
    editImages = buildEditImagesState();
    editImageInputs = new Array(editImages.length).fill(null);
    editImageLoadFailures = new Set();
    lastEditPreviewUrls = [];
  }

  function resetEditLinkState() {
    const currentLink = primaryLink && !primaryLinkIsImage ? primaryLink : null;
    editLinkAction = 'keep';
    editLinkMetadata = (currentLink?.metadata as LinkMetadata) ?? null;
    editLinkUrl = currentLink?.url ?? '';
    editLinkInputValue = currentLink?.url ?? '';
    editLinkInputError = null;
    editLinkPreviewError = null;
    isEditLinkInputVisible = false;
    isEditLinkLoading = false;
  }

  function removeEditLinkPreview() {
    editLinkAction = 'remove';
    editLinkMetadata = null;
    editLinkInputValue = editLinkUrl;
    editLinkInputError = null;
    editLinkPreviewError = null;
    isEditLinkInputVisible = false;
  }

  function isValidUrl(value: string): boolean {
    try {
      const parsed = new URL(value);
      return parsed.protocol === 'http:' || parsed.protocol === 'https:';
    } catch {
      return false;
    }
  }

  function openEditLinkInput() {
    if (isSaving || isEditLinkLoading) {
      return;
    }
    isEditLinkInputVisible = true;
    editLinkInputError = null;
    editLinkPreviewError = null;
  }

  function closeEditLinkInput() {
    isEditLinkInputVisible = false;
    editLinkInputError = null;
    editLinkPreviewError = null;
    editLinkInputValue = editLinkUrl;
  }

  async function submitEditLinkInput() {
    let value = editLinkInputValue.trim();
    if (!value) {
      editLinkInputError = 'Enter a link URL.';
      return;
    }

    if (!/^https?:\/\//i.test(value)) {
      value = `https://${value}`;
    }

    if (!isValidUrl(value)) {
      editLinkInputError = 'Enter a valid http(s) URL.';
      return;
    }

    editLinkInputError = null;
    editLinkPreviewError = null;
    isEditLinkLoading = true;

    try {
      const response = await api.previewLink(value);
      editLinkMetadata = response.metadata;
      editLinkUrl = value;
      editLinkInputValue = value;
      editLinkAction = 'replace';
      isEditLinkInputVisible = false;
    } catch (err) {
      editLinkPreviewError = err instanceof Error ? err.message : 'Failed to load preview';
      editLinkMetadata = null;
    } finally {
      isEditLinkLoading = false;
    }
  }

  function handleEditLinkInputKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter') {
      event.preventDefault();
      submitEditLinkInput();
    }
  }

  function startEdit() {
    editContent = post.content;
    editMentionUsernames = [];
    editError = null;
    resetEditImages();
    resetEditLinkState();
    isEditing = true;
  }

  function cancelEdit() {
    isEditing = false;
    editContent = post.content;
    editMentionUsernames = [];
    editError = null;
    resetEditImages();
    resetEditLinkState();
  }

  async function saveEdit() {
    const trimmed = editContent.trim();
    if (!trimmed) {
      editError = 'Content is required.';
      return;
    }
    if (isEditImageUploading) {
      return;
    }

    isSaving = true;
    editError = null;

    try {
      const linksPayload = buildEditLinksPayload();
      const response = await api.updatePost(post.id, {
        content: trimmed,
        links: linksPayload,
        removeLinkMetadata: editLinkAction === 'remove',
        mentionUsernames: editMentionUsernames,
      });
      postStore.upsertPost(response.post);
      post = { ...post, ...response.post };
      isEditing = false;
      editMentionUsernames = [];
    } catch (err) {
      editError = err instanceof Error ? err.message : 'Failed to update post';
    } finally {
      isSaving = false;
    }
  }

  function handleEditKeyDown(event: KeyboardEvent) {
    if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
      const trimmed = editContent.trim();
      if (!trimmed || isSaving || isEditImageUploading) {
        return;
      }
      event.preventDefault();
      saveEdit();
    }
  }

  async function toggleReaction(emoji: string) {
    const hasReacted = userReactions.has(emoji);
    // Optimistic update
    postStore.toggleReaction(post.id, emoji);

    try {
      if (hasReacted) {
        await api.removePostReaction(post.id, emoji);
      } else {
        await api.addPostReaction(post.id, emoji);
      }
    } catch (e) {
      logError('Failed to toggle reaction', { postId: post.id, emoji }, e);
      // Revert on error
      postStore.toggleReaction(post.id, emoji);
    }
  }

  async function toggleHighlightReaction(linkId: string, highlight: { id?: string; viewerReacted?: boolean }) {
    if (!highlight.id) {
      return;
    }
    const hasReacted = Boolean(highlight.viewerReacted);
    const delta = hasReacted ? -1 : 1;
    postStore.updateHighlightReaction(post.id, linkId, highlight.id, delta, !hasReacted);

    try {
      if (hasReacted) {
        await api.removeHighlightReaction(post.id, highlight.id);
      } else {
        await api.addHighlightReaction(post.id, highlight.id);
      }
    } catch (e) {
      logError('Failed to toggle highlight reaction', { postId: post.id, highlightId: highlight.id }, e);
      postStore.updateHighlightReaction(post.id, linkId, highlight.id, -delta, hasReacted);
    }
  }

  function handleHighlightReaction(highlight: { id?: string; viewerReacted?: boolean }) {
    if (!primaryLink?.id) {
      return;
    }
    void toggleHighlightReaction(primaryLink.id, highlight);
  }

  function getProviderIcon(provider: string | undefined): string {
    switch (provider) {
      case 'spotify':
        return '🎵';
      case 'youtube':
        return '▶️';
      case 'soundcloud':
        return '☁️';
      case 'imdb':
      case 'rottentomatoes':
        return '🎬';
      case 'goodreads':
        return '📚';
      case 'eventbrite':
      case 'ra':
        return '📅';
      default:
        return '🔗';
    }
  }

  function isSpotifyEmbedUrl(value: string | undefined): boolean {
    if (!value) return false;
    return value.includes('open.spotify.com/embed/');
  }

  function getSeekUnavailableMessage(provider: string | undefined): string {
    if (!provider) return 'Seeking not supported for this embed.';
    if (provider === 'spotify' || provider === 'bandcamp') return 'Seeking not supported for this embed.';
    if (provider === 'soundcloud') return 'Player is still loading.';
    return 'Seeking not supported for this embed.';
  }

  async function handleHighlightSeek(timestamp: number): Promise<boolean> {
    if (!embedController || !embedController.supportsSeeking) return false;
    try {
      await embedController.seekTo(timestamp);
      return true;
    } catch {
      return false;
    }
  }

  const handleEmbedReady = (controller: EmbedController) => {
    embedController = controller;
  };

  function parseMoviePercentScore(value: unknown): number | undefined {
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value;
    }
    if (typeof value !== 'string') {
      return undefined;
    }

    const trimmed = value.trim();
    if (!trimmed) {
      return undefined;
    }

    const direct = Number(trimmed);
    if (Number.isFinite(direct)) {
      return direct;
    }

    const match = trimmed.match(/^(-?\d+(?:\.\d+)?)\s*(?:%|\/\s*100)$/i);
    if (!match?.[1]) {
      return undefined;
    }

    const parsed = Number(match[1]);
    return Number.isFinite(parsed) ? parsed : undefined;
  }

  function normalizeMoviePercentScore(...values: unknown[]): number | undefined {
    for (const value of values) {
      const parsed = parseMoviePercentScore(value);
      if (typeof parsed === 'number') {
        return parsed;
      }
    }
    return undefined;
  }

  function normalizeBookMetadata(rawBook: unknown): BookMetadata | null {
    if (!rawBook) {
      return null;
    }

    let parsedBook: Record<string, unknown> | null = null;
    if (typeof rawBook === 'string') {
      try {
        const maybeObject = JSON.parse(rawBook) as unknown;
        if (maybeObject && typeof maybeObject === 'object' && !Array.isArray(maybeObject)) {
          parsedBook = maybeObject as Record<string, unknown>;
        }
      } catch {
        return null;
      }
    } else if (typeof rawBook === 'object' && !Array.isArray(rawBook)) {
      parsedBook = rawBook as Record<string, unknown>;
    }

    if (!parsedBook) {
      return null;
    }

    const normalizeString = (value: unknown): string | undefined => {
      if (typeof value !== 'string') {
        return undefined;
      }
      const trimmed = value.trim();
      return trimmed.length > 0 ? trimmed : undefined;
    };

    const normalizeNumber = (value: unknown): number | undefined => {
      if (typeof value !== 'number' || !Number.isFinite(value)) {
        return undefined;
      }
      const rounded = Math.round(value);
      return rounded > 0 ? rounded : undefined;
    };

    const normalizeStringArray = (value: unknown): string[] | undefined => {
      if (!Array.isArray(value)) {
        return undefined;
      }
      const normalized = value
        .filter((entry): entry is string => typeof entry === 'string')
        .map((entry) => entry.trim())
        .filter((entry) => entry.length > 0);
      return normalized.length > 0 ? normalized : undefined;
    };

    const title = normalizeString(parsedBook.title);
    const authors = normalizeStringArray(parsedBook.authors);
    const description = normalizeString(parsedBook.description);
    const coverURL =
      normalizeString(parsedBook.cover_url) ?? normalizeString(parsedBook.coverUrl);
    const pageCount =
      normalizeNumber(parsedBook.page_count) ?? normalizeNumber(parsedBook.pageCount);
    const genres = normalizeStringArray(parsedBook.genres);
    const publishDate =
      normalizeString(parsedBook.publish_date) ?? normalizeString(parsedBook.publishDate);
    const openLibraryKey =
      normalizeString(parsedBook.open_library_key) ?? normalizeString(parsedBook.openLibraryKey);
    const goodreadsURL =
      normalizeString(parsedBook.goodreads_url) ?? normalizeString(parsedBook.goodreadsUrl);

    if (
      !title &&
      !authors &&
      !description &&
      !coverURL &&
      !pageCount &&
      !genres &&
      !publishDate &&
      !openLibraryKey &&
      !goodreadsURL
    ) {
      return null;
    }

    return {
      ...(title ? { title } : {}),
      ...(authors ? { authors } : {}),
      ...(description ? { description } : {}),
      ...(coverURL ? { cover_url: coverURL } : {}),
      ...(typeof pageCount === 'number' ? { page_count: pageCount } : {}),
      ...(genres ? { genres } : {}),
      ...(publishDate ? { publish_date: publishDate } : {}),
      ...(openLibraryKey ? { open_library_key: openLibraryKey } : {}),
      ...(goodreadsURL ? { goodreads_url: goodreadsURL } : {}),
    };
  }

  function normalizeMovieMetadata(movie?: MovieMetadata): MovieCardMetadata | null {
    if (!movie) {
      return null;
    }

    const title = typeof movie.title === 'string' ? movie.title.trim() : '';
    if (!title) {
      return null;
    }

    const normalizedReleaseDate =
      (typeof movie.releaseDate === 'string' ? movie.releaseDate : undefined) ??
      (typeof movie.release_date === 'string' ? movie.release_date : undefined);
    const normalizedDirector = typeof movie.director === 'string' ? movie.director : undefined;
    const normalizedTrailerKey =
      (typeof movie.trailerKey === 'string' ? movie.trailerKey : undefined) ??
      (typeof movie.trailer_key === 'string' ? movie.trailer_key : undefined);
    const normalizedRating =
      typeof movie.tmdbRating === 'number'
        ? movie.tmdbRating
        : typeof movie.tmdb_rating === 'number'
          ? movie.tmdb_rating
          : undefined;
    const normalizedRottenTomatoesScore = normalizeMoviePercentScore(
      movie.rottenTomatoesScore,
      movie.rotten_tomatoes_score
    );
    const normalizedMetacriticScore = normalizeMoviePercentScore(
      movie.metacriticScore,
      movie.metacritic_score
    );
    const normalizedIMDBIDRaw =
      (typeof movie.imdbId === 'string' ? movie.imdbId : undefined) ??
      (typeof movie.imdb_id === 'string' ? movie.imdb_id : undefined);
    const normalizedIMDBID = normalizedIMDBIDRaw?.trim().toLowerCase();
    const imdbId = normalizedIMDBID && /^tt\d+$/.test(normalizedIMDBID) ? normalizedIMDBID : undefined;
    const normalizedRottenTomatoesURL =
      (typeof movie.rottenTomatoesUrl === 'string' ? movie.rottenTomatoesUrl : undefined) ??
      (typeof movie.rotten_tomatoes_url === 'string' ? movie.rotten_tomatoes_url : undefined);
    const rottenTomatoesUrl = normalizedRottenTomatoesURL?.trim();
    const normalizedRuntime =
      typeof movie.runtime === 'number' && Number.isFinite(movie.runtime) ? movie.runtime : undefined;
    const normalizedGenres =
      Array.isArray(movie.genres) && movie.genres.length > 0
        ? movie.genres.filter((genre): genre is string => typeof genre === 'string' && genre.trim().length > 0)
        : undefined;
    const normalizedCast =
      Array.isArray(movie.cast) && movie.cast.length > 0
        ? movie.cast
            .filter(
              (member): member is MovieCastMember =>
                !!member && typeof member.name === 'string' && member.name.trim().length > 0
            )
            .map((member) => ({
              name: member.name.trim(),
              character: typeof member.character === 'string' ? member.character : '',
            }))
        : undefined;
    const normalizedTMDBID =
      typeof movie.tmdbId === 'number'
        ? movie.tmdbId
        : typeof movie.tmdb_id === 'number'
          ? movie.tmdb_id
          : undefined;
    const rawTMDBMediaType =
      (typeof movie.tmdbMediaType === 'string' ? movie.tmdbMediaType : undefined) ??
      (typeof movie.tmdb_media_type === 'string' ? movie.tmdb_media_type : undefined);
    const normalizedTMDBMediaType =
      rawTMDBMediaType?.trim().toLowerCase() === 'movie'
        ? 'movie'
        : rawTMDBMediaType?.trim().toLowerCase() === 'tv' ||
            rawTMDBMediaType?.trim().toLowerCase() === 'series'
          ? 'tv'
          : undefined;
    const normalizedSeasons =
      Array.isArray(movie.seasons) && movie.seasons.length > 0
        ? movie.seasons
            .map((season): MovieCardSeason | null => {
              if (!season || typeof season !== 'object') {
                return null;
              }

              const seasonNumberValue =
                typeof season.seasonNumber === 'number'
                  ? season.seasonNumber
                  : typeof season.season_number === 'number'
                    ? season.season_number
                    : null;
              if (seasonNumberValue === null || !Number.isFinite(seasonNumberValue)) {
                return null;
              }

              const episodeCountValue =
                typeof season.episodeCount === 'number'
                  ? season.episodeCount
                  : typeof season.episode_count === 'number'
                    ? season.episode_count
                    : undefined;
              const airDateValue =
                (typeof season.airDate === 'string' ? season.airDate : undefined) ??
                (typeof season.air_date === 'string' ? season.air_date : undefined);
              const posterValue =
                (typeof season.poster === 'string' ? season.poster : undefined) ??
                (typeof season.poster_url === 'string' ? season.poster_url : undefined) ??
                (typeof season.posterUrl === 'string' ? season.posterUrl : undefined);

              return {
                seasonNumber: Math.trunc(seasonNumberValue),
                ...(typeof episodeCountValue === 'number' && Number.isFinite(episodeCountValue)
                  ? { episodeCount: Math.trunc(episodeCountValue) }
                  : {}),
                ...(airDateValue ? { airDate: airDateValue } : {}),
                ...(typeof season.name === 'string' ? { name: season.name } : {}),
                ...(typeof season.overview === 'string' ? { overview: season.overview } : {}),
                ...(posterValue ? { poster: posterValue } : {}),
              };
            })
            .filter((season): season is MovieCardSeason => season !== null)
            .sort((a, b) => a.seasonNumber - b.seasonNumber)
        : undefined;

    return {
      title,
      ...(typeof movie.overview === 'string' ? { overview: movie.overview } : {}),
      ...(typeof movie.poster === 'string' ? { poster: movie.poster } : {}),
      ...(typeof movie.backdrop === 'string' ? { backdrop: movie.backdrop } : {}),
      ...(typeof normalizedRuntime === 'number' ? { runtime: normalizedRuntime } : {}),
      ...(normalizedGenres ? { genres: normalizedGenres } : {}),
      ...(normalizedReleaseDate ? { releaseDate: normalizedReleaseDate } : {}),
      ...(normalizedCast ? { cast: normalizedCast } : {}),
      ...(normalizedDirector ? { director: normalizedDirector } : {}),
      ...(typeof normalizedRating === 'number' ? { tmdbRating: normalizedRating } : {}),
      ...(typeof normalizedRottenTomatoesScore === 'number'
        ? { rottenTomatoesScore: normalizedRottenTomatoesScore }
        : {}),
      ...(typeof normalizedMetacriticScore === 'number'
        ? { metacriticScore: normalizedMetacriticScore }
        : {}),
      ...(imdbId ? { imdbId } : {}),
      ...(rottenTomatoesUrl ? { rottenTomatoesUrl } : {}),
      ...(normalizedTrailerKey ? { trailerKey: normalizedTrailerKey } : {}),
      ...(typeof normalizedTMDBID === 'number' ? { tmdbId: normalizedTMDBID } : {}),
      ...(normalizedTMDBMediaType ? { tmdbMediaType: normalizedTMDBMediaType } : {}),
      ...(normalizedSeasons && normalizedSeasons.length > 0 ? { seasons: normalizedSeasons } : {}),
    };
  }

  function getMovieMetadataFromLink(link?: Link): MovieCardMetadata | null {
    if (!link?.metadata) {
      return null;
    }

    const metadataWithMovie = link.metadata as LinkMetadataWithMovie;
    const normalizedMovie = normalizeMovieMetadata(metadataWithMovie.movie);
    if (normalizedMovie) {
      return normalizedMovie;
    }

    if (metadataWithMovie.type !== 'movie' && metadataWithMovie.type !== 'series') {
      return null;
    }

    const fallbackTitle =
      typeof metadataWithMovie.title === 'string' ? metadataWithMovie.title.trim() : '';
    if (!fallbackTitle) {
      return null;
    }

    return {
      title: fallbackTitle,
      ...(typeof metadataWithMovie.description === 'string'
        ? { overview: metadataWithMovie.description }
        : {}),
      ...(typeof metadataWithMovie.image === 'string' ? { poster: metadataWithMovie.image } : {}),
    };
  }

  function getBookMetadataFromLink(link?: Link): BookMetadata | null {
    if (!link?.metadata) {
      return null;
    }

    const metadataWithBook = link.metadata as LinkMetadataWithBook;
    const rawBookData = metadataWithBook.book_data ?? metadataWithBook.bookData ?? metadataWithBook.book;
    return normalizeBookMetadata(rawBookData);
  }

  function normalizePodcastKind(kind: unknown): 'show' | 'episode' | null {
    if (typeof kind !== 'string') {
      return null;
    }

    const normalized = kind.trim().toLowerCase();
    if (normalized === 'show') {
      return 'show';
    }
    if (normalized === 'episode') {
      return 'episode';
    }
    return null;
  }

  function normalizePodcastHighlightEpisodes(
    episodes: unknown
  ): PodcastCardMetadata['highlightEpisodes'] {
    if (!Array.isArray(episodes)) {
      return [];
    }

    return episodes
      .map((episode) => {
        if (!episode || typeof episode !== 'object' || Array.isArray(episode)) {
          return null;
        }

        const record = episode as PodcastHighlightEpisodeMetadata;
        const title = typeof record.title === 'string' ? record.title.trim() : '';
        const url = typeof record.url === 'string' ? record.url.trim() : '';
        if (!title || !url) {
          return null;
        }

        const note = typeof record.note === 'string' ? record.note.trim() : '';
        return {
          title,
          url,
          ...(note ? { note } : {}),
        };
      })
      .filter((episode): episode is PodcastCardMetadata['highlightEpisodes'][number] => episode !== null);
  }

  function getPodcastMetadataFromLink(link?: Link): PodcastCardMetadata | null {
    if (!link?.metadata?.podcast) {
      return null;
    }

    const podcast = link.metadata.podcast as PodcastMetadata;
    const kind = normalizePodcastKind(podcast.kind);
    if (!kind) {
      return null;
    }

    const highlightEpisodes = normalizePodcastHighlightEpisodes(
      podcast.highlightEpisodes ?? podcast.highlight_episodes
    );

    return {
      kind,
      highlightEpisodes,
    };
  }

  $: postImages = (post.images ?? []).slice().sort((a, b) => a.position - b.position);
  $: hasPostImages = postImages.length > 0;
  $: imageLinks = (post.links ?? []).filter((item) => Boolean(getImageLinkUrl(item)));
  $: imageItems = hasPostImages
    ? postImages.map((item, index): ImageItem => ({
        id: item.id,
        url: item.url,
        title: item.caption || item.altText || `Image ${index + 1}`,
        altText: item.altText || item.caption || `Image ${index + 1}`,
      }))
    : imageLinks
        .map((item): ImageItem => ({
          link: item,
          url: getImageLinkUrl(item) ?? '',
          title: item.metadata?.title || 'Uploaded image',
          altText: item.metadata?.title || 'Uploaded image',
        }))
        .filter((item) => Boolean(item.url));
  $: primaryLink = post.links?.[0];
  $: primaryLinkIsImage = primaryLink ? Boolean(getImageLinkUrl(primaryLink)) : false;
  $: metadata = primaryLink?.metadata;
  $: primaryBookMetadata = getBookMetadataFromLink(primaryLink);
  $: primaryMovieMetadata = getMovieMetadataFromLink(primaryLink);
  $: primaryPodcastMetadata = isPodcastSection ? getPodcastMetadataFromLink(primaryLink) : null;
  $: podcastKindLabel =
    primaryPodcastMetadata?.kind === 'show'
      ? 'Show'
      : primaryPodcastMetadata?.kind === 'episode'
        ? 'Episode'
        : null;
  $: movieLinks = isMovieSection
    ? (post.links ?? []).filter((link) => Boolean(getMovieMetadataFromLink(link)))
    : [];
  $: secondaryMovieLinks = movieLinks.filter((link) => link !== primaryLink);
  $: bandcampEmbed =
    metadata?.embed &&
    (metadata.embed.provider ?? '').toLowerCase() === 'bandcamp' &&
    metadata.embed.embedUrl
      ? metadata.embed
      : undefined;
  $: soundCloudEmbed =
    soundCloudEmbedFromFrontend ??
    (metadata?.embed?.provider === 'soundcloud' && metadata.embed.embedUrl
      ? ({ ...metadata.embed, embedUrl: metadata.embed.embedUrl } as SoundCloudEmbedData)
      : undefined);
  $: spotifyEmbed = (() => {
    const url = primaryLink?.url;
    if (url && isSpotifyUrl(url)) {
      return parseSpotifyUrl(url);
    }
    if (metadata?.embed && isSpotifyEmbedUrl(metadata.embed.embedUrl)) {
      return metadata.embed;
    }
    return undefined;
  })();
  $: spotifyEmbedUrl =
    spotifyEmbed?.embedUrl ??
    (isSpotifyEmbedUrl(metadata?.embedUrl) ? metadata?.embedUrl : undefined);
  $: spotifyEmbedHeight = spotifyEmbed?.height;
  $: youtubeEmbedUrl = (() => {
    const url = primaryLink?.url;
    if (url && isYouTubeUrl(url)) {
      return parseYouTubeUrl(url);
    }
    return metadata?.embed?.provider === 'youtube' ? metadata.embed.embedUrl : undefined;
  })();
  $: highlightEmbedProvider = soundCloudEmbed
    ? 'soundcloud'
    : youtubeEmbedUrl
      ? 'youtube'
      : spotifyEmbedUrl
        ? 'spotify'
        : bandcampEmbed
          ? 'bandcamp'
          : metadata?.embed?.provider;
  $: highlightSeekMessage = getSeekUnavailableMessage(highlightEmbedProvider);
  $: canSeekTimestamps = !!embedController?.supportsSeeking;
  $: movieStatsForBar = isMovieSection
    ? {
        watchlistCount: movieStats?.watchlistCount ?? 0,
        watchCount: movieStats?.watchCount ?? 0,
        ...(typeof movieStats?.averageRating === 'number'
          ? { avgRating: movieStats.averageRating }
          : {}),
        viewerWatchlisted: movieStats?.viewerWatchlisted ?? false,
        viewerWatched: movieStats?.viewerWatched ?? false,
        ...(typeof movieStats?.viewerRating === 'number'
          ? { viewerRating: movieStats.viewerRating }
          : {}),
        ...(movieStats?.viewerCategories ? { viewerCategories: movieStats.viewerCategories } : {}),
      }
    : null;
  $: bookStatsForBar =
    isBookSection && bookStats
      ? {
          bookshelfCount: bookStats.bookshelfCount ?? 0,
          readCount: bookStats.readCount ?? 0,
          averageRating:
            typeof bookStats.averageRating === 'number' && Number.isFinite(bookStats.averageRating)
              ? bookStats.averageRating
              : null,
          viewerOnBookshelf: bookStats.viewerOnBookshelf ?? false,
          viewerCategories: bookStats.viewerCategories ?? [],
          viewerRead: bookStats.viewerRead ?? false,
          viewerRating:
            typeof bookStats.viewerRating === 'number' && Number.isFinite(bookStats.viewerRating)
              ? bookStats.viewerRating
              : null,
        }
      : null;
  $: {
    if (embedController && (!highlightEmbedProvider || embedController.provider !== highlightEmbedProvider)) {
      embedController = null;
    }
  }
  $: primaryImageUrl = imageItems.length > 0 ? imageItems[0].url : undefined;
  $: isInternalUploadLink =
    !hasPostImages && imageItems.length > 0 && imageItems[0].link
      ? isInternalUploadUrl(imageItems[0].link?.url ?? '')
      : false;
  $: displayContent =
    !isEditing && primaryImageUrl && isInternalUploadLink
      ? stripInternalUploadUrls(post.content)
      : post.content;
  $: canEdit = $currentUser?.id === post.userId;
  $: canDelete = $currentUser?.id === post.userId || $isAdmin;
  $: originalImageUrls = imageLinks.map((link) => getImageLinkUrl(link));
  $: editImagePreviewUrls = editImages.map((state, index) => {
    const originalUrl = originalImageUrls[index];
    if (state?.action === 'replace' && state.uploadUrl) {
      return state.uploadUrl;
    }
    if (state?.action === 'keep') {
      return originalUrl;
    }
    return undefined;
  });
  $: isEditImageUploading = editImages.some((item) => item.uploading);
  $: activeImageItem = imageItems[activeImageIndex];
  $: activeImageUrl = activeImageItem?.url;
  $: activeImageLink = activeImageItem?.link;
  $: activeImageTitle = activeImageItem?.title ?? 'Uploaded image';
  $: activeImageAlt = activeImageItem?.altText ?? activeImageTitle;
  $: activeImageFailed = imageLoadFailures.has(activeImageIndex);
  $: isActiveImageInternal = activeImageLink
    ? isInternalUploadUrl(activeImageLink.url)
    : false;
  $: lightboxImageItem = imageItems[lightboxImageIndex];
  $: lightboxImageUrl = lightboxImageItem?.url;
  $: lightboxAltText =
    lightboxImageItem?.altText ?? lightboxImageItem?.title ?? 'Full size image';
  $: if (imageReplyTarget) {
    const target = imageReplyTarget;
    const stillExists = imageItems.find((item, index) =>
      item.id && target.id
        ? item.id === target.id
        : item.url === target.url && index === target.index
    );
    if (!stillExists) {
      imageReplyTarget = null;
    }
  }

  let activeImageIndex = 0;
  let imageLoadFailures = new Set<number>();
  let lastImageSignature = '';

  $: {
    const signature = imageItems.map((item) => item.url).join('|');
    if (signature !== lastImageSignature) {
      activeImageIndex = 0;
      lightboxImageIndex = 0;
      lightboxPreloadedIndices = new Set();
      imageLoadFailures = new Set();
      lastImageSignature = signature;
    }
  }

  function markImageFailed(index: number) {
    if (!imageLoadFailures.has(index)) {
      const next = new Set(imageLoadFailures);
      next.add(index);
      imageLoadFailures = next;
    }
  }

  function goToImage(index: number) {
    if (imageItems.length === 0) {
      return;
    }
    const clamped = (index + imageItems.length) % imageItems.length;
    activeImageIndex = clamped;
  }

  function nextImage() {
    goToImage(activeImageIndex + 1);
  }

  function previousImage() {
    goToImage(activeImageIndex - 1);
  }

  function handleCarouselKeydown(event: KeyboardEvent) {
    if (imageItems.length <= 1) {
      return;
    }
    if (event.key === 'ArrowLeft') {
      event.preventDefault();
      previousImage();
    }
    if (event.key === 'ArrowRight') {
      event.preventDefault();
      nextImage();
    }
  }

  let touchStartX = 0;
  let touchStartY = 0;
  let touchActive = false;
  let lightboxTouchStartX = 0;
  let lightboxTouchStartY = 0;
  let lightboxTouchActive = false;
  let lightboxPreloadedIndices = new Set<number>();

  function handleTouchStart(event: TouchEvent) {
    if (imageItems.length <= 1) {
      return;
    }
    const touch = event.touches[0];
    if (!touch) {
      return;
    }
    touchStartX = touch.clientX;
    touchStartY = touch.clientY;
    touchActive = true;
  }

  function handleTouchEnd(event: TouchEvent) {
    if (!touchActive || imageItems.length <= 1) {
      touchActive = false;
      return;
    }
    const touch = event.changedTouches[0];
    if (!touch) {
      touchActive = false;
      return;
    }
    const deltaX = touch.clientX - touchStartX;
    const deltaY = touch.clientY - touchStartY;
    touchActive = false;
    if (Math.abs(deltaX) > 40 && Math.abs(deltaX) > Math.abs(deltaY) * 1.5) {
      if (deltaX < 0) {
        nextImage();
      } else {
        previousImage();
      }
    }
  }

  function preloadLightboxImage(index: number) {
    if (lightboxPreloadedIndices.has(index)) {
      return;
    }
    const url = imageItems[index]?.url;
    if (!url) {
      return;
    }
    const img = new Image();
    img.src = url;
    const next = new Set(lightboxPreloadedIndices);
    next.add(index);
    lightboxPreloadedIndices = next;
  }

  function preloadLightboxAdjacent(index: number) {
    if (imageItems.length <= 1) {
      preloadLightboxImage(index);
      return;
    }
    preloadLightboxImage(index);
    preloadLightboxImage((index + 1) % imageItems.length);
    preloadLightboxImage((index - 1 + imageItems.length) % imageItems.length);
  }

  let editImageLoadFailures = new Set<number>();
  let lastEditPreviewUrls: Array<string | undefined> = [];
  $: {
    const nextFailures = new Set<number>();
    editImageLoadFailures.forEach((index) => {
      if (index >= editImagePreviewUrls.length) {
        return;
      }
      if (editImagePreviewUrls[index] === lastEditPreviewUrls[index]) {
        nextFailures.add(index);
      }
    });
    editImageLoadFailures = nextFailures;
    lastEditPreviewUrls = [...editImagePreviewUrls];
  }

  function validateImageFile(file: File): string | null {
    if (file.type && !ALLOWED_IMAGE_MIME_TYPES.includes(file.type)) {
      return 'Only image files are supported.';
    }
    if (
      !file.type &&
      !new RegExp(`\\.(${ALLOWED_IMAGE_EXTENSIONS.join('|')})$`, 'i').test(file.name)
    ) {
      return 'Only image files are supported.';
    }
    if (file.size > MAX_UPLOAD_BYTES) {
      return `Images must be ${MAX_UPLOAD_LABEL} or smaller.`;
    }
    return null;
  }

  function updateEditImage(index: number, patch: Partial<EditImageState>) {
    editImages = editImages.map((item, i) => (i === index ? { ...item, ...patch } : item));
  }

  async function handleEditImageSelect(index: number, event: Event) {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) {
      return;
    }

    const validationError = validateImageFile(file);
    if (validationError) {
      updateEditImage(index, { uploadError: validationError });
      input.value = '';
      return;
    }

    updateEditImage(index, { uploading: true, progress: 0, uploadError: null });

    try {
      const response = await api.uploadImage(file, (progress) => {
        updateEditImage(index, { progress });
      });
      updateEditImage(index, { uploadUrl: response.url, action: 'replace' });
    } catch (err) {
      updateEditImage(index, {
        uploadError: err instanceof Error ? err.message : 'Upload failed',
      });
    } finally {
      updateEditImage(index, { uploading: false });
      input.value = '';
    }
  }

  function removeEditImage(index: number) {
    updateEditImage(index, {
      action: 'remove',
      uploadUrl: null,
      uploadError: null,
      uploading: false,
      progress: 0,
    });
  }

  function undoEditImageRemoval(index: number) {
    updateEditImage(index, {
      action: 'keep',
      uploadUrl: null,
      uploadError: null,
      uploading: false,
      progress: 0,
    });
  }

  function buildEditLinksPayload():
    | { url: string; highlights?: { timestamp: number; label?: string }[] }[]
    | null
    | undefined {
    const originalLinks = post.links ?? [];
    const hasImageChanges = editImages.some((item) => item.action !== 'keep');
    const hasLinkChanges = editLinkAction !== 'keep';
    const replacementUrl = editLinkUrl.trim();

    if (!hasImageChanges && !hasLinkChanges) {
      return undefined;
    }

    if (!hasImageChanges && editLinkAction === 'remove') {
      return undefined;
    }

    if (editLinkAction === 'replace' && !replacementUrl) {
      return undefined;
    }

    if (originalLinks.length === 0 && editLinkAction !== 'replace') {
      return undefined;
    }

    const imageIndexByLinkIndex = new Map<number, number>();
    const imageLinkIndices = originalLinks.reduce<number[]>((indices, item, index) => {
      if (getImageLinkUrl(item)) {
        indices.push(index);
      }
      return indices;
    }, []);
    if (hasImageChanges && imageLinkIndices.length === 0) {
      return undefined;
    }
    imageLinkIndices.forEach((linkIndex, imageIndex) => {
      imageIndexByLinkIndex.set(linkIndex, imageIndex);
    });

    const primaryNonImageIndex = originalLinks.findIndex((item) => !getImageLinkUrl(item));
    const nextLinks: { url: string; highlights?: { timestamp: number; label?: string }[] }[] = [];
    originalLinks.forEach((item, linkIndex) => {
      const isPrimaryNonImage = linkIndex === primaryNonImageIndex;
      if (isPrimaryNonImage) {
        if (editLinkAction === 'remove') {
          return;
        }
        if (editLinkAction === 'replace' && replacementUrl) {
          nextLinks.push({ url: replacementUrl });
          return;
        }
      }

      const baseLink = {
        url: item.url,
        ...(item.highlights && item.highlights.length > 0 ? { highlights: item.highlights } : {}),
      };
      const imageIndex = imageIndexByLinkIndex.get(linkIndex);
      if (imageIndex === undefined) {
        nextLinks.push(baseLink);
        return;
      }
      const editState = editImages[imageIndex];
      if (!editState || editState.action === 'keep') {
        nextLinks.push(baseLink);
        return;
      }
      if (editState.action === 'remove') {
        return;
      }
      if (editState.action === 'replace' && editState.uploadUrl) {
        nextLinks.push({ url: editState.uploadUrl });
        return;
      }
      nextLinks.push({ url: item.url });
    });

    if (editLinkAction === 'replace' && primaryNonImageIndex === -1 && replacementUrl) {
      nextLinks.unshift({ url: replacementUrl });
    }

    return nextLinks;
  }

  async function deletePost() {
    if (typeof window !== 'undefined') {
      const confirmed = window.confirm('Delete this post?');
      if (!confirmed) {
        return;
      }
    }

    isDeleting = true;
    deleteError = null;

    try {
      await api.deletePost(post.id);
      postStore.removePost(post.id);
    } catch (err) {
      deleteError = err instanceof Error ? err.message : 'Failed to delete post';
      logError('Failed to delete post', { postId: post.id }, err);
    } finally {
      isDeleting = false;
    }
  }

  const renderStart = typeof performance !== 'undefined' ? performance.now() : null;
  onMount(() => {
    if (renderStart === null) {
      return;
    }
    recordComponentRender('PostCard', performance.now() - renderStart);
  });
</script>

<svelte:window on:keydown={handleLightboxKeydown} />

<article class="bg-white rounded-lg shadow-sm border border-gray-200 p-4 hover:shadow-md transition-shadow">
  {#if showSectionPill && sectionInfo}
    <div class="mb-3 flex flex-wrap items-center gap-2">
      {#if showSectionLabel}
        <span class="text-sm font-medium text-gray-500">Posted in:</span>
      {/if}
      <span class="inline-flex items-center gap-1.5 rounded-full border border-gray-200 bg-gray-100 px-3 py-1 text-sm font-semibold text-gray-600">
        {#if sectionInfo.icon}
          <span class="text-base leading-none" aria-hidden="true">{sectionInfo.icon}</span>
        {/if}
        <span class="truncate">{sectionInfo.name}</span>
      </span>
    </div>
  {/if}
  <div class="flex items-start gap-3">
    {#if post.user?.id}
      <a
        href={buildProfileHref(post.user.id)}
        on:click={(event) => handleProfileNavigation(event, post.user?.id)}
        class="flex-shrink-0"
        aria-label={`View ${(post.user?.username ?? 'user')}'s profile`}
      >
        {#if post.user?.profilePictureUrl}
          <img
            src={post.user.profilePictureUrl}
            alt={post.user.username}
            class="w-10 h-10 rounded-full object-cover"
          />
        {:else}
          <div class="w-10 h-10 rounded-full bg-gray-200 flex items-center justify-center">
            <span class="text-gray-500 text-sm font-medium">
              {post.user?.username?.charAt(0).toUpperCase() || '?'}
            </span>
          </div>
        {/if}
      </a>
    {:else}
      {#if post.user?.profilePictureUrl}
        <img
          src={post.user?.profilePictureUrl}
          alt={post.user?.username}
          class="w-10 h-10 rounded-full object-cover flex-shrink-0"
        />
      {:else}
        <div class="w-10 h-10 rounded-full bg-gray-200 flex items-center justify-center flex-shrink-0">
          <span class="text-gray-500 text-sm font-medium">
            {post.user?.username?.charAt(0).toUpperCase() || '?'}
          </span>
        </div>
      {/if}
    {/if}

    <div class="flex-1 min-w-0">
      <div class="flex flex-wrap items-center gap-2 mb-1">
        {#if post.user?.id}
          <a
            href={buildProfileHref(post.user.id)}
            class="font-medium text-gray-900 truncate hover:underline"
            on:click={(event) => handleProfileNavigation(event, post.user?.id)}
          >
            {post.user?.username || 'Unknown'}
          </a>
        {:else}
          <span class="font-medium text-gray-900 truncate">
            {post.user?.username || 'Unknown'}
          </span>
        {/if}
        <span class="text-gray-400 text-sm">·</span>
        <RelativeTime dateString={post.createdAt} className="text-gray-500 text-sm" />
        <EditedBadge createdAt={post.createdAt} updatedAt={post.updatedAt} />
        <div class="ml-auto flex items-center gap-2 relative">
          {#if canEdit}
            <button
              type="button"
              class="inline-flex items-center gap-1 rounded-md border border-gray-200 px-2.5 py-1 text-xs font-medium text-gray-600 hover:text-gray-800 hover:bg-gray-50"
              on:click={startEdit}
            >
              <svg class="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                <path
                  d="M4 13.5V16h2.5l7.35-7.35-2.5-2.5L4 13.5zM16.85 5.65a.5.5 0 000-.7l-1.8-1.8a.5.5 0 00-.7 0l-1.6 1.6 2.5 2.5 1.6-1.6z"
                />
              </svg>
              <span>Edit</span>
            </button>
          {/if}
          {#if canDelete}
            <button
              type="button"
              class="inline-flex items-center gap-1 rounded-md border border-red-200 px-2.5 py-1 text-xs font-medium text-red-600 hover:text-red-700 hover:bg-red-50 disabled:opacity-60"
              on:click={deletePost}
              disabled={isDeleting}
            >
              <svg class="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                <path
                  d="M6 7a1 1 0 011 1v6a1 1 0 11-2 0V8a1 1 0 011-1zm4 0a1 1 0 011 1v6a1 1 0 11-2 0V8a1 1 0 011-1zm-1-5a1 1 0 00-1 1v1H5a1 1 0 000 2h10a1 1 0 100-2h-3V3a1 1 0 00-1-1H9z"
                />
              </svg>
              <span>{isDeleting ? 'Deleting...' : 'Delete'}</span>
            </button>
          {/if}
          <button
            type="button"
            class="inline-flex items-center gap-1 rounded-md border border-gray-200 px-2.5 py-1 text-xs font-medium text-gray-600 hover:text-gray-800 hover:bg-gray-50"
            on:click={copyThreadLink}
          >
            <svg class="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
              <path
                d="M7 5a3 3 0 013-3h4a3 3 0 013 3v4a3 3 0 01-3 3h-1a1 1 0 110-2h1a1 1 0 001-1V5a1 1 0 00-1-1h-4a1 1 0 00-1 1v1a1 1 0 11-2 0V5z"
              />
              <path
                d="M3 8a3 3 0 013-3h4a3 3 0 013 3v4a3 3 0 01-3 3H6a3 3 0 01-3-3V8zm3-1a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H6z"
              />
            </svg>
            <span>Share</span>
          </button>
          {#if copiedLink}
            <span
              class="absolute -top-6 right-0 rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] text-emerald-700 shadow"
              role="status"
              aria-live="polite"
            >
              Link copied
            </span>
          {/if}
        </div>
      </div>

      {#if deleteError}
        <div class="mb-2 text-xs text-red-600">{deleteError}</div>
      {/if}

      {#if isEditing}
        <div class="mb-3 space-y-4">
          {#if editImages.length > 0}
            <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 space-y-4">
              <div class="text-xs font-semibold uppercase tracking-wide text-gray-500">
                Images
              </div>
              {#each editImages as editImage, index}
                <div class="rounded-lg border border-gray-200 bg-white p-3 space-y-3">
                  <div class="text-xs font-semibold uppercase tracking-wide text-gray-500">
                    Image {index + 1}
                  </div>
                  {#if editImagePreviewUrls[index]}
                    <div class="rounded-lg border border-gray-200 overflow-hidden bg-white">
                      {#if editImageLoadFailures.has(index)}
                        <div class="flex items-center justify-center px-4 py-6 text-sm text-gray-500">
                          Image unavailable. Try uploading again.
                        </div>
                      {:else}
                        <img
                          src={editImagePreviewUrls[index]}
                          alt={`Post preview ${index + 1}`}
                          class="w-full max-h-[24rem] object-contain bg-white"
                          loading="lazy"
                          on:error={() => {
                            const next = new Set(editImageLoadFailures);
                            next.add(index);
                            editImageLoadFailures = next;
                          }}
                        />
                      {/if}
                    </div>
                  {/if}
                  <input
                    type="file"
                    bind:this={editImageInputs[index]}
                    on:change={(event) => handleEditImageSelect(index, event)}
                    accept={ACCEPTED_IMAGE_TYPES}
                    aria-label={`Upload replacement for image ${index + 1}`}
                    data-testid={`edit-image-input-${index}`}
                    class="hidden"
                  />
                  <div class="flex flex-wrap items-center gap-2">
                    <button
                      type="button"
                      class="w-full sm:w-auto px-2.5 py-1.5 rounded-md border border-gray-300 text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-60"
                      on:click={() => editImageInputs[index]?.click()}
                      disabled={isSaving || editImage.uploading}
                      aria-label={`Replace image ${index + 1}`}
                    >
                      Replace image
                    </button>
                    {#if editImage.action !== 'remove'}
                      <button
                        type="button"
                        class="w-full sm:w-auto px-2.5 py-1.5 rounded-md border border-red-200 text-sm text-red-600 hover:bg-red-50 disabled:opacity-60"
                        on:click={() => removeEditImage(index)}
                        disabled={isSaving || editImage.uploading}
                        aria-label={`Remove image ${index + 1}`}
                      >
                        Remove image
                      </button>
                    {:else}
                      <button
                        type="button"
                        class="w-full sm:w-auto px-2.5 py-1.5 rounded-md border border-gray-300 text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-60"
                        on:click={() => undoEditImageRemoval(index)}
                        disabled={isSaving || editImage.uploading}
                        aria-label={`Keep image ${index + 1}`}
                      >
                        Keep image
                      </button>
                    {/if}
                  </div>
                  {#if editImage.uploading}
                    <div class="text-xs text-gray-500">
                      Uploading image... {editImage.progress}%
                    </div>
                    <div class="h-1 w-full bg-gray-200 rounded">
                      <div
                        class="h-1 bg-blue-600 rounded"
                        style={`width: ${editImage.progress}%`}
                      ></div>
                    </div>
                  {/if}
                  {#if editImage.uploadError}
                    <div class="text-sm text-red-600">{editImage.uploadError}</div>
                  {/if}
                  {#if editImage.action === 'remove'}
                    <div class="text-xs text-amber-700 bg-amber-50 border border-amber-200 rounded px-2 py-1">
                      Image will be removed when you save.
                    </div>
                  {:else if editImage.action === 'replace'}
                    <div class="text-xs text-blue-700 bg-blue-50 border border-blue-200 rounded px-2 py-1">
                      New image will replace the existing one when you save.
                    </div>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}
          <MentionTextarea
            id={`edit-post-${post.id}`}
            name={`edit-post-${post.id}`}
            bind:value={editContent}
            bind:mentionUsernames={editMentionUsernames}
            on:keydown={(event) => handleEditKeyDown(event.detail)}
            rows={4}
            ariaLabel="Edit post content"
            className="w-full rounded-lg border border-gray-300 p-2 text-sm text-gray-800 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
          />
          {#if editLinkMetadata}
            <LinkPreview metadata={editLinkMetadata} onRemove={removeEditLinkPreview} />
          {:else if isEditLinkLoading}
            <div class="flex items-center gap-2 p-3 bg-gray-50 border border-gray-200 rounded-lg">
              <svg class="w-5 h-5 text-gray-400 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
              <span class="text-sm text-gray-500">Loading link preview...</span>
            </div>
          {:else if editLinkPreviewError}
            <div class="flex items-center justify-between p-3 bg-red-50 border border-red-200 rounded-lg">
              <span class="text-sm text-red-600">{editLinkPreviewError}</span>
              <button
                type="button"
                on:click={() => (editLinkPreviewError = null)}
                class="text-sm text-red-600 hover:text-red-800 font-medium"
              >
                Dismiss
              </button>
            </div>
          {/if}
          {#if editLinkAction === 'remove'}
            <div class="text-xs text-amber-700 bg-amber-50 border border-amber-200 rounded px-2 py-1">
              Link preview will be removed when you save.
            </div>
          {/if}
          {#if !editLinkMetadata && !isEditLinkInputVisible}
            <button
              type="button"
              class="w-full sm:w-auto px-2.5 py-1.5 rounded-md border border-gray-300 text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-60"
              on:click={openEditLinkInput}
              disabled={isSaving || isEditLinkLoading}
            >
              Add link preview
            </button>
          {/if}
          {#if isEditLinkInputVisible}
            <div class="space-y-2">
              <div class="flex flex-col sm:flex-row gap-2">
                <div class="flex-1">
                  <label for={`edit-post-link-${post.id}`} class="sr-only">Link URL</label>
                  <input
                    id={`edit-post-link-${post.id}`}
                    type="url"
                    bind:value={editLinkInputValue}
                    on:keydown={handleEditLinkInputKeydown}
                    placeholder="Paste a link (https://...)"
                    disabled={isSaving || isEditLinkLoading}
                    class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary focus:border-transparent disabled:opacity-50 disabled:bg-gray-100"
                  />
                </div>
                <button
                  type="button"
                  on:click={submitEditLinkInput}
                  disabled={isSaving || isEditLinkLoading}
                  class="px-3 py-2 bg-primary text-white font-medium rounded-lg hover:bg-secondary transition-colors disabled:opacity-50"
                >
                  Add link
                </button>
                <button
                  type="button"
                  on:click={closeEditLinkInput}
                  disabled={isSaving || isEditLinkLoading}
                  class="px-3 py-2 border border-gray-200 text-gray-600 font-medium rounded-lg hover:bg-gray-50 transition-colors disabled:opacity-50"
                >
                  Cancel
                </button>
              </div>
              {#if editLinkInputError}
                <p class="text-xs text-red-600">{editLinkInputError}</p>
              {:else}
                <p class="text-xs text-gray-500">We’ll fetch a preview after you add the link.</p>
              {/if}
            </div>
          {/if}
          {#if editError}
            <div class="text-sm text-red-600">{editError}</div>
          {/if}
          <div class="flex items-center gap-2">
            <button
              type="button"
              class="px-3 py-1.5 rounded-md bg-blue-600 text-white text-sm hover:bg-blue-700 disabled:opacity-60"
              on:click={saveEdit}
              disabled={isSaving || isEditImageUploading}
            >
              {isSaving ? 'Saving...' : 'Save'}
            </button>
            <button
              type="button"
              class="px-3 py-1.5 rounded-md border border-gray-300 text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-60"
              on:click={cancelEdit}
              disabled={isSaving || isEditImageUploading}
            >
              Cancel
            </button>
          </div>
        </div>
      {:else}
        <LinkifiedText
          text={displayContent}
          highlightQuery={highlightQuery}
          className="text-gray-800 whitespace-pre-wrap break-words mb-3"
        />
      {/if}

      {#if !isEditing && imageItems.length > 0}
        {#if imageItems.length === 1}
          <div
            id={`post-images-${post.id}`}
            class="relative mb-3 rounded-lg border border-gray-200 overflow-hidden bg-gray-50"
          >
            {#if activeImageFailed}
              <div class="flex items-center justify-center px-4 py-6 text-sm text-gray-500">
                Image unavailable. Try opening the link directly.
              </div>
            {:else if activeImageUrl}
              <button
                type="button"
                class="w-full text-left"
                aria-label="Open full-size image"
                aria-haspopup="dialog"
                on:click={() => openImageLightbox(0)}
              >
                <img
                  src={activeImageUrl}
                  alt={activeImageAlt}
                  class="w-full max-h-[28rem] object-contain bg-white"
                  loading="lazy"
                  on:error={() => {
                    markImageFailed(0);
                  }}
                />
              </button>
            {/if}
            {#if activeImageItem}
              <button
                type="button"
                class="absolute bottom-3 right-3 inline-flex items-center gap-1 rounded-full border border-blue-200 bg-white/95 px-3 py-1 text-xs text-blue-700 shadow-sm hover:bg-white"
                on:click|stopPropagation={() => startImageReply(0)}
              >
                Reply to image
              </button>
            {/if}
          </div>
        {:else}
          <div id={`post-images-${post.id}`} class="mb-3">
            <div class="relative rounded-lg border border-gray-200 overflow-hidden bg-gray-50">
              <!-- svelte-ignore a11y-no-noninteractive-tabindex a11y-no-noninteractive-element-interactions -->
              <div
                class="relative"
                tabindex="0"
                role="region"
                aria-roledescription="carousel"
                aria-label="Post images"
                on:keydown={handleCarouselKeydown}
                on:touchstart={handleTouchStart}
                on:touchend={handleTouchEnd}
                on:touchcancel={() => {
                  touchActive = false;
                }}
                style="touch-action: pan-y;"
              >
                <div
                  class="flex transition-transform duration-300 ease-out"
                  style={`transform: translateX(-${activeImageIndex * 100}%);`}
                >
                  {#each imageItems as item, index}
                    <div class="relative w-full flex-shrink-0">
                      {#if imageLoadFailures.has(index)}
                        <div class="flex items-center justify-center px-4 py-6 text-sm text-gray-500">
                          Image unavailable. Try opening the link directly.
                        </div>
                      {:else}
                        <button
                          type="button"
                          class="w-full text-left"
                          aria-label={`Open image ${index + 1} in full size`}
                          aria-haspopup="dialog"
                          on:click={() => openImageLightbox(index)}
                        >
                          <img
                            src={item.url}
                            alt={item.altText ?? item.title}
                            class="w-full max-h-[28rem] object-contain bg-white"
                            loading={index === activeImageIndex ? 'eager' : 'lazy'}
                            on:error={() => {
                              markImageFailed(index);
                            }}
                          />
                        </button>
                      {/if}
                      {#if item}
                        <button
                          type="button"
                          class="absolute bottom-3 right-3 z-10 inline-flex items-center gap-1 rounded-full border border-blue-200 bg-white/95 px-3 py-1 text-xs text-blue-700 shadow-sm hover:bg-white"
                          on:click|stopPropagation={() => startImageReply(index)}
                        >
                          Reply to image
                        </button>
                      {/if}
                    </div>
                  {/each}
                </div>

                <div class="absolute inset-y-0 left-2 hidden sm:flex items-center pointer-events-none">
                  <button
                    type="button"
                    class="flex h-9 w-9 items-center justify-center rounded-full bg-white/80 text-gray-700 shadow hover:bg-white pointer-events-auto"
                    aria-label="Previous image"
                    on:click={previousImage}
                  >
                    ‹
                  </button>
                </div>
                <div class="absolute inset-y-0 right-2 hidden sm:flex items-center pointer-events-none">
                  <button
                    type="button"
                    class="flex h-9 w-9 items-center justify-center rounded-full bg-white/80 text-gray-700 shadow hover:bg-white pointer-events-auto"
                    aria-label="Next image"
                    on:click={nextImage}
                  >
                    ›
                  </button>
                </div>

                <div class="absolute bottom-2 left-0 right-0 flex items-center justify-center">
                  <span class="rounded-full bg-black/60 px-2 py-1 text-xs text-white">
                    {activeImageIndex + 1}/{imageItems.length}
                  </span>
                </div>
              </div>
            </div>
            <div class="mt-2 flex items-center justify-center gap-1">
              {#each imageItems as _, index}
                <button
                  type="button"
                  class={`h-2.5 w-2.5 rounded-full transition-colors ${
                    index === activeImageIndex ? 'bg-gray-900' : 'bg-gray-300'
                  }`}
                  aria-label={`Go to image ${index + 1} of ${imageItems.length}`}
                  aria-current={index === activeImageIndex ? 'true' : 'false'}
                  on:click={() => goToImage(index)}
                ></button>
              {/each}
            </div>
          </div>
        {/if}
        {#if activeImageLink && (!isActiveImageInternal || activeImageFailed)}
          <a
            href={activeImageLink.url}
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 text-sm break-all"
          >
            <span>🔗</span>
            <span class="underline">{activeImageLink.url}</span>
          </a>
        {/if}
        {#if primaryLink && soundCloudEmbed}
          <SoundCloudEmbed
            embedUrl={soundCloudEmbed.embedUrl}
            height={soundCloudEmbed.height}
            title={metadata?.title}
            onReady={handleEmbedReady}
          />
        {:else if primaryLink && bandcampEmbed}
          <BandcampEmbed embed={bandcampEmbed} linkUrl={primaryLink.url} title={metadata?.title} />
        {:else if primaryLink && isBookSection && primaryBookMetadata}
          <BookCard
            bookData={primaryBookMetadata}
            compact={!isThreadView}
            threadHref={buildThreadHref(sectionSlug, post.id)}
          />
        {:else if primaryLink && isMovieSection && primaryMovieMetadata}
          <MovieCard movie={primaryMovieMetadata} />
        {:else if primaryLink && metadata?.recipe}
          <RecipeCard
            recipe={metadata.recipe}
            fallbackImage={metadata.image}
            fallbackTitle={metadata.title}
          />
        {:else if primaryLink && metadata?.embed?.provider === 'youtube' && metadata.embed.embedUrl}
          <div class="mt-3">
            <YouTubeEmbed
              embedUrl={metadata.embed.embedUrl}
              title={metadata.title || 'YouTube video'}
              onReady={handleEmbedReady}
            />
          </div>
        {:else if primaryLink && metadata && !primaryLinkIsImage && (!isBookSection || primaryBookMetadata)}
          {#if spotifyEmbedUrl}
            <div class="mt-3">
              <SpotifyEmbed embedUrl={spotifyEmbedUrl} height={spotifyEmbedHeight} />
            </div>
          {:else}
            <a
              href={primaryLink.url}
              target="_blank"
              rel="noopener noreferrer"
              class="mt-3 block rounded-lg border border-gray-200 overflow-hidden hover:border-gray-300 transition-colors"
            >
              <div class="flex">
                {#if metadata.image}
                  <div class="w-24 h-24 flex-shrink-0">
                    <img
                      src={metadata.image}
                      alt={metadata.title || 'Link preview'}
                      class="w-full h-full object-cover"
                    />
                  </div>
                {/if}
                <div class="flex-1 p-3 min-w-0">
                  <div class="flex items-center gap-1 mb-1">
                    <span>{getProviderIcon(metadata.provider)}</span>
                    {#if metadata.provider}
                      <span class="text-xs text-gray-500 capitalize">{metadata.provider}</span>
                    {/if}
                  </div>
                  {#if metadata.title}
                    <h4 class="font-medium text-gray-900 text-sm truncate">
                      {metadata.title}
                    </h4>
                  {/if}
                  {#if metadata.description}
                    <p class="text-gray-600 text-xs line-clamp-2 mt-0.5">
                      {metadata.description}
                    </p>
                  {/if}
                  {#if metadata.author}
                    <p class="text-gray-500 text-xs mt-1">
                      by {metadata.author}
                    </p>
                  {/if}
                </div>
              </div>
            </a>
          {/if}
        {:else if primaryLink && isBookSection && !primaryBookMetadata}
          <a
            href={primaryLink.url}
            target="_blank"
            rel="noopener noreferrer"
            class="mt-3 inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 text-sm break-all"
          >
            <span>🔗</span>
            <span class="underline">{primaryLink.url}</span>
          </a>
        {/if}
      {:else if !isEditing && primaryLink && metadata && (!isBookSection || primaryBookMetadata)}
        {#if soundCloudEmbed}
          <SoundCloudEmbed
            embedUrl={soundCloudEmbed.embedUrl}
            height={soundCloudEmbed.height}
            title={metadata?.title}
            onReady={handleEmbedReady}
          />
        {:else if bandcampEmbed}
          <BandcampEmbed embed={bandcampEmbed} linkUrl={primaryLink.url} title={metadata?.title} />
        {:else if isBookSection && primaryBookMetadata}
          <BookCard
            bookData={primaryBookMetadata}
            compact={!isThreadView}
            threadHref={buildThreadHref(sectionSlug, post.id)}
          />
        {:else if isMovieSection && primaryMovieMetadata}
          <MovieCard movie={primaryMovieMetadata} />
        {:else if metadata.recipe}
          <RecipeCard
            recipe={metadata.recipe}
            fallbackImage={metadata.image}
            fallbackTitle={metadata.title}
          />
        {:else if spotifyEmbedUrl}
          <SpotifyEmbed embedUrl={spotifyEmbedUrl} height={spotifyEmbedHeight} />
        {:else if youtubeEmbedUrl}
          <div class="mt-3">
            <YouTubeEmbed
              embedUrl={youtubeEmbedUrl}
              title={metadata?.title || 'YouTube video'}
              onReady={handleEmbedReady}
            />
          </div>
        {:else}
          <a
            href={primaryLink.url}
            target="_blank"
            rel="noopener noreferrer"
            class="block rounded-lg border border-gray-200 overflow-hidden hover:border-gray-300 transition-colors"
          >
            <div class="flex">
              {#if metadata.image}
                <div class="w-24 h-24 flex-shrink-0">
                  <img
                    src={metadata.image}
                    alt={metadata.title || 'Link preview'}
                    class="w-full h-full object-cover"
                  />
                </div>
              {/if}
              <div class="flex-1 p-3 min-w-0">
                <div class="flex items-center gap-1 mb-1">
                  <span>{getProviderIcon(metadata.provider)}</span>
                  {#if metadata.provider}
                    <span class="text-xs text-gray-500 capitalize">{metadata.provider}</span>
                  {/if}
                </div>
                {#if metadata.title}
                  <h4 class="font-medium text-gray-900 text-sm truncate">
                    {metadata.title}
                  </h4>
                {/if}
                {#if metadata.description}
                  <p class="text-gray-600 text-xs line-clamp-2 mt-0.5">
                    {metadata.description}
                  </p>
                {/if}
                {#if metadata.author}
                  <p class="text-gray-500 text-xs mt-1">
                    by {metadata.author}
                  </p>
                {/if}
              </div>
            </div>
          </a>
        {/if}
      {:else if !isEditing && primaryLink}
        <a
          href={primaryLink.url}
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 text-sm break-all"
        >
          <span>🔗</span>
          <span class="underline">{primaryLink.url}</span>
        </a>
      {/if}

      {#if isPodcastSection && primaryPodcastMetadata && podcastKindLabel}
        <div
          class="mt-3 rounded-lg border border-amber-200 bg-amber-50/70 p-3"
          data-testid="podcast-metadata-block"
        >
          <div class="flex items-center justify-between gap-2">
            <span class="text-xs font-medium uppercase tracking-wide text-amber-700">Podcast</span>
            <span
              class="inline-flex items-center rounded-full border border-amber-300 bg-white px-2 py-0.5 text-xs font-semibold text-amber-800"
              data-testid="podcast-kind-badge"
            >
              {podcastKindLabel}
            </span>
          </div>
          {#if primaryPodcastMetadata.kind === 'show' && primaryPodcastMetadata.highlightEpisodes.length > 0}
            <div class="mt-3 border-t border-amber-200 pt-3" data-testid="podcast-highlight-episodes">
              <p class="text-xs font-semibold text-amber-800">Highlighted Episodes</p>
              <ul class="mt-2 space-y-2">
                {#each primaryPodcastMetadata.highlightEpisodes as episode, index (`${episode.url}-${index}`)}
                  <li class="rounded-md border border-amber-100 bg-white px-3 py-2" data-testid="podcast-highlight-episode">
                    <a
                      href={episode.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      class="text-sm font-medium text-amber-900 hover:underline"
                    >
                      {episode.title}
                    </a>
                    {#if episode.note}
                      <p class="mt-1 text-xs text-amber-700">{episode.note}</p>
                    {/if}
                  </li>
                {/each}
              </ul>
            </div>
          {/if}
        </div>
      {/if}

      {#if isPodcastSection}
        <div class="mt-3">
          <PodcastSaveButton postId={post.id} {post} />
        </div>
      {/if}

      {#if isMovieSection}
        {#each secondaryMovieLinks as link, index (link.id ?? `${link.url}-${index}`)}
          {@const secondaryMovieMetadata = getMovieMetadataFromLink(link)}
          {#if secondaryMovieMetadata}
            <div class="mt-3">
              <MovieCard movie={secondaryMovieMetadata} />
            </div>
          {/if}
        {/each}
      {/if}

      {#if !isEditing && primaryLink?.highlights?.length}
        <div class="mt-2">
          <HighlightDisplay
            highlights={primaryLink.highlights}
            postId={post.id}
            onSeek={highlightEmbedProvider ? handleHighlightSeek : undefined}
            onToggleReaction={primaryLink?.id ? handleHighlightReaction : undefined}
            unsupportedMessage={highlightSeekMessage}
          />
        </div>
      {/if}

      <div class="mt-3 flex flex-wrap items-center gap-2">
        <div class="inline-flex items-center gap-1 rounded-full border border-gray-200 bg-white px-2 py-1 text-xs text-gray-600">
          <span>💬</span>
          <span>{post.commentCount || 0}</span>
        </div>
        <ReactionBar
          reactionCounts={post.reactionCounts ?? {}}
          userReactions={userReactions}
          onToggle={toggleReaction}
          postId={post.id}
        />
      </div>

      {#if sectionType === 'recipe'}
        <div class="mt-3">
          <RecipeStatsBar
            postId={post.id}
            saveCount={recipeStats?.saveCount ?? 0}
            cookCount={recipeStats?.cookCount ?? 0}
            averageRating={recipeStats?.averageRating ?? null}
            showEmpty
          />
        </div>
      {/if}
      {#if isBookSection && bookStatsForBar}
        <div class="mt-3">
          <BookStatsBar postId={post.id} bookStats={bookStatsForBar} />
        </div>
      {/if}
      {#if isBookSection && isThreadView}
        <div class="mt-3">
          <QuoteList postId={post.id} currentUserId={$currentUser?.id ?? ''} isAdmin={$isAdmin} />
        </div>
      {/if}
      {#if isMovieSection && movieStatsForBar}
        <div class="mt-3">
          <MovieStatsBar postId={post.id} stats={movieStatsForBar} />
        </div>
      {/if}

      <div class="mt-4 border-t border-gray-200 pt-4">
        <CommentThread
          postId={post.id}
          commentCount={post.commentCount ?? 0}
          {highlightCommentId}
          {highlightCommentIds}
          {profileUserId}
          {highlightQuery}
          {imageItems}
          imageReplyTarget={imageReplyTarget}
          onClearImageReply={clearImageReplyTarget}
          onImageReferenceClick={handleImageReferenceNavigate}
          sectionType={sectionType}
          onTimestampSeek={canSeekTimestamps ? handleHighlightSeek : null}
        />
      </div>
    </div>
  </div>
</article>

{#if isImageLightboxOpen && lightboxImageUrl}
  <div class="fixed inset-0 z-50 flex items-center justify-center px-4 py-6">
    <button
      type="button"
      class="absolute inset-0 bg-black/70"
      aria-label="Close image"
      on:click={closeImageLightbox}
    ></button>
    <div
      class={`relative z-10 max-h-full max-w-full ${imageItems.length > 1 ? 'pb-10' : ''}`}
      role="dialog"
      aria-modal="true"
      aria-label="Full size image"
      on:touchstart={handleLightboxTouchStart}
      on:touchend={handleLightboxTouchEnd}
      on:touchcancel={() => {
        lightboxTouchActive = false;
      }}
    >
      <button
        type="button"
        class="absolute -top-3 -right-3 flex h-8 w-8 items-center justify-center rounded-full bg-white text-gray-700 shadow-md hover:bg-gray-100"
        aria-label="Close image"
        on:click={closeImageLightbox}
      >
        ✕
      </button>
      {#if imageItems.length > 1}
        <div class="absolute inset-y-0 left-2 flex items-center pointer-events-none">
          <button
            type="button"
            class="flex h-10 w-10 items-center justify-center rounded-full bg-white/80 text-gray-700 shadow hover:bg-white pointer-events-auto"
            aria-label="Previous image"
            on:click={previousLightboxImage}
          >
            ‹
          </button>
        </div>
        <div class="absolute inset-y-0 right-2 flex items-center pointer-events-none">
          <button
            type="button"
            class="flex h-10 w-10 items-center justify-center rounded-full bg-white/80 text-gray-700 shadow hover:bg-white pointer-events-auto"
            aria-label="Next image"
            on:click={nextLightboxImage}
          >
            ›
          </button>
        </div>
      {/if}
      <div class="relative flex items-center justify-center">
        {#key lightboxImageUrl}
          <img
            src={lightboxImageUrl}
            alt={lightboxAltText}
            class="max-h-[85vh] w-auto max-w-[95vw] rounded-lg object-contain bg-white shadow-lg"
            style="touch-action: pan-y pinch-zoom;"
            in:fade={{ duration: 180 }}
          />
        {/key}
      </div>
      {#if lightboxImageUrl}
        <div class="relative z-10 mt-3 flex justify-center">
          <button
            type="button"
            class="inline-flex items-center gap-2 rounded-full bg-blue-600 px-4 py-2 text-xs font-medium text-white hover:bg-blue-700"
            on:click={() => {
              closeImageLightbox();
              startImageReply(lightboxImageIndex);
            }}
          >
            Reply to this image
          </button>
        </div>
      {/if}
      {#if imageItems.length > 1}
        <div class="absolute bottom-3 left-0 right-0 z-0 flex items-center justify-center">
          <span class="rounded-full bg-black/60 px-2.5 py-1 text-xs text-white">
            {lightboxImageIndex + 1} of {imageItems.length}
          </span>
        </div>
      {/if}
    </div>
  </div>
{/if}
