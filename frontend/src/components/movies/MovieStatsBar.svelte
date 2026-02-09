<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api, type WatchlistUser, type WatchLog } from '../../services/api';
  import RatingStars from '../recipes/RatingStars.svelte';
  import WatchButton from './WatchButton.svelte';
  import WatchlistSaveButton from './WatchlistSaveButton.svelte';

  type MovieStats = {
    watchlistCount: number;
    watchCount: number;
    avgRating?: number;
    viewerWatchlisted: boolean;
    viewerWatched: boolean;
    viewerRating?: number;
    viewerCategories?: string[];
  };

  export let postId: string;
  export let stats: MovieStats;

  const TOOLTIP_DELAY = 150;

  let watchlistTooltipContainer: HTMLDivElement | null = null;
  let watchTooltipContainer: HTMLDivElement | null = null;
  let showWatchlistTooltip = false;
  let showWatchTooltip = false;
  let watchlistTooltipLoading = false;
  let watchTooltipLoading = false;
  let watchlistTooltipError: string | null = null;
  let watchTooltipError: string | null = null;
  let watchlistUsers: WatchlistUser[] = [];
  let watchLogs: WatchLog[] = [];
  let watchlistTooltipTimeout: ReturnType<typeof setTimeout> | null = null;
  let watchTooltipTimeout: ReturnType<typeof setTimeout> | null = null;
  let lastWatchlistKey = '';
  let lastWatchKey = '';

  const clampCount = (value: number) => Math.max(0, Number.isFinite(value) ? value : 0);
  const clampRating = (value: number) => Math.min(5, Math.max(0, value));

  $: normalizedWatchlistCount = clampCount(stats?.watchlistCount ?? 0);
  $: normalizedWatchCount = clampCount(stats?.watchCount ?? 0);
  $: normalizedAvgRating =
    typeof stats?.avgRating === 'number' && Number.isFinite(stats.avgRating)
      ? clampRating(stats.avgRating)
      : null;
  $: hasAvgRating = normalizedAvgRating !== null;
  $: formattedAvgRating =
    normalizedAvgRating === null
      ? ''
      : Number.isInteger(normalizedAvgRating)
        ? normalizedAvgRating.toFixed(0)
        : normalizedAvgRating.toFixed(1);

  $: watchlistKey = `${postId}:${normalizedWatchlistCount}`;
  $: watchKey = `${postId}:${normalizedWatchCount}:${normalizedAvgRating ?? 'none'}`;

  function getInitial(name?: string | null) {
    const trimmed = name?.trim();
    return trimmed && trimmed.length > 0 ? trimmed.charAt(0).toUpperCase() : '?';
  }

  function handleFocusOut(event: FocusEvent, container: HTMLDivElement | null, hide: () => void) {
    if (container && event.relatedTarget) {
      const target = event.relatedTarget as Node;
      if (container.contains(target)) {
        return;
      }
    }

    hide();
  }

  function hideWatchlistTooltip() {
    showWatchlistTooltip = false;
    if (watchlistTooltipTimeout) {
      clearTimeout(watchlistTooltipTimeout);
      watchlistTooltipTimeout = null;
    }
  }

  function hideWatchTooltip() {
    showWatchTooltip = false;
    if (watchTooltipTimeout) {
      clearTimeout(watchTooltipTimeout);
      watchTooltipTimeout = null;
    }
  }

  async function loadWatchlistTooltipData(force = false) {
    if (!postId || normalizedWatchlistCount <= 0) {
      return;
    }

    if (!force && watchlistUsers.length > 0 && watchlistKey === lastWatchlistKey) {
      return;
    }

    watchlistTooltipLoading = true;
    watchlistTooltipError = null;

    try {
      const response = await api.getPostWatchlistInfo(postId);
      watchlistUsers = response.users ?? [];
      lastWatchlistKey = watchlistKey;
    } catch (error) {
      watchlistTooltipError =
        error instanceof Error ? error.message : 'Failed to load watchlist users.';
    } finally {
      watchlistTooltipLoading = false;
    }
  }

  async function loadWatchTooltipData(force = false) {
    if (!postId || normalizedWatchCount <= 0) {
      return;
    }

    if (!force && watchLogs.length > 0 && watchKey === lastWatchKey) {
      return;
    }

    watchTooltipLoading = true;
    watchTooltipError = null;

    try {
      const response = await api.getPostWatchLogs(postId);
      watchLogs = response.logs ?? [];
      lastWatchKey = watchKey;
    } catch (error) {
      watchTooltipError = error instanceof Error ? error.message : 'Failed to load watches.';
    } finally {
      watchTooltipLoading = false;
    }
  }

  function showWatchlistTooltipWithDelay() {
    if (!postId || normalizedWatchlistCount <= 0) {
      return;
    }

    if (watchlistTooltipTimeout) {
      clearTimeout(watchlistTooltipTimeout);
    }

    watchlistTooltipTimeout = setTimeout(async () => {
      showWatchlistTooltip = true;
      await loadWatchlistTooltipData();
    }, TOOLTIP_DELAY);
  }

  function showWatchTooltipWithDelay() {
    if (!postId || normalizedWatchCount <= 0) {
      return;
    }

    if (watchTooltipTimeout) {
      clearTimeout(watchTooltipTimeout);
    }

    watchTooltipTimeout = setTimeout(async () => {
      showWatchTooltip = true;
      await loadWatchTooltipData();
    }, TOOLTIP_DELAY);
  }

  onDestroy(() => {
    if (watchlistTooltipTimeout) {
      clearTimeout(watchlistTooltipTimeout);
    }

    if (watchTooltipTimeout) {
      clearTimeout(watchTooltipTimeout);
    }
  });
</script>

