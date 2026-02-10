<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { get } from 'svelte/store';
  import {
    bookStore,
    bookStoreMeta,
    bookshelfCategories,
    myBookshelf,
  } from '../../stores/bookStore';
  import type { BookshelfItem } from '../../services/api';
  import type { BookStats } from '../../stores/postStore';

  export let postId: string;
  export let bookStats: BookStats;

  let dropdownOpen = false;
  let showCreateCategoryInline = false;
  let newCategoryName = '';
  let createCategoryError: string | null = null;
  let isSaving = false;
  let isCreatingCategory = false;
  let dropdownRef: HTMLDivElement | null = null;
  let controlsRef: HTMLDivElement | null = null;
  let newCategoryInputRef: HTMLInputElement | null = null;
  let pendingSelection = new Set<string>();
  let hasRequestedBookshelfLoad = false;
  let hasBookshelfLoadSettled = false;
  let fallbackSaved = false;
  let fallbackSelection = new Set<string>();

  $: initialCategories = normalizeCategoryList(bookStats?.viewerCategories ?? []);
  $: if (!hasBookshelfLoadSettled && !dropdownOpen && !isSaving) {
    fallbackSelection = new Set(initialCategories);
    fallbackSaved = Boolean(bookStats?.viewerOnBookshelf) || initialCategories.length > 0;
  }

  $: storeSavedCategoryNames = getSavedCategories($myBookshelf, postId);
  $: storeReady = $myBookshelf.size > 0 || hasBookshelfLoadSettled;
  $: effectiveSavedCategories = storeReady ? storeSavedCategoryNames : Array.from(fallbackSelection);
  $: isSaved = storeReady ? storeSavedCategoryNames.length > 0 : fallbackSaved;
  $: categoryNames = buildCategoryList(
    $bookshelfCategories.map((category) => category.name),
    effectiveSavedCategories
  );

  onMount(() => {
    const meta = get(bookStoreMeta);
    const categories = get(bookshelfCategories);
    const bookshelf = get(myBookshelf);

    if (!meta.loading.categories && categories.length === 0) {
      void bookStore.loadBookshelfCategories();
    }

    if (!meta.loading.myBookshelf && bookshelf.size === 0) {
      hasRequestedBookshelfLoad = true;
      void bookStore.loadMyBookshelf().finally(() => {
        hasBookshelfLoadSettled = true;
      });
    } else if (!meta.loading.myBookshelf) {
      hasBookshelfLoadSettled = true;
    }
  });

  $: if (hasRequestedBookshelfLoad && !$bookStoreMeta.loading.myBookshelf) {
    hasBookshelfLoadSettled = true;
  }

  function normalizeCategoryList(categories: string[]): string[] {
    const seen = new Set<string>();
    const normalized: string[] = [];

    for (const category of categories) {
      const trimmed = category.trim();
      if (!trimmed) {
        continue;
      }
      if (seen.has(trimmed.toLowerCase())) {
        continue;
      }
      seen.add(trimmed.toLowerCase());
      normalized.push(trimmed);
    }

    return normalized;
  }

  function getSavedCategories(map: Map<string, BookshelfItem[]>, targetPostId: string): string[] {
    const saved: string[] = [];

    for (const [categoryName, items] of map.entries()) {
      if (items.some((item) => item.postId === targetPostId)) {
        saved.push(categoryName);
      }
    }

    return saved;
  }

  function buildCategoryList(existing: string[], selected: string[]): string[] {
    const seen = new Set<string>();
    const merged: string[] = [];

    for (const category of existing) {
      const normalized = category.trim();
      if (!normalized) {
        continue;
      }
      const key = normalized.toLowerCase();
      if (seen.has(key)) {
        continue;
      }
      seen.add(key);
      merged.push(normalized);
    }

    for (const category of selected) {
      const normalized = category.trim();
      if (!normalized) {
        continue;
      }
      const key = normalized.toLowerCase();
      if (seen.has(key)) {
        continue;
      }
      seen.add(key);
      merged.push(normalized);
    }

    return merged;
  }

  function setsEqual(left: Set<string>, right: Set<string>): boolean {
    if (left.size !== right.size) {
      return false;
    }
    for (const value of left) {
      if (!right.has(value)) {
        return false;
      }
    }
    return true;
  }

  function updateFallbackState(selection: Set<string>) {
    fallbackSelection = new Set(selection);
    fallbackSaved = selection.size > 0;
  }

  function openDropdown() {
    dropdownOpen = true;
    showCreateCategoryInline = false;
    newCategoryName = '';
    createCategoryError = null;
    pendingSelection = new Set(effectiveSavedCategories);
  }

  async function applySelectionIfChanged() {
    const previous = new Set(effectiveSavedCategories);
    const next = new Set(pendingSelection);

    if (setsEqual(previous, next)) {
      return;
    }

    isSaving = true;
    try {
      if (next.size === 0) {
        await bookStore.removeFromBookshelf(postId);
      } else {
        await bookStore.addToBookshelf(postId, Array.from(next));
      }
      updateFallbackState(next);
    } finally {
      isSaving = false;
    }
  }

  async function closeDropdown({ apply = true }: { apply?: boolean } = {}) {
    if (!dropdownOpen) {
      return;
    }

    if (apply) {
      await applySelectionIfChanged();
    }

    dropdownOpen = false;
    showCreateCategoryInline = false;
    newCategoryName = '';
    createCategoryError = null;
  }

  function toggleDropdown() {
    if (isSaving) {
      return;
    }

    if (dropdownOpen) {
      void closeDropdown();
      return;
    }

    openDropdown();
  }

  function toggleCategory(categoryName: string) {
    if (pendingSelection.has(categoryName)) {
      pendingSelection = new Set<string>();
      return;
    }
    pendingSelection = new Set<string>([categoryName]);
  }

  async function toggleSave() {
    if (isSaving) {
      return;
    }

    isSaving = true;
    try {
      if (isSaved) {
        await bookStore.removeFromBookshelf(postId);
        updateFallbackState(new Set<string>());
      } else {
        const nextSelection = new Set<string>(
          effectiveSavedCategories.length > 0 ? effectiveSavedCategories : []
        );
        await bookStore.addToBookshelf(postId, Array.from(nextSelection));
        updateFallbackState(nextSelection);
      }
    } finally {
      isSaving = false;
    }
  }

  async function openInlineCategoryCreate() {
    showCreateCategoryInline = true;
    createCategoryError = null;
    newCategoryName = '';
    await tick();
    newCategoryInputRef?.focus();
  }

  function cancelInlineCategoryCreate() {
    showCreateCategoryInline = false;
    createCategoryError = null;
    newCategoryName = '';
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

    const duplicate = categoryNames.some((entry) => entry.toLowerCase() === trimmed.toLowerCase());
    if (duplicate) {
      createCategoryError = 'Category already exists.';
      return;
    }

    isCreatingCategory = true;
    createCategoryError = null;
    await bookStore.createCategory(trimmed);

    const createdCategory = get(bookshelfCategories).find(
      (entry) => entry.name.toLowerCase() === trimmed.toLowerCase()
    );

    if (!createdCategory) {
      createCategoryError = get(bookStoreMeta).error ?? 'Failed to create category.';
      isCreatingCategory = false;
      return;
    }

    pendingSelection = new Set([...pendingSelection, createdCategory.name]);
    showCreateCategoryInline = false;
    newCategoryName = '';
    isCreatingCategory = false;
  }

  function handleWindowClick(event: MouseEvent) {
    if (!dropdownOpen) {
      return;
    }

    const eventPath = typeof event.composedPath === 'function' ? event.composedPath() : [];
    if (controlsRef && eventPath.includes(controlsRef)) {
      return;
    }
    if (dropdownRef && eventPath.includes(dropdownRef)) {
      return;
    }

    const target = event.target;
    if (controlsRef && target instanceof Node && controlsRef.contains(target)) {
      return;
    }
    if (dropdownRef && target instanceof Node && dropdownRef.contains(target)) {
      return;
    }

    void closeDropdown();
  }

  function handleWindowKeydown(event: KeyboardEvent) {
    if (event.key !== 'Escape') {
      return;
    }

    if (showCreateCategoryInline) {
      cancelInlineCategoryCreate();
      return;
    }

    if (dropdownOpen) {
      void closeDropdown();
    }
  }
