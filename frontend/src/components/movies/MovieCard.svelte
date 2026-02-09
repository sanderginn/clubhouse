<script lang="ts">
  import { onDestroy, tick } from 'svelte';
  import YouTubeEmbed from '../../lib/components/embeds/YouTubeEmbed.svelte';
  import { lockBodyScroll, unlockBodyScroll } from '../../lib/scrollLock';

  type CastMember = {
    name: string;
    character: string;
  };

  type MovieMetadata = {
    title: string;
    overview?: string;
    poster?: string;
    backdrop?: string;
    runtime?: number;
    genres?: string[];
    releaseDate?: string;
    cast?: CastMember[];
    director?: string;
    tmdbRating?: number;
    trailerKey?: string;
    tmdbId?: number;
    tmdb_id?: number;
    tmdbMediaType?: string;
    tmdb_media_type?: string;
  };

  export let movie: MovieMetadata = { title: '' };
  export let expanded = false;

  const FOCUSABLE_SELECTOR =
    'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

  let posterLoadFailed = false;
  let previousPosterUrl: string | null = null;
  let trailerModalOpen = false;
  let trailerDialog: HTMLDivElement | null = null;
  let trailerCloseButton: HTMLButtonElement | null = null;
  let previousFocusedElement: HTMLElement | null = null;

  $: posterUrl = movie.poster || movie.backdrop || null;
  $: if (posterUrl !== previousPosterUrl) {
    previousPosterUrl = posterUrl;
    posterLoadFailed = false;
  }
  $: title = movie.title?.trim() || 'Untitled movie';
  $: releaseYear = formatReleaseYear(movie.releaseDate);
  $: runtimeLabel = formatRuntime(movie.runtime);
  $: ratingLabel =
    typeof movie.tmdbRating === 'number' && Number.isFinite(movie.tmdbRating)
      ? movie.tmdbRating.toFixed(1)
      : 'N/A';
  $: metaLine = runtimeLabel ? `★ ${ratingLabel} · ${runtimeLabel}` : `★ ${ratingLabel}`;
  $: visibleGenres = (movie.genres ?? []).filter(Boolean).slice(0, 3);
  $: remainingGenres = Math.max((movie.genres?.length ?? 0) - visibleGenres.length, 0);
  $: directorLabel = movie.director?.trim() || 'Unknown';
  $: overviewText = movie.overview?.trim() || 'No synopsis available.';
  $: castList = (movie.cast ?? []).slice(0, 5);
  $: trailerEmbedUrl = buildTrailerUrl(movie.trailerKey);
  $: tmdbId = resolveTMDBID(movie.tmdbId ?? movie.tmdb_id);
  $: tmdbMediaType = normalizeTMDBMediaType(movie.tmdbMediaType ?? movie.tmdb_media_type);
  $: tmdbUrl =
    typeof tmdbId === 'number' && tmdbMediaType
      ? `https://www.themoviedb.org/${tmdbMediaType}/${tmdbId}`
      : null;

  function formatRuntime(runtime?: number): string | null {
    if (typeof runtime !== 'number' || !Number.isFinite(runtime) || runtime <= 0) {
      return null;
    }

    const totalMinutes = Math.round(runtime);
    const hours = Math.floor(totalMinutes / 60);
    const minutes = totalMinutes % 60;

    if (hours === 0) {
      return `${minutes}m`;
    }
    if (minutes === 0) {
      return `${hours}h`;
    }
    return `${hours}h ${minutes}m`;
  }

  function formatReleaseYear(date?: string): string | null {
    const value = date?.trim();
    if (!value) {
      return null;
    }

    const matchedYear = value.match(/^(\d{4})/);
    if (matchedYear?.[1]) {
      return matchedYear[1];
    }

    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) {
      return null;
    }

    return String(parsed.getUTCFullYear());
  }

  function buildTrailerUrl(trailerKey?: string): string | null {
    const key = trailerKey?.trim();
    if (!key) {
      return null;
    }

    return `https://www.youtube-nocookie.com/embed/${encodeURIComponent(key)}`;
  }

  function resolveTMDBID(rawValue?: number): number | null {
    if (typeof rawValue !== 'number' || !Number.isFinite(rawValue)) {
      return null;
    }
    const parsed = Math.trunc(rawValue);
    if (parsed <= 0) {
      return null;
    }
    return parsed;
  }

  function normalizeTMDBMediaType(value?: string): 'movie' | 'tv' | null {
    const normalized = value?.trim().toLowerCase();
    if (normalized === 'movie') {
      return 'movie';
    }
    if (normalized === 'tv' || normalized === 'series') {
      return 'tv';
    }
    return null;
  }

  function toggleExpanded() {
    expanded = !expanded;
  }

  function openTrailerModal() {
    if (!trailerEmbedUrl || typeof document === 'undefined') {
      return;
    }

    previousFocusedElement =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;
    trailerModalOpen = true;
    lockBodyScroll();

    void tick().then(() => {
      trailerCloseButton?.focus();
    });
  }

  function closeTrailerModal() {
    if (!trailerModalOpen) {
      return;
    }

    trailerModalOpen = false;
    unlockBodyScroll();
    previousFocusedElement?.focus();
  }

  function trapFocus(event: KeyboardEvent) {
    if (!trailerDialog || typeof document === 'undefined') {
      return;
    }

    const focusableElements = Array.from(
      trailerDialog.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)
    ).filter((element) => !element.hasAttribute('disabled'));

    if (focusableElements.length === 0) {
      event.preventDefault();
      trailerDialog.focus();
      return;
    }

    const firstElement = focusableElements[0];
    const lastElement = focusableElements[focusableElements.length - 1];
    const activeElement = document.activeElement as HTMLElement | null;

    if (!activeElement || !focusableElements.includes(activeElement)) {
      event.preventDefault();
      firstElement.focus();
      return;
    }

    if (event.shiftKey && activeElement === firstElement) {
      event.preventDefault();
      lastElement.focus();
      return;
    }

    if (!event.shiftKey && activeElement === lastElement) {
      event.preventDefault();
      firstElement.focus();
    }
  }

  function handleWindowKeydown(event: KeyboardEvent) {
    if (!trailerModalOpen) {
      return;
    }

    if (event.key === 'Escape') {
      event.preventDefault();
      closeTrailerModal();
      return;
    }

    if (
      event.key === 'Tab' &&
      trailerDialog &&
      typeof document !== 'undefined' &&
      trailerDialog.contains(document.activeElement)
    ) {
      trapFocus(event);
    }
  }

  function handlePosterError() {
    posterLoadFailed = true;
  }

  onDestroy(() => {
    if (trailerModalOpen) {
      unlockBodyScroll();
    }
  });
