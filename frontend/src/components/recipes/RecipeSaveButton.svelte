<script lang="ts">
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { api, type SavedRecipe as ApiSavedRecipe, type RecipeCategory as ApiRecipeCategory } from '../../services/api';
  import {
    recipeStore,
    savedRecipesByCategory,
    sortedCategories,
    type SavedRecipe as StoreSavedRecipe,
    type RecipeCategory as StoreRecipeCategory,
  } from '../../stores/recipeStore';
  import { postStore } from '../../stores/postStore';
  import { currentUser } from '../../stores/authStore';

  export let postId: string;

  let dropdownOpen = false;
  let showNewCategoryModal = false;
  let newCategoryName = '';
  let errorMessage: string | null = null;
  let isSaving = false;
  let isCreatingCategory = false;
  let dropdownPlacement: 'bottom' | 'top' = 'bottom';
  let triggerRef: HTMLButtonElement | null = null;
  let dropdownRef: HTMLDivElement | null = null;
  let pendingSelection = new Set<string>();

  const DROPDOWN_MIN_SPACE = 260;

  $: savedCategoryNames = getSavedCategories($savedRecipesByCategory, postId);
  $: isSaved = savedCategoryNames.length > 0;
  $: savedCategoryCount = savedCategoryNames.length;
  $: categoryNames = buildCategoryList($sortedCategories, savedCategoryNames);

  onMount(() => {
    const state = get(recipeStore);
    if (!state.isLoadingCategories && state.categories.length === 0) {
      recipeStore.loadCategories();
    }
    if (!state.isLoadingSaved && state.savedRecipes.size === 0) {
      recipeStore.loadSavedRecipes();
    }
  });

  function getSavedCategories(
    map: Map<string, StoreSavedRecipe[]>,
    targetPostId: string
  ): string[] {
    const names: string[] = [];
    for (const [category, recipes] of map.entries()) {
      if (recipes.some((recipe) => recipe.postId === targetPostId)) {
        names.push(category);
      }
    }
    return names;
  }

  function buildCategoryList(
    categories: StoreRecipeCategory[],
    savedNames: string[]
  ): string[] {
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

  function openDropdown() {
    dropdownOpen = true;
    pendingSelection = new Set(savedCategoryNames);
    errorMessage = null;
    updatePlacement();
  }

  function closeDropdown() {
    dropdownOpen = false;
  }

  function toggleDropdown() {
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

  function openCreateCategoryModal() {
    dropdownOpen = false;
    showNewCategoryModal = true;
    newCategoryName = '';
    errorMessage = null;
  }

  function closeCreateCategoryModal() {
    showNewCategoryModal = false;
  }

  async function createCategory() {
    if (isCreatingCategory) {
      return;
    }
    const trimmed = newCategoryName.trim();
    if (!trimmed) {
      errorMessage = 'Category name is required.';
      return;
    }

    isCreatingCategory = true;
    errorMessage = null;

    try {
      const response = await api.createRecipeCategory(trimmed);
      const category = mapApiRecipeCategory(response.category);
      recipeStore.applyCategory(category);
      pendingSelection = new Set([...pendingSelection, category.name]);
      showNewCategoryModal = false;
      dropdownOpen = true;
      updatePlacement();
    } catch (error) {
      errorMessage =
        error instanceof Error ? error.message : 'Failed to create category.';
    } finally {
      isCreatingCategory = false;
    }
  }

  function buildOptimisticRecipe(category: string): StoreSavedRecipe {
    const userId = get(currentUser)?.id ?? 'unknown';
    return {
      id: `temp-${postId}-${category}-${Date.now()}`,
      userId,
      postId,
      category,
      createdAt: new Date().toISOString(),
    };
  }

  function mapApiSavedRecipe(recipe: ApiSavedRecipe): StoreSavedRecipe {
    return {
      id: recipe.id,
      userId: recipe.user_id,
      postId: recipe.post_id,
      category: recipe.category,
      createdAt: recipe.created_at,
      deletedAt: recipe.deleted_at ?? undefined,
    };
  }

  function mapApiRecipeCategory(category: ApiRecipeCategory): StoreRecipeCategory {
    return {
      id: category.id,
      userId: category.user_id,
      name: category.name,
      position: category.position,
      createdAt: category.created_at,
    };
  }

  function setToArray(set: Set<string>): string[] {
    return Array.from(set.values());
  }

  function applyOptimisticChange(previous: Set<string>, next: Set<string>) {
    const toAdd = setToArray(next).filter((category) => !previous.has(category));
    const toRemove = setToArray(previous).filter((category) => !next.has(category));

    for (const category of toAdd) {
      recipeStore.applySavedRecipes([buildOptimisticRecipe(category)]);
    }

    for (const category of toRemove) {
      recipeStore.applyUnsave(postId, category);
    }

    const wasSaved = previous.size > 0;
    const willBeSaved = next.size > 0;
    if (!wasSaved && willBeSaved) {
      postStore.updateRecipeSaveCount(postId, 1);
    } else if (wasSaved && !willBeSaved) {
      postStore.updateRecipeSaveCount(postId, -1);
    }
  }

  function revertOptimisticChange(previous: Set<string>, next: Set<string>) {
    const added = setToArray(next).filter((category) => !previous.has(category));
    const removed = setToArray(previous).filter((category) => !next.has(category));

    for (const category of added) {
      recipeStore.applyUnsave(postId, category);
    }

    for (const category of removed) {
      recipeStore.applySavedRecipes([buildOptimisticRecipe(category)]);
    }

    const wasSaved = previous.size > 0;
    const willBeSaved = next.size > 0;
    if (!wasSaved && willBeSaved) {
      postStore.updateRecipeSaveCount(postId, -1);
    } else if (wasSaved && !willBeSaved) {
      postStore.updateRecipeSaveCount(postId, 1);
    }
  }

  async function persistChanges(previous: Set<string>, next: Set<string>) {
    const toAdd = setToArray(next).filter((category) => !previous.has(category));
    const toRemove = setToArray(previous).filter((category) => !next.has(category));

    if (next.size === 0 && previous.size > 0) {
      await api.unsaveRecipe(postId);
      return;
    }

    if (toAdd.length > 0) {
      const response = await api.saveRecipe(postId, toAdd);
      const savedRecipes = (response.saved_recipes ?? []).map(mapApiSavedRecipe);
      if (savedRecipes.length > 0) {
        recipeStore.applySavedRecipes(savedRecipes);
      }
    }

    if (toRemove.length > 0) {
      for (const category of toRemove) {
        await api.unsaveRecipe(postId, category);
      }
    }
  }

  async function applySelection() {
    if (isSaving) {
      return;
    }

    const previous = new Set(savedCategoryNames);
    const next = new Set(pendingSelection);

    if (previous.size === next.size && setToArray(previous).every((value) => next.has(value))) {
      closeDropdown();
      return;
    }

    isSaving = true;
    errorMessage = null;

    applyOptimisticChange(previous, next);

    try {
      await persistChanges(previous, next);
      closeDropdown();
    } catch (error) {
      errorMessage =
        error instanceof Error ? error.message : 'Failed to update saved recipes.';
      revertOptimisticChange(previous, next);
      await recipeStore.loadSavedRecipes();
    } finally {
      isSaving = false;
    }
  }

  function handleWindowClick(event: MouseEvent) {
    if (!dropdownOpen) {
      return;
    }
    const target = event.target;
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
    if (event.key !== 'Escape') {
      return;
    }
    if (showNewCategoryModal) {
      closeCreateCategoryModal();
    } else if (dropdownOpen) {
      closeDropdown();
    }
  }}
/>

<div class="relative inline-flex" data-testid="recipe-save-button">
  <button
    bind:this={triggerRef}
    type="button"
    class={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-semibold transition-colors ${
      isSaved
        ? 'border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100'
        : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50'
    }`}
    on:click|stopPropagation={toggleDropdown}
    aria-haspopup="true"
    aria-expanded={dropdownOpen}
  >
    {#if isSaved}
      <svg class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
        <path
          d="M5 2.5A1.5 1.5 0 0 1 6.5 1h7A1.5 1.5 0 0 1 15 2.5v15l-5-3-5 3v-15z"
        />
      </svg>
      <span>Saved</span>
      <span
        class="rounded-full bg-emerald-100 px-2 py-0.5 text-[11px] font-semibold text-emerald-700"
        data-testid="recipe-save-count"
      >
        {savedCategoryCount}
      </span>
    {:else}
      <svg class="h-4 w-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" aria-hidden="true">
        <path
          d="M5 2.5A1.5 1.5 0 0 1 6.5 1h7A1.5 1.5 0 0 1 15 2.5v15l-5-3-5 3v-15z"
          stroke-linejoin="round"
          stroke-width="1.5"
        />
      </svg>
      <span>Add to my recipes</span>
    {/if}
  </button>

  {#if dropdownOpen}
    <div
      bind:this={dropdownRef}
      class={`absolute right-0 z-40 w-72 max-w-[90vw] rounded-lg border border-gray-200 bg-white shadow-lg ${
        dropdownPlacement === 'top' ? 'bottom-full mb-2' : 'top-full mt-2'
      }`}
      role="dialog"
      aria-label="Select recipe categories"
    >
      <div class="px-4 py-3 border-b border-gray-100">
        <p class="text-sm font-semibold text-gray-900">Save to categories</p>
        <p class="text-xs text-gray-500">Choose where this recipe lives.</p>
      </div>

      <div class="max-h-56 overflow-y-auto px-4 py-3 space-y-2">
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
              />
              <span class="text-sm text-gray-700">{category}</span>
            </label>
          {/each}
        {/if}
      </div>

      <div class="border-t border-gray-100 px-4 py-3">
        <button
          type="button"
          class="text-xs font-semibold text-blue-600 hover:text-blue-800"
          on:click={openCreateCategoryModal}
        >
          + Create new category
        </button>
      </div>

      {#if errorMessage}
        <div class="px-4 pb-2 text-xs text-red-600" role="status" aria-live="polite">
          {errorMessage}
        </div>
      {/if}

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
          class="rounded-full bg-blue-600 px-4 py-1.5 text-xs font-semibold text-white hover:bg-blue-700 disabled:opacity-60"
          on:click={applySelection}
          disabled={isSaving}
        >
          {isSaving ? 'Saving…' : 'Apply'}
        </button>
      </div>
    </div>
  {/if}
</div>

{#if showNewCategoryModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center px-4 py-6">
    <button
      type="button"
      class="absolute inset-0 bg-black/40"
      aria-label="Close modal"
      on:click={closeCreateCategoryModal}
    ></button>
    <div
      class="relative z-10 w-full max-w-sm rounded-xl bg-white p-4 shadow-lg"
      role="dialog"
      aria-modal="true"
      aria-label="Create new category"
    >
      <h3 class="text-sm font-semibold text-gray-900">New category</h3>
      <p class="mt-1 text-xs text-gray-500">Name your category and we'll save it for future recipes.</p>
      <input
        class="mt-3 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
        placeholder="e.g. Weeknight dinners"
        bind:value={newCategoryName}
        on:keydown={(event) => {
          if (event.key === 'Enter') {
            event.preventDefault();
            createCategory();
          }
        }}
      />
      {#if errorMessage}
        <p class="mt-2 text-xs text-red-600" role="status" aria-live="polite">
          {errorMessage}
        </p>
      {/if}
      <div class="mt-4 flex items-center justify-end gap-2">
        <button
          type="button"
          class="text-xs font-semibold text-gray-600 hover:text-gray-800"
          on:click={closeCreateCategoryModal}
          disabled={isCreatingCategory}
        >
          Cancel
        </button>
        <button
          type="button"
          class="rounded-full bg-blue-600 px-4 py-1.5 text-xs font-semibold text-white hover:bg-blue-700 disabled:opacity-60"
          on:click={createCategory}
          disabled={isCreatingCategory}
        >
          {isCreatingCategory ? 'Creating…' : 'Create'}
        </button>
      </div>
    </div>
  </div>
{/if}