</script>

<svelte:window on:click={handleWindowClick} on:keydown={handleWindowKeydown} />

<div class="relative inline-flex" bind:this={controlsRef} data-testid="bookshelf-save-button">
  <button
    type="button"
    class={`inline-flex items-center gap-2 rounded-l-full border border-r-0 px-3 py-1.5 text-xs font-semibold transition-colors ${
      isSaved
        ? 'border-emerald-300 bg-emerald-50 text-emerald-700 hover:bg-emerald-100'
        : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50'
    }`}
    on:click|stopPropagation={toggleSave}
    aria-label={isSaved ? 'Remove from bookshelf' : 'Save to bookshelf'}
    disabled={isSaving}
  >
    {#if isSaving}
      <span
        class="h-4 w-4 animate-spin rounded-full border border-current border-t-transparent"
        data-testid="bookshelf-save-spinner"
        aria-hidden="true"
      ></span>
    {:else if isSaved}
      <svg class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
        <path d="M5 2.75A1.75 1.75 0 0 0 3.25 4.5v12.18a.5.5 0 0 0 .82.384L10 11.85l5.93 5.213a.5.5 0 0 0 .82-.384V4.5A1.75 1.75 0 0 0 15 2.75H5Z" />
      </svg>
    {:else}
      <svg class="h-4 w-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" aria-hidden="true">
        <path
          d="M5 3h10a1 1 0 0 1 1 1v13l-6-4-6 4V4a1 1 0 0 1 1-1Z"
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="1.5"
        />
      </svg>
    {/if}

    <span>{isSaved ? 'Saved' : 'Save'}</span>
  </button>

  <button
    type="button"
    class={`inline-flex items-center rounded-r-full border px-2 py-1.5 transition-colors ${
      isSaved
        ? 'border-emerald-300 bg-emerald-50 text-emerald-700 hover:bg-emerald-100'
        : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50'
    }`}
    on:click|stopPropagation={toggleDropdown}
    aria-label={dropdownOpen ? 'Close bookshelf categories' : 'Open bookshelf categories'}
    aria-haspopup="dialog"
    aria-expanded={dropdownOpen}
    disabled={isSaving}
    data-testid="bookshelf-dropdown-toggle"
  >
    <svg
      class={`h-4 w-4 transition-transform ${dropdownOpen ? 'rotate-180' : ''}`}
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path d="M5.23 7.21a.75.75 0 0 1 1.06.02L10 11.173l3.71-3.94a.75.75 0 1 1 1.08 1.04l-4.25 4.51a.75.75 0 0 1-1.08 0l-4.25-4.51a.75.75 0 0 1 .02-1.06Z" />
    </svg>
  </button>

  {#if dropdownOpen}
    <div
      bind:this={dropdownRef}
      class="absolute right-0 top-full z-40 mt-2 w-72 max-w-[90vw] rounded-lg border border-gray-200 bg-white shadow-lg"
      role="dialog"
      aria-label="Select bookshelf categories"
    >
      <div class="border-b border-gray-100 px-4 py-3">
        <p class="text-sm font-semibold text-gray-900">Save to bookshelf</p>
        <p class="text-xs text-gray-500">Select a category. Changes apply when this menu closes.</p>
      </div>

      <div class="max-h-56 space-y-2 overflow-y-auto px-4 py-3">
        {#if categoryNames.length === 0}
          <p class="text-xs text-gray-500">No categories yet.</p>
        {:else}
          {#each categoryNames as category}
            <label class="flex items-center gap-2 rounded-md px-2 py-1 hover:bg-gray-50">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-emerald-600 focus:ring-emerald-500"
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
          <div class="space-y-2" data-testid="bookshelf-new-category-inline">
            <input
              bind:this={newCategoryInputRef}
              class="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="New category name"
              bind:value={newCategoryName}
              aria-label="New category name"
              on:keydown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault();
                  void createCategory();
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
                class="rounded-full bg-emerald-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-emerald-700 disabled:opacity-60"
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
            class="text-xs font-semibold text-emerald-600 hover:text-emerald-800"
            on:click={openInlineCategoryCreate}
          >
            + Create category
          </button>
        {/if}
      </div>
    </div>
  {/if}
</div>