</script>

<svelte:window on:keydown={handleWindowKeydown} />

<article
  class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm"
  data-testid="movie-card"
>
  <div class="flex flex-col gap-4 p-4 sm:flex-row sm:items-start sm:p-5">
    <div
      class={`relative w-full shrink-0 overflow-hidden rounded-lg bg-slate-100 ${
        expanded ? 'h-72 sm:h-80 sm:w-52' : 'h-52 sm:h-44 sm:w-32'
      }`}
    >
      {#if posterUrl && !posterLoadFailed}
        <img
          src={posterUrl}
          alt={`${title} poster`}
          class="h-full w-full object-cover"
          loading="lazy"
          on:error={handlePosterError}
          data-testid="movie-poster"
        />
      {:else}
        <div
          class="flex h-full w-full items-center justify-center bg-slate-200 px-4 text-center text-sm font-medium text-slate-600"
          data-testid="movie-poster-fallback"
        >
          Poster unavailable
        </div>
      {/if}
    </div>

    <div class="min-w-0 flex-1">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0">
          <h3 class="text-lg font-semibold text-slate-900" data-testid="movie-title">
            {title}
            {#if releaseYear}
              <span class="text-slate-500">({releaseYear})</span>
            {/if}
          </h3>
          <p class="mt-1 text-sm text-slate-700" data-testid="movie-meta-line">{metaLine}</p>
          {#if tmdbUrl}
            <a
              href={tmdbUrl}
              target="_blank"
              rel="noopener noreferrer"
              class="mt-1 inline-flex text-xs font-medium text-slate-500 underline-offset-2 hover:text-slate-700 hover:underline"
              data-testid="movie-tmdb-link"
            >
              View on TMDB
            </a>
          {/if}
        </div>

        <button
          type="button"
          class="rounded-full border border-slate-200 px-3 py-1 text-xs font-semibold text-slate-700 hover:border-slate-300 hover:bg-slate-50"
          on:click={toggleExpanded}
          data-testid="movie-expand-toggle"
        >
          {expanded ? 'Collapse details' : 'View details'}
        </button>
      </div>

      {#if visibleGenres.length > 0}
        <div class="mt-3 flex flex-wrap gap-2" data-testid="movie-genres">
          {#each visibleGenres as genre}
            <span class="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-700"
              >{genre}</span
            >
          {/each}
          {#if remainingGenres > 0}
            <span class="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-700">
              +{remainingGenres}
            </span>
          {/if}
        </div>
      {/if}

      <p
        class="mt-3 max-w-full truncate text-sm text-slate-700"
        title={directorLabel}
        data-testid="movie-director"
      >
        Dir: {directorLabel}
      </p>

      {#if expanded}
        <div
          class="mt-4 space-y-4 rounded-lg border border-slate-100 bg-slate-50 p-4"
          data-testid="movie-expanded-content"
        >
          <p class="text-sm leading-relaxed text-slate-700" data-testid="movie-overview">
            {overviewText}
          </p>

          {#if castList.length > 0}
            <div>
              <p class="text-xs font-semibold uppercase tracking-wide text-slate-500">Cast</p>
              <ul class="mt-2 space-y-1 text-sm text-slate-700" data-testid="movie-cast-list">
                {#each castList as castMember}
                  <li>
                    <span class="font-medium text-slate-800">{castMember.name}</span>
                    {#if castMember.character}
                      <span class="text-slate-600"> as {castMember.character}</span>
                    {/if}
                  </li>
                {/each}
              </ul>
            </div>
          {/if}

          {#if trailerEmbedUrl}
            <button
              type="button"
              class="rounded-full bg-red-600 px-4 py-2 text-xs font-semibold text-white hover:bg-red-700"
              aria-haspopup="dialog"
              on:click={openTrailerModal}
              data-testid="movie-trailer-button"
            >
              Watch Trailer
            </button>
          {/if}
        </div>
      {/if}
    </div>
  </div>
</article>

{#if trailerModalOpen && trailerEmbedUrl}
  <div class="fixed inset-0 z-50 flex items-center justify-center px-4 py-6">
    <button
      type="button"
      class="absolute inset-0 bg-black/70"
      aria-label="Close trailer modal"
      on:click={closeTrailerModal}
    ></button>
    <div
      bind:this={trailerDialog}
      class="relative z-10 w-full max-w-3xl rounded-xl bg-white p-4 shadow-xl sm:p-5"
      role="dialog"
      aria-modal="true"
      aria-label={`Trailer for ${title}`}
      tabindex="-1"
    >
      <div class="mb-3 flex items-center justify-between gap-3">
        <h4 class="text-sm font-semibold text-slate-900">Watch Trailer</h4>
        <button
          type="button"
          class="rounded-full border border-slate-200 px-3 py-1 text-xs font-semibold text-slate-700 hover:border-slate-300 hover:bg-slate-50"
          on:click={closeTrailerModal}
          bind:this={trailerCloseButton}
          data-testid="movie-trailer-close"
        >
          Close
        </button>
      </div>
      <YouTubeEmbed embedUrl={trailerEmbedUrl} title={`${title} trailer`} />
    </div>
  </div>
{/if}
