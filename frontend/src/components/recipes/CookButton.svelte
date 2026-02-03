<script lang="ts">
  import { get } from 'svelte/store';
  import { recipeStore, type CookLog } from '../../stores/recipeStore';
  import RatingStars from './RatingStars.svelte';

  export let postId: string;

  let isModalOpen = false;
  let ratingValue = 0;
  let notesValue = '';
  let isSaving = false;
  let formError = '';
  let optimisticCookLog: CookLog | null | undefined = undefined;

  $: storedCookLog = $recipeStore.cookLogs.find((log) => log.postId === postId) ?? null;
  $: activeCookLog = optimisticCookLog !== undefined ? optimisticCookLog : storedCookLog;
  $: isCooked = Boolean(activeCookLog);

  function openModal() {
    if (isSaving) {
      return;
    }

    ratingValue = activeCookLog?.rating ?? 0;
    notesValue = activeCookLog?.notes ?? '';
    formError = '';
    isModalOpen = true;
  }

  function closeModal() {
    if (isSaving) {
      return;
    }

    isModalOpen = false;
    formError = '';
  }

  function buildOptimisticLog(): CookLog {
    const now = new Date().toISOString();
    return {
      id: activeCookLog?.id ?? 'optimistic',
      userId: activeCookLog?.userId ?? 'me',
      postId,
      rating: ratingValue,
      notes: notesValue.trim() ? notesValue.trim() : null,
      createdAt: activeCookLog?.createdAt ?? now,
      updatedAt: now,
      deletedAt: null,
      post: activeCookLog?.post,
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
    formError = '';

    const previous = activeCookLog;
    optimisticCookLog = buildOptimisticLog();

    recipeStore.setError(null);

    const notes = notesValue.trim();

    try {
      if (previous) {
        await recipeStore.updateCookLog(postId, ratingValue, notes || undefined);
      } else {
        await recipeStore.logCook(postId, ratingValue, notes || undefined);
      }
    } finally {
      const { error } = get(recipeStore);
      if (error) {
        optimisticCookLog = previous ?? null;
        formError = error;
      } else {
        optimisticCookLog = undefined;
        isModalOpen = false;
      }
      isSaving = false;
    }
  }

  async function handleRemove() {
    if (!activeCookLog || isSaving) {
      return;
    }

    isSaving = true;
    formError = '';

    const previous = activeCookLog;
    optimisticCookLog = null;

    recipeStore.setError(null);

    try {
      await recipeStore.removeCookLog(postId);
    } finally {
      const { error } = get(recipeStore);
      if (error) {
        optimisticCookLog = previous;
        formError = error;
      } else {
        optimisticCookLog = undefined;
        isModalOpen = false;
      }
      isSaving = false;
    }
  }
</script>

<svelte:window
  on:keydown={(event) => {
    if (!isModalOpen) {
      return;
    }

    if (event.key === 'Escape') {
      event.stopPropagation();
      closeModal();
    }
  }}
/>

{#if isCooked}
  <div class="flex flex-wrap items-center gap-2" data-testid="cook-status">
    <div class="inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700">
      <span class="inline-flex items-center gap-1">
        <svg
          viewBox="0 0 20 20"
          class="h-4 w-4"
          aria-hidden="true"
          fill="currentColor"
        >
          <path
            d="M7.5 13.4L3.6 9.6a1 1 0 0 1 1.4-1.4l2.5 2.5 7.1-7.1a1 1 0 1 1 1.4 1.4l-8 8a1 1 0 0 1-1.4 0z"
          />
        </svg>
        Cooked
      </span>
      <RatingStars value={activeCookLog?.rating ?? 0} readonly size="sm" ariaLabel="Cooked rating" />
    </div>
    <button
      type="button"
      class="text-xs font-semibold text-blue-600 hover:text-blue-700 hover:underline"
      on:click={openModal}
      data-testid="cook-edit"
    >
      Edit
    </button>
  </div>
{:else}
  <button
    type="button"
    class="inline-flex items-center gap-2 rounded-full border border-gray-200 bg-white px-3 py-1 text-xs font-semibold text-gray-700 hover:border-gray-300 hover:bg-gray-50"
    on:click={openModal}
    data-testid="cook-button"
  >
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
      <path d="M3 9h18" />
      <path d="M5 9l1 9h12l1-9" />
      <path d="M9 4h6" />
      <path d="M10 4v3" />
      <path d="M14 4v3" />
    </svg>
    I cooked this
  </button>
{/if}

{#if isModalOpen}
  <div class="fixed inset-0 z-50 flex items-center justify-center px-4 py-6">
    <button
      type="button"
      class="absolute inset-0 bg-black/60"
      aria-label="Close cook rating"
      on:click={closeModal}
      disabled={isSaving}
    ></button>
    <div
      class="relative z-10 w-full max-w-md rounded-xl bg-white p-6 shadow-xl"
      role="dialog"
      aria-modal="true"
      aria-label="Cook log"
      data-testid="cook-modal"
    >
      <div class="flex items-start justify-between gap-4">
        <div>
          <h3 class="text-lg font-semibold text-gray-900">Rate this cook</h3>
          <p class="mt-1 text-sm text-gray-500">Share how it turned out.</p>
        </div>
        <button
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
              value={ratingValue}
              onChange={(value) => {
                ratingValue = value;
                if (value > 0) {
                  formError = '';
                }
              }}
              ariaLabel="Cook rating"
            />
          </div>
          {#if ratingValue <= 0}
            <p class="mt-2 text-xs text-gray-500">Select a rating to save.</p>
          {/if}
        </div>

        <div>
          <label class="text-sm font-semibold text-gray-700" for="cook-notes">
            Notes (optional)
          </label>
          <textarea
            id="cook-notes"
            rows="3"
            class="mt-2 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-700 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
            placeholder="Add any tips or changes you made"
            bind:value={notesValue}
            data-testid="cook-notes"
          ></textarea>
        </div>

        {#if formError}
          <p class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
            {formError}
          </p>
        {/if}
      </div>

      <div class="mt-6 flex flex-wrap items-center justify-between gap-3">
        {#if isCooked}
          <button
            type="button"
            class="rounded-lg border border-red-200 px-3 py-1.5 text-sm font-semibold text-red-600 hover:bg-red-50 disabled:opacity-60"
            on:click={handleRemove}
            disabled={isSaving}
            data-testid="cook-remove"
          >
            Remove
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
            class="rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60"
            on:click={handleSave}
            disabled={isSaving}
            data-testid="cook-save"
          >
            {isSaving ? 'Saving...' : 'Save'}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
