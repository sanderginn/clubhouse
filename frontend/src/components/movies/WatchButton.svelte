<script lang="ts">
  import { onDestroy, tick } from 'svelte';
  import { get } from 'svelte/store';
  import { movieStore, type WatchLog } from '../../stores/movieStore';
  import RatingStars from '../recipes/RatingStars.svelte';

  export let postId: string;
  export let initialWatched = false;
  export let initialRating: number | null = null;
  export let watchCount = 0;

  const FOCUSABLE_SELECTOR =
    'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

  const initialWatchTimestamp = new Date().toISOString();

  let isModalOpen = false;
  let ratingValue = 0;
  let notesValue = '';
  let isSaving = false;
  let savingAction: 'save' | 'remove' | null = null;
  let formError = '';
  let toastMessage = '';
  let toastTimer: ReturnType<typeof setTimeout> | null = null;
  let modalRef: HTMLDivElement | null = null;
  let closeButtonRef: HTMLButtonElement | null = null;
  let optimisticWatchLog: WatchLog | null | undefined = undefined;
  let propWatchLog: WatchLog | null = null;
  let storedWatchLog: WatchLog | null = null;
  let baseWatchLog: WatchLog | null = null;
  let activeWatchLog: WatchLog | null = null;

  $: propWatchLog =
    initialWatched
      ? {
          id: 'initial-watch-log',
          userId: 'me',
          postId,
          rating: initialRating ?? 0,
          notes: undefined,
          watchedAt: initialWatchTimestamp,
          post: undefined,
        }
      : null;
  $: storedWatchLog = $movieStore.watchLogs.find((log) => log.postId === postId) ?? null;
  $: baseWatchLog = storedWatchLog ?? propWatchLog;
  $: activeWatchLog = optimisticWatchLog !== undefined ? optimisticWatchLog : baseWatchLog;
  $: isWatched = Boolean(activeWatchLog);
  $: displayedRating = activeWatchLog?.rating ?? 0;
  $: watchCountLabel = `${watchCount} ${watchCount === 1 ? 'watch' : 'watches'}`;

  function showToast(message: string) {
    toastMessage = message;
    if (toastTimer) {
      clearTimeout(toastTimer);
    }
    toastTimer = setTimeout(() => {
      toastMessage = '';
      toastTimer = null;
    }, 4000);
  }

  function openModal() {
    if (isSaving) {
      return;
    }

    ratingValue = activeWatchLog?.rating ?? initialRating ?? 0;
    notesValue = activeWatchLog?.notes ?? '';
    formError = '';
    isModalOpen = true;

    void tick().then(() => {
      closeButtonRef?.focus();
    });
  }

  function closeModal() {
    if (isSaving) {
      return;
    }

    isModalOpen = false;
    formError = '';
  }

  function buildOptimisticLog(): WatchLog {
    const now = new Date().toISOString();
    const notes = notesValue.trim();

    return {
      id: activeWatchLog?.id ?? 'optimistic-watch-log',
      userId: activeWatchLog?.userId ?? 'me',
      postId,
      rating: ratingValue,
      notes: notes || undefined,
      watchedAt: now,
      post: activeWatchLog?.post,
    };
  }

  async function handleSave() {
    if (ratingValue <= 0) {
      formError = 'Select a rating to continue.';
      return;
    }

    if (isSaving) {
      return;
    }

    isSaving = true;
    savingAction = 'save';
    formError = '';

    const previous = activeWatchLog;
    const notes = notesValue.trim();

    optimisticWatchLog = buildOptimisticLog();
    movieStore.setError(null);

    try {
      if (previous) {
        await movieStore.updateWatchLog(postId, ratingValue, notes || undefined);
      } else {
        await movieStore.logWatch(postId, ratingValue, notes || undefined);
      }
    } finally {
      const { error } = get(movieStore);
      if (error) {
        optimisticWatchLog = previous ?? null;
        formError = error;
        showToast(error);
      } else {
        optimisticWatchLog = undefined;
        isModalOpen = false;
      }
      isSaving = false;
      savingAction = null;
    }
  }

  async function handleRemove() {
    if (!activeWatchLog || isSaving) {
      return;
    }

    isSaving = true;
    savingAction = 'remove';
    formError = '';

    const previous = activeWatchLog;
    optimisticWatchLog = null;
    movieStore.setError(null);

    try {
      await movieStore.removeWatchLog(postId);
    } finally {
      const { error } = get(movieStore);
      if (error) {
        optimisticWatchLog = previous;
        formError = error;
        showToast(error);
      } else {
        optimisticWatchLog = undefined;
        isModalOpen = false;
      }
      isSaving = false;
      savingAction = null;
    }
  }

  function trapModalFocus(event: KeyboardEvent) {
    if (!modalRef || typeof document === 'undefined') {
      return;
    }

    const focusable = Array.from(modalRef.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
      (element) => !element.hasAttribute('disabled')
    );

    if (focusable.length === 0) {
      event.preventDefault();
      modalRef.focus();
      return;
    }

    const firstElement = focusable[0];
    const lastElement = focusable[focusable.length - 1];
    const activeElement = document.activeElement as HTMLElement | null;

    if (!activeElement || !focusable.includes(activeElement)) {
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
    if (!isModalOpen) {
      return;
    }

    if (event.key === 'Escape') {
      event.preventDefault();
      closeModal();
      return;
    }

    if (event.key === 'Tab') {
      trapModalFocus(event);
    }
  }

  onDestroy(() => {
    if (toastTimer) {
      clearTimeout(toastTimer);
    }
  });
</script>

<svelte:window on:keydown={handleWindowKeydown} />

<div class="inline-flex items-center gap-2" data-testid="watch-button-container">
  <button
    type="button"
    class={`inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-semibold transition-colors ${
      isWatched
        ? 'border-amber-200 bg-amber-50 text-amber-800 hover:border-amber-300 hover:bg-amber-100'
        : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50'
    }`}
    on:click={openModal}
    data-testid={isWatched ? 'watched-button' : 'watch-button'}
  >
    {#if isWatched}
      <svg viewBox="0 0 20 20" class="h-4 w-4 text-amber-500" fill="currentColor" aria-hidden="true">
        <path
          d="M10 1.5l2.47 5.4 5.88.5-4.42 3.83 1.33 5.77L10 13.9 4.74 17l1.33-5.77L1.65 7.4l5.88-.5L10 1.5z"
        />
      </svg>
      <span>{displayedRating > 0 ? `Watched ★${displayedRating}` : 'Watched'}</span>
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
        <path d="M1 12s4-7 11-7 11 7 11 7-4 7-11 7S1 12 1 12z" />
        <circle cx="12" cy="12" r="3" />
      </svg>
      <span>Mark Watched</span>
    {/if}
  </button>

  {#if watchCount > 0}
    <span class="text-xs text-gray-500" data-testid="watch-count">{watchCountLabel}</span>
  {/if}
</div>

{#if isModalOpen}
  <div class="fixed inset-0 z-50 flex items-center justify-center px-4 py-6">
    <button
      type="button"
      class="absolute inset-0 bg-black/60"
      aria-label="Close watch rating"
      on:click={closeModal}
      disabled={isSaving}
    ></button>

    <div
      bind:this={modalRef}
      class="relative z-10 w-full max-w-md rounded-xl bg-white p-6 shadow-xl"
      role="dialog"
      aria-modal="true"
      aria-label="Watch log"
      data-testid="watch-modal"
      tabindex="-1"
    >
      <div class="flex items-start justify-between gap-4">
        <div>
          <h3 class="text-lg font-semibold text-gray-900">Rate this movie</h3>
          <p class="mt-1 text-sm text-gray-500">Keep track of what you watched.</p>
        </div>
        <button
          bind:this={closeButtonRef}
          type="button"
          class="flex h-8 w-8 items-center justify-center rounded-full border border-gray-200 text-gray-500 hover:bg-gray-50"
          aria-label="Close modal"
          on:click={closeModal}
          disabled={isSaving}
        >
          x
        </button>
      </div>

      <div class="mt-5 space-y-4">
        <div>
          <p class="text-sm font-semibold text-gray-700">Rating</p>
          <div class="mt-2">
            <RatingStars
              bind:value={ratingValue}
              on:change={(event) => {
                if (event.detail > 0) {
                  formError = '';
                }
              }}
              ariaLabel="Watch rating"
            />
          </div>
          {#if ratingValue <= 0}
            <p class="mt-2 text-xs text-gray-500">Select a rating to save.</p>
          {/if}
        </div>

        <div>
          <label class="text-sm font-semibold text-gray-700" for="watch-notes">
            Notes (optional)
          </label>
          <textarea
            id="watch-notes"
            rows="3"
            class="mt-2 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-700 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
            placeholder="What stood out?"
            bind:value={notesValue}
            data-testid="watch-notes"
          ></textarea>
        </div>

        {#if formError}
          <p class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
            {formError}
          </p>
        {/if}
      </div>

      <div class="mt-6 flex flex-wrap items-center justify-between gap-3">
        {#if isWatched}
          <button
            type="button"
            class="inline-flex items-center gap-2 rounded-lg border border-red-200 px-3 py-1.5 text-sm font-semibold text-red-600 hover:bg-red-50 disabled:opacity-60"
            on:click={handleRemove}
            disabled={isSaving}
            data-testid="watch-remove"
          >
            {#if isSaving && savingAction === 'remove'}
              <svg class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 0 1 8-8V0C5.373 0 0 5.373 0 12h4z"></path>
              </svg>
              Removing...
            {:else}
              Remove
            {/if}
          </button>
        {:else}
          <span class="text-xs text-gray-500">You can update this later.</span>
        {/if}

        <div class="flex items-center gap-2">
          <button
            type="button"
            class="rounded-lg border border-gray-200 px-3 py-1.5 text-sm font-semibold text-gray-600 hover:bg-gray-50 disabled:opacity-60"
            on:click={closeModal}
            disabled={isSaving}
          >
            Cancel
          </button>
          <button
            type="button"
            class="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60"
            on:click={handleSave}
            disabled={isSaving}
            data-testid="watch-save"
          >
            {#if isSaving && savingAction === 'save'}
              <svg class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 0 1 8-8V0C5.373 0 0 5.373 0 12h4z"></path>
              </svg>
              Saving...
            {:else}
              Save
            {/if}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}

{#if toastMessage}
  <div
    class="fixed bottom-4 right-4 z-50 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm font-medium text-red-700 shadow"
    role="status"
    aria-live="polite"
    data-testid="watch-toast"
  >
    {toastMessage}
  </div>
{/if}
