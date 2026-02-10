<script lang="ts">
  import { onDestroy } from 'svelte';
  import { get } from 'svelte/store';
  import { bookStore, bookStoreMeta, readHistory } from '../../stores/bookStore';
  import type { BookStats } from '../../stores/postStore';
  import RatingStars from '../recipes/RatingStars.svelte';

  type ReadLogLike = {
    id: string;
    userId: string;
    postId: string;
    rating?: number | null;
    createdAt: string;
  };

  export let postId: string;
  export let bookStats: BookStats;

  const LONG_PRESS_MS = 500;

  let isSaving = false;
  let isRatingPopoverOpen = false;
  let isRemoveMenuOpen = false;
  let longPressTimer: ReturnType<typeof setTimeout> | null = null;
  let longPressTriggered = false;
  let optimisticReadLog: ReadLogLike | null | undefined = undefined;
  let hasDismissedInitialRead = false;
  let initialReadSignature = '';
  let rootRef: HTMLDivElement | null = null;

  $: {
    const nextSignature = `${postId}:${bookStats?.viewerRead ?? false}:${bookStats?.viewerRating ?? ''}`;
    if (nextSignature !== initialReadSignature) {
      initialReadSignature = nextSignature;
      hasDismissedInitialRead = false;
    }
  }

  $: propReadLog =
    bookStats?.viewerRead && !hasDismissedInitialRead
      ? {
          id: 'initial-read-log',
          userId: 'me',
          postId,
          rating: normalizeRating(bookStats?.viewerRating),
          createdAt: new Date().toISOString(),
        }
      : null;
  $: storedReadLog = $readHistory.find((entry) => entry.postId === postId) ?? null;
  $: baseReadLog = storedReadLog ?? propReadLog;
  $: activeReadLog = optimisticReadLog !== undefined ? optimisticReadLog : baseReadLog;
  $: isRead = Boolean(activeReadLog);
  $: displayedRating = normalizeRating(activeReadLog?.rating);

  function normalizeRating(value: number | null | undefined): number | null {
    if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) {
      return null;
    }
    return Math.min(5, Math.max(1, Math.round(value)));
  }

  function clearLongPressTimer() {
    if (!longPressTimer) {
      return;
    }
    clearTimeout(longPressTimer);
    longPressTimer = null;
  }

  function buildOptimisticReadLog(rating: number | null): ReadLogLike {
    const now = new Date().toISOString();
    return {
      id: activeReadLog?.id ?? 'optimistic-read-log',
      userId: activeReadLog?.userId ?? 'me',
      postId,
      rating,
      createdAt: activeReadLog?.createdAt ?? now,
    };
  }

  async function markAsRead() {
    if (isSaving) {
      return;
    }

    isSaving = true;
    const previous = activeReadLog;
    optimisticReadLog = buildOptimisticReadLog(displayedRating);

    try {
      await bookStore.logRead(postId, displayedRating ?? undefined);
    } finally {
      const { error } = get(bookStoreMeta);
      if (error) {
        optimisticReadLog = previous ?? null;
      } else {
        optimisticReadLog = undefined;
      }
      isSaving = false;
    }
  }

  async function updateRating(nextRating: number) {
    if (isSaving || nextRating <= 0 || !isRead) {
      return;
    }

    isSaving = true;
    const previous = activeReadLog;
    optimisticReadLog = buildOptimisticReadLog(nextRating);

    try {
      await bookStore.updateRating(postId, nextRating);
    } finally {
      const { error } = get(bookStoreMeta);
      if (error) {
        optimisticReadLog = previous ?? null;
      } else {
        optimisticReadLog = undefined;
      }
      isSaving = false;
    }
  }

  async function removeReadLog() {
    if (!isRead || isSaving) {
      return;
    }

    isSaving = true;
    const previous = activeReadLog;
    optimisticReadLog = null;

    try {
      await bookStore.removeRead(postId);
    } finally {
      const { error } = get(bookStoreMeta);
      if (error) {
        optimisticReadLog = previous ?? null;
      } else {
        hasDismissedInitialRead = true;
        optimisticReadLog = undefined;
        isRemoveMenuOpen = false;
        isRatingPopoverOpen = false;
      }
      isSaving = false;
    }
  }

  async function handlePrimaryClick() {
    if (isSaving) {
      return;
    }

    if (!isRead) {
      await markAsRead();
      return;
    }

    isRemoveMenuOpen = false;
    isRatingPopoverOpen = !isRatingPopoverOpen;
  }

  function openRemoveMenu() {
    if (!isRead || isSaving) {
      return;
    }

    isRatingPopoverOpen = false;
    isRemoveMenuOpen = true;
  }

  function handleContextMenu(event: MouseEvent) {
    if (!isRead) {
      return;
    }
    event.preventDefault();
    openRemoveMenu();
  }

  function handlePointerDown(event: PointerEvent) {
    if (!isRead || isSaving || event.button !== 0) {
      return;
    }

    clearLongPressTimer();
    longPressTriggered = false;
    longPressTimer = setTimeout(() => {
      longPressTriggered = true;
      openRemoveMenu();
    }, LONG_PRESS_MS);
  }

  function handlePointerRelease() {
    clearLongPressTimer();
  }

  function handleButtonClick() {
    if (longPressTriggered) {
      longPressTriggered = false;
      return;
    }

    void handlePrimaryClick();
  }

  function handleWindowClick(event: MouseEvent) {
    if (!rootRef) {
      return;
    }

    const target = event.target;
    if (target instanceof Node && rootRef.contains(target)) {
      return;
    }

    isRatingPopoverOpen = false;
    isRemoveMenuOpen = false;
  }

  function handleWindowKeydown(event: KeyboardEvent) {
    if (event.key !== 'Escape') {
      return;
    }
    isRatingPopoverOpen = false;
    isRemoveMenuOpen = false;
  }

  onDestroy(() => {
    clearLongPressTimer();
  });
