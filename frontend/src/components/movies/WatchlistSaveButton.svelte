<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import { get } from 'svelte/store';
  import {
    api,
    type WatchlistCategory as ApiWatchlistCategory,
    type WatchlistItem as ApiWatchlistItem,
  } from '../../services/api';
  import {
    movieStore,
    sortedCategories,
    watchlistByCategory,
    type WatchlistCategory as StoreWatchlistCategory,
    type WatchlistItem as StoreWatchlistItem,
  } from '../../stores/movieStore';
  import { postStore } from '../../stores/postStore';
  import { currentUser } from '../../stores/authStore';

  export let postId: string;
  export let initialSaved: boolean = false;
  export let initialCategories: string[] = [];
  export let saveCount: number = 0;

  let dropdownOpen = false;
  let showCreateCategoryInline = false;
  let newCategoryName = '';
  let createCategoryError: string | null = null;
  let isSaving = false;
  let isCreatingCategory = false;
  let dropdownPlacement: 'bottom' | 'top' = 'bottom';
  let triggerRef: HTMLButtonElement | null = null;
  let dropdownRef: HTMLDivElement | null = null;
  let newCategoryInputRef: HTMLInputElement | null = null;
  let pendingSelection = new Set<string>();
  let toastMessage: string | null = null;
  let lastToggleAt = 0;
  let hasRequestedWatchlistLoad = false;
  let hasWatchlistLoadSettled = false;
  let fallbackSaved = initialSaved || initialCategories.length > 0;
  let fallbackSelection = new Set(
    initialCategories.map((category) => category.trim()).filter((category) => category.length > 0)
  );
  let localSaveCount = Math.max(0, Number.isFinite(saveCount) ? saveCount : 0);
  let toastTimeout: ReturnType<typeof setTimeout> | null = null;

  const DROPDOWN_MIN_SPACE = 260;
  const TOGGLE_DEBOUNCE_MS = 180;

  $: storeSavedCategoryNames = getSavedCategories($watchlistByCategory, postId);
  $: storeReady =
    $watchlistByCategory.size > 0 ||
    (hasWatchlistLoadSettled && !$movieStore.isLoadingWatchlist && !$movieStore.error);
  $: effectiveSavedCategories = storeReady ? storeSavedCategoryNames : Array.from(fallbackSelection);
  $: isSaved = storeReady ? storeSavedCategoryNames.length > 0 : fallbackSaved;
  $: categoryNames = buildCategoryList($sortedCategories, effectiveSavedCategories);
  $: displayedSaveCount = Math.max(0, localSaveCount);

  onMount(() => {
    const state = get(movieStore);

    if (!state.isLoadingCategories && state.categories.length === 0) {
      void movieStore.loadWatchlistCategories();
    }

    if (!state.isLoadingWatchlist && state.watchlist.size === 0) {
      hasRequestedWatchlistLoad = true;
      void movieStore.loadWatchlist().finally(() => {
        hasWatchlistLoadSettled = true;
      });
    } else if (!state.isLoadingWatchlist) {
      hasWatchlistLoadSettled = true;
    }
  });

  onDestroy(() => {
    if (toastTimeout) {
      clearTimeout(toastTimeout);
    }
  });

  $: if (hasRequestedWatchlistLoad && !$movieStore.isLoadingWatchlist) {
    hasWatchlistLoadSettled = true;
  }

  function getSavedCategories(map: Map<string, StoreWatchlistItem[]>, targetPostId: string): string[] {
    const names: string[] = [];

    for (const [category, items] of map.entries()) {
      if (items.some((item) => item.postId === targetPostId)) {
        names.push(category);
      }
    }

    return names;
  }

  function buildCategoryList(categories: StoreWatchlistCategory[], savedNames: string[]): string[] {
    const seen = new Set<string>();
    const result: string[] = [];

    for (const category of categories) {
      if (!seen.has(category.name)) {
        result.push(category.name);
        seen.add(category.name);
      }
    }

    for (const name of savedNames) {
      if (!seen.has(name)) {
        result.push(name);
        seen.add(name);
      }
    }

    return result;
  }

  function updatePlacement() {
    if (!triggerRef || typeof window === 'undefined') {
      return;
    }

    const rect = triggerRef.getBoundingClientRect();
    const spaceBelow = window.innerHeight - rect.bottom;
    dropdownPlacement = spaceBelow < DROPDOWN_MIN_SPACE ? 'top' : 'bottom';
  }

  function showToast(message: string) {
    toastMessage = message;

    if (toastTimeout) {
      clearTimeout(toastTimeout);
    }

    toastTimeout = setTimeout(() => {
      toastMessage = null;
      toastTimeout = null;
    }, 3500);
  }

  function openDropdown() {
    dropdownOpen = true;
    pendingSelection = new Set(effectiveSavedCategories);
    showCreateCategoryInline = false;
    newCategoryName = '';
    createCategoryError = null;
    updatePlacement();
  }

  function closeDropdown() {
    dropdownOpen = false;
    showCreateCategoryInline = false;
    createCategoryError = null;
  }

  function toggleDropdown() {
    const now = Date.now();
    if (now - lastToggleAt < TOGGLE_DEBOUNCE_MS) {
      return;
    }
    lastToggleAt = now;

    if (dropdownOpen) {
      closeDropdown();
      return;
    }

    openDropdown();
  }

  function toggleCategory(category: string) {
    const next = new Set(pendingSelection);
    if (next.has(category)) {
      next.delete(category);
    } else {
      next.add(category);
    }
    pendingSelection = next;
  }

  async function openInlineCategoryCreate() {
    showCreateCategoryInline = true;
    newCategoryName = '';
    createCategoryError = null;
    await tick();
    newCategoryInputRef?.focus();
  }

  function cancelInlineCategoryCreate() {
    showCreateCategoryInline = false;
    newCategoryName = '';
    createCategoryError = null;
  }

  function setToArray(set: Set<string>): string[] {
    return Array.from(set.values());
  }

  function updateFallbackState(next: Set<string>) {
    fallbackSelection = new Set(next);
    fallbackSaved = next.size > 0;
  }

  function buildOptimisticWatchlistItem(category: string): StoreWatchlistItem {
    const userId = get(currentUser)?.id ?? 'unknown';
    return {
      id: `temp-${postId}-${category}-${Date.now()}`,
      userId,
      postId,
      category,
      createdAt: new Date().toISOString(),
    };
  }

  function mapApiWatchlistItem(item: ApiWatchlistItem): StoreWatchlistItem {
    return {
      id: item.id,
      userId: item.userId,
      postId: item.postId,
      category: item.category,
      createdAt: item.createdAt,
    };
  }

  function mapApiWatchlistCategory(category: ApiWatchlistCategory): StoreWatchlistCategory {
    return {
      id: category.id,
      name: category.name,
      position: category.position,
    };
  }

  function applyOptimisticChange(previous: Set<string>, next: Set<string>) {
    const toAdd = setToArray(next).filter((category) => !previous.has(category));
    const toRemove = setToArray(previous).filter((category) => !next.has(category));

    for (const category of toAdd) {
      movieStore.applyWatchlistItems([buildOptimisticWatchlistItem(category)]);
    }

    for (const category of toRemove) {
      movieStore.applyUnwatchlist(postId, category);
    }

    const wasSaved = previous.size > 0;
    const willBeSaved = next.size > 0;
    if (!wasSaved && willBeSaved) {
      localSaveCount = Math.max(0, localSaveCount + 1);
    } else if (wasSaved && !willBeSaved) {
      localSaveCount = Math.max(0, localSaveCount - 1);
    }

    postStore.setMovieWatchlistState(postId, willBeSaved, setToArray(next));
    updateFallbackState(next);
  }

  function revertOptimisticChange(previous: Set<string>, next: Set<string>) {
    const added = setToArray(next).filter((category) => !previous.has(category));
    const removed = setToArray(previous).filter((category) => !next.has(category));

    for (const category of added) {
      movieStore.applyUnwatchlist(postId, category);
    }

    for (const category of removed) {
      movieStore.applyWatchlistItems([buildOptimisticWatchlistItem(category)]);
    }

    const wasSaved = previous.size > 0;
    const willBeSaved = next.size > 0;
    if (!wasSaved && willBeSaved) {
      localSaveCount = Math.max(0, localSaveCount - 1);
    } else if (wasSaved && !willBeSaved) {
      localSaveCount = Math.max(0, localSaveCount + 1);
    }

    postStore.setMovieWatchlistState(postId, previous.size > 0, setToArray(previous));
    updateFallbackState(previous);
  }

  async function persistChanges(previous: Set<string>, next: Set<string>) {
    const toAdd = setToArray(next).filter((category) => !previous.has(category));
    const toRemove = setToArray(previous).filter((category) => !next.has(category));

    if (next.size === 0 && previous.size > 0) {
      await api.removeFromWatchlist(postId);
      return;
    }

    if (toAdd.length > 0) {
      const response = await api.addToWatchlist(postId, toAdd);
      const watchlistItems = (response.watchlistItems ?? []).map(mapApiWatchlistItem);
      if (watchlistItems.length > 0) {
        movieStore.applyWatchlistItems(watchlistItems);
      }
    }

    if (toRemove.length > 0) {
      for (const category of toRemove) {
        await api.removeFromWatchlist(postId, category);
      }
    }
  }

  async function applySelection() {
    if (isSaving) {
      return;
    }

    const previous = new Set(effectiveSavedCategories);
    const next = new Set(pendingSelection);

    if (previous.size === next.size && setToArray(previous).every((value) => next.has(value))) {
      closeDropdown();
      return;
    }

    isSaving = true;

    applyOptimisticChange(previous, next);

    try {
      await persistChanges(previous, next);
      closeDropdown();
    } catch (error) {
      const message =
        error instanceof Error ? error.message : 'Failed to update watchlist. Please try again.';
      showToast(message);
      revertOptimisticChange(previous, next);
      pendingSelection = new Set(previous);
      await movieStore.loadWatchlist();
    } finally {
      isSaving = false;
    }
  }

  async function createCategory() {
    if (isCreatingCategory) {
      return;
    }

    const trimmed = newCategoryName.trim();
    if (!trimmed) {
      createCategoryError = 'Category name is required.';
      return;
    }

    const alreadyExists = categoryNames.some(
      (category) => category.toLowerCase() === trimmed.toLowerCase()
    );
    if (alreadyExists) {
      createCategoryError = 'Category already exists.';
      return;
    }

    isCreatingCategory = true;
    createCategoryError = null;

    try {
      const response = await api.createWatchlistCategory(trimmed);
      movieStore.applyCategory(mapApiWatchlistCategory(response.category));
      pendingSelection = new Set([...pendingSelection, response.category.name]);
      showCreateCategoryInline = false;
      newCategoryName = '';
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to create category.';
      createCategoryError = message;
      showToast(message);
    } finally {
      isCreatingCategory = false;
    }
  }

  function handleWindowClick(event: MouseEvent) {
    if (!dropdownOpen) {
      return;
    }

    const eventPath = typeof event.composedPath === 'function' ? event.composedPath() : [];
    if (triggerRef && eventPath.includes(triggerRef)) {
      return;
    }
    if (dropdownRef && eventPath.includes(dropdownRef)) {
      return;
    }

    const target = event.target;
    if (triggerRef && target instanceof Node && triggerRef.contains(target)) {
      return;
    }
    if (dropdownRef && target instanceof Node && dropdownRef.contains(target)) {
      return;
    }

    closeDropdown();
  }