<div
  class="flex flex-col gap-3 rounded-xl border border-gray-200 bg-white px-3 py-2 text-xs text-gray-600 sm:flex-row sm:items-center sm:justify-between"
  data-testid="movie-stats-bar"
>
  <div class="flex flex-wrap items-center gap-3">
    <div
      class="relative"
      role="group"
      bind:this={watchlistTooltipContainer}
      on:mouseenter={showWatchlistTooltipWithDelay}
      on:mouseleave={hideWatchlistTooltip}
      on:focusin={showWatchlistTooltipWithDelay}
      on:focusout={(event) => handleFocusOut(event, watchlistTooltipContainer, hideWatchlistTooltip)}
      data-testid="movie-watchlist-stat"
    >
      <div class="inline-flex items-center gap-1.5">
        <span class="inline-flex h-6 w-6 items-center justify-center rounded-full bg-violet-50 text-violet-700">
          📋
        </span>
        <span class="font-semibold text-gray-800">{normalizedWatchlistCount}</span>
        <span>saved</span>
      </div>

      {#if showWatchlistTooltip}
        <div
          class="absolute left-0 top-full z-20 mt-2 w-64 rounded-lg border border-gray-200 bg-white p-3 text-xs shadow-lg"
          role="tooltip"
          data-testid="movie-watchlist-tooltip"
        >
          {#if watchlistTooltipLoading}
            <p class="text-gray-500">Loading watchlist users...</p>
          {:else if watchlistTooltipError}
            <p class="text-red-500">{watchlistTooltipError}</p>
          {:else if watchlistUsers.length === 0}
            <p class="text-gray-500">No saves yet.</p>
          {:else}
            <div class="space-y-2">
              {#each watchlistUsers as user}
                <div class="flex items-center gap-2">
                  {#if user.avatar}
                    <img
                      src={user.avatar}
                      alt={user.displayName}
                      class="h-7 w-7 rounded-full object-cover"
                    />
                  {:else}
                    <div class="h-7 w-7 rounded-full bg-gray-200 text-gray-500 flex items-center justify-center text-xs font-semibold">
                      {getInitial(user.displayName)}
                    </div>
                  {/if}
                  <span class="text-gray-700">{user.displayName || user.username || 'Unknown'}</span>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/if}
    </div>

    <div
      class="relative"
      role="group"
      bind:this={watchTooltipContainer}
      on:mouseenter={showWatchTooltipWithDelay}
      on:mouseleave={hideWatchTooltip}
      on:focusin={showWatchTooltipWithDelay}
      on:focusout={(event) => handleFocusOut(event, watchTooltipContainer, hideWatchTooltip)}
      data-testid="movie-watch-stat"
    >
      <div class="inline-flex items-center gap-1.5">
        <span class="inline-flex h-6 w-6 items-center justify-center rounded-full bg-amber-50 text-amber-700">
          👁
        </span>
        <span class="font-semibold text-gray-800">{normalizedWatchCount}</span>
        <span>watched</span>
      </div>

      {#if showWatchTooltip}
        <div
          class="absolute left-0 top-full z-20 mt-2 w-72 rounded-lg border border-gray-200 bg-white p-3 text-xs shadow-lg"
          role="tooltip"
          data-testid="movie-watch-tooltip"
        >
          {#if watchTooltipLoading}
            <p class="text-gray-500">Loading watch logs...</p>
          {:else if watchTooltipError}
            <p class="text-red-500">{watchTooltipError}</p>
          {:else if watchLogs.length === 0}
            <p class="text-gray-500">No watches yet.</p>
          {:else}
            <div class="space-y-2">
              {#each watchLogs as log}
                <div class="flex items-center justify-between gap-2">
                  <div class="flex items-center gap-2 min-w-0">
                    {#if log.user?.avatar}
                      <img
                        src={log.user.avatar}
                        alt={log.user.displayName}
                        class="h-7 w-7 rounded-full object-cover"
                      />
                    {:else}
                      <div class="h-7 w-7 rounded-full bg-gray-200 text-gray-500 flex items-center justify-center text-xs font-semibold">
                        {getInitial(log.user?.displayName)}
                      </div>
                    {/if}
                    <span class="text-gray-700 truncate">{log.user?.displayName || log.user?.username || 'Unknown'}</span>
                  </div>
                  <div class="flex items-center gap-1">
                    <RatingStars
                      value={log.rating}
                      readonly
                      size="sm"
                      ariaLabel={`${log.user?.displayName || 'User'} rating`}
                    />
                    <span class="text-gray-500 font-semibold">{log.rating.toFixed(1)}</span>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/if}
    </div>

    {#if hasAvgRating}
      <div class="inline-flex items-center gap-1.5" data-testid="movie-average-rating">
        <span class="inline-flex h-6 w-6 items-center justify-center rounded-full bg-amber-100 text-amber-700">
          ★
        </span>
        <span class="font-semibold text-gray-800">{formattedAvgRating}</span>
        <span>avg</span>
        <RatingStars
          value={normalizedAvgRating ?? 0}
          readonly
          size="sm"
          ariaLabel={`Average rating ${formattedAvgRating}`}
        />
      </div>
    {/if}
  </div>

  <div class="flex flex-wrap items-center gap-2" data-testid="movie-stats-actions">
    <WatchlistSaveButton
      postId={postId}
      initialSaved={stats?.viewerWatchlisted ?? false}
      initialCategories={stats?.viewerCategories ?? []}
      saveCount={normalizedWatchlistCount}
    />
    <WatchButton
      postId={postId}
      initialWatched={stats?.viewerWatched ?? false}
      initialRating={stats?.viewerRating ?? null}
      watchCount={normalizedWatchCount}
    />
  </div>
</div>