</script>

<svelte:window on:click={handleWindowClick} on:keydown={handleWindowKeydown} />

<div class="relative inline-flex items-center" bind:this={rootRef} data-testid="read-button-container">
  <button
    type="button"
    class={`inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-semibold transition-colors ${
      isRead
        ? 'border-emerald-200 bg-emerald-50 text-emerald-800 hover:border-emerald-300 hover:bg-emerald-100'
        : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50'
    }`}
    on:click={handleButtonClick}
    on:contextmenu={handleContextMenu}
    on:pointerdown={handlePointerDown}
    on:pointerup={handlePointerRelease}
    on:pointercancel={handlePointerRelease}
    on:pointerleave={handlePointerRelease}
    disabled={isSaving}
    data-testid={isRead ? 'read-button-read' : 'read-button'}
    aria-haspopup={isRead ? 'dialog' : undefined}
    aria-expanded={isRead ? isRatingPopoverOpen || isRemoveMenuOpen : undefined}
  >
    {#if isRead}
      <svg
        viewBox="0 0 20 20"
        class="h-4 w-4 text-emerald-600"
        fill="currentColor"
        aria-hidden="true"
      >
        <path
          d="M7.5 13.4L3.6 9.6a1 1 0 0 1 1.4-1.4l2.5 2.5 7.1-7.1a1 1 0 1 1 1.4 1.4l-8 8a1 1 0 0 1-1.4 0z"
        />
      </svg>
      {#if displayedRating}
        <span>Read</span>
        <span data-testid="read-rating-display">
          <RatingStars value={displayedRating} readonly size="sm" ariaLabel="Read rating" />
        </span>
        <span class="sr-only" data-testid="read-rating-value">{displayedRating}</span>
      {:else}
        <span>Read</span>
      {/if}
    {:else}
      <svg
        viewBox="0 0 24 24"
        class="h-4 w-4"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path d="M4 3.5h11a2 2 0 0 1 2 2V19l-3-1.5L11 19l-3-1.5L4 19V3.5z" />
        <path d="m8.5 11 1.8 1.8 3.2-3.2" />
      </svg>
      <span>Mark as Read</span>
    {/if}
  </button>

  {#if isRatingPopoverOpen}
    <div
      class="absolute right-0 top-full z-10 mt-2 w-52 rounded-xl border border-gray-200 bg-white p-3 shadow-lg"
      role="dialog"
      aria-label="Read rating"
      data-testid="read-rating-popover"
    >
      <p class="text-xs font-semibold text-gray-700">Your rating</p>
      <div class="mt-2">
        <RatingStars
          value={displayedRating ?? 0}
          on:change={(event) => void updateRating(event.detail)}
          ariaLabel="Read rating selector"
        />
      </div>
      <p class="mt-2 text-[11px] text-gray-500">Right-click or long press to remove this read log.</p>
    </div>
  {/if}

  {#if isRemoveMenuOpen}
    <div
      class="absolute right-0 top-full z-10 mt-2 w-52 rounded-xl border border-gray-200 bg-white p-2 shadow-lg"
      role="menu"
      aria-label="Read actions"
      data-testid="read-remove-menu"
    >
      <button
        type="button"
        class="w-full rounded-lg px-3 py-2 text-left text-xs font-semibold text-red-600 hover:bg-red-50 disabled:opacity-60"
        on:click={() => void removeReadLog()}
        disabled={isSaving}
        data-testid="read-remove"
      >
        Remove read log
      </button>
    </div>
  {/if}
</div>