</script>

<svelte:window
  on:click={handleWindowClick}
  on:resize={() => {
    if (dropdownOpen) {
      updatePlacement();
    }
  }}
  on:keydown={(event) => {
    if (event.key === 'Escape') {
      if (showCreateCategoryInline) {
        cancelInlineCategoryCreate();
        return;
      }
      if (dropdownOpen) {
        closeDropdown();
      }
    }
  }}
/>

<div class="relative inline-flex" data-testid="watchlist-save-button">
  <button
    bind:this={triggerRef}
    type="button"
    class={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-semibold transition-colors ${
      isSaved
        ? 'border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100'
        : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50'
    }`}
    on:click|stopPropagation={toggleDropdown}
    on:keydown={(event) => {
      if (event.key !== 'Enter') {
        return;
      }
      event.preventDefault();
      toggleDropdown();
    }}
    aria-label={isSaved ? 'Edit watchlist categories' : 'Add movie to watchlist'}
    aria-haspopup="dialog"
    aria-expanded={dropdownOpen}
    disabled={isSaving}
  >
    {#if isSaved}
      <svg class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
        <path d="M16.707 6.707a1 1 0 0 0-1.414-1.414L8.5 12.086 5.707 9.293a1 1 0 0 0-1.414 1.414l3.5 3.5a1 1 0 0 0 1.414 0l7.5-7.5Z" />
      </svg>
      <span>In List</span>
    {:else}
      <svg class="h-4 w-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" aria-hidden="true">
        <path d="M10 4v12M4 10h12" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" />
      </svg>
      <span>Add to List</span>
    {/if}

    {#if displayedSaveCount > 0}
      <span
        class={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${
          isSaved ? 'bg-emerald-100 text-emerald-700' : 'bg-gray-100 text-gray-700'
        }`}
        data-testid="watchlist-save-count"
      >
        {displayedSaveCount}
      </span>
    {/if}
  </button>

  {#if dropdownOpen}
    <div
      bind:this={dropdownRef}
      class={`absolute right-0 z-40 w-72 max-w-[90vw] rounded-lg border border-gray-200 bg-white shadow-lg ${
        dropdownPlacement === 'top' ? 'bottom-full mb-2' : 'top-full mt-2'
      }`}
      role="dialog"
      aria-label="Select watchlist categories"
    >
      <div class="border-b border-gray-100 px-4 py-3">
        <p class="text-sm font-semibold text-gray-900">Save to watchlist</p>
        <p class="text-xs text-gray-500">Choose one or more categories.</p>
      </div>

      <div class="max-h-56 space-y-2 overflow-y-auto px-4 py-3">
        {#if categoryNames.length === 0}
          <p class="text-xs text-gray-500">No categories yet.</p>
        {:else}
          {#each categoryNames as category}
            <label class="flex items-center gap-2 rounded-md px-2 py-1 hover:bg-gray-50">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                checked={pendingSelection.has(category)}
                on:change={() => toggleCategory(category)}
                aria-label={category}
              />
              <span class="text-sm text-gray-700">{category}</span>
            </label>
          {/each}
        {/if}
      </div>

      <div class="border-t border-gray-100 px-4 py-3">
        {#if showCreateCategoryInline}
          <div class="space-y-2" data-testid="watchlist-new-category-inline">
            <input
              bind:this={newCategoryInputRef}
              class="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              placeholder="New category name"
              bind:value={newCategoryName}
              aria-label="New category name"
              on:keydown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault();
                  createCategory();
                  return;
                }
                if (event.key === 'Escape') {
                  event.preventDefault();
                  event.stopPropagation();
                  cancelInlineCategoryCreate();
                }
              }}
            />
            {#if createCategoryError}
              <p class="text-xs text-red-600" role="status" aria-live="polite">{createCategoryError}</p>
            {/if}
            <div class="flex items-center justify-end gap-2">
              <button
                type="button"
                class="text-xs font-semibold text-gray-600 hover:text-gray-800"
                on:click={cancelInlineCategoryCreate}
                disabled={isCreatingCategory}
              >
                Cancel
              </button>
              <button
                type="button"
                class="rounded-full bg-blue-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-blue-700 disabled:opacity-60"
                on:click={createCategory}
                disabled={isCreatingCategory}
              >
                {isCreatingCategory ? 'Creating…' : 'Create'}
              </button>
            </div>
          </div>
        {:else}
          <button
            type="button"
            class="text-xs font-semibold text-blue-600 hover:text-blue-800"
            on:click={openInlineCategoryCreate}
          >
            + Create category
          </button>
        {/if}
      </div>

      <div class="flex items-center justify-between gap-2 border-t border-gray-100 px-4 py-3">
        <button
          type="button"
          class="text-xs font-semibold text-gray-600 hover:text-gray-800"
          on:click={closeDropdown}
          disabled={isSaving}
        >
          Cancel
        </button>
        <button
          type="button"
          class="inline-flex items-center gap-2 rounded-full bg-blue-600 px-4 py-1.5 text-xs font-semibold text-white hover:bg-blue-700 disabled:opacity-60"
          on:click={applySelection}
          disabled={isSaving}
        >
          {#if isSaving}
            <span class="h-3 w-3 animate-spin rounded-full border border-white border-t-transparent" aria-hidden="true"></span>
            <span>Saving…</span>
          {:else}
            <span>Apply</span>
          {/if}
        </button>
      </div>
    </div>
  {/if}

  {#if toastMessage}
    <div
      class="pointer-events-none absolute -bottom-12 right-0 z-50 max-w-xs rounded-lg bg-red-600 px-3 py-2 text-xs font-medium text-white shadow-md"
      role="alert"
      aria-live="assertive"
      data-testid="watchlist-error-toast"
    >
      {toastMessage}
    </div>
  {/if}
</div>
