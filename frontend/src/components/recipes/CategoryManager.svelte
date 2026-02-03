<script lang="ts">
  import type { RecipeCategory, SavedRecipe } from '../../stores/recipeStore';
  import {
    recipeStore,
    sortedCategories,
    savedRecipesByCategory,
  } from '../../stores/recipeStore';

  let isAdding = false;
  let newCategoryName = '';
  let editingCategoryId: string | null = null;
  let editingCategoryName = '';
  let deleteTarget: RecipeCategory | null = null;
  let localError: string | null = null;
  let isBusy = false;
  let isReordering = false;

  $: categories = $sortedCategories;
  $: recipeCounts = new Map<string, number>(
    Array.from($savedRecipesByCategory.entries()).map(([name, recipes]) => [
      name,
      (recipes as SavedRecipe[]).length,
    ])
  );
  $: displayError = localError ?? $recipeStore.error;

  function resetLocalError() {
    localError = null;
  }

  function startAdd() {
    isAdding = true;
    newCategoryName = '';
    resetLocalError();
  }

  function cancelAdd() {
    isAdding = false;
    newCategoryName = '';
    resetLocalError();
  }

  async function confirmAdd() {
    const name = newCategoryName.trim();
    if (!name) {
      localError = 'Category name is required.';
      return;
    }

    isBusy = true;
    try {
      await recipeStore.createCategory(name);
      isAdding = false;
      newCategoryName = '';
      resetLocalError();
    } finally {
      isBusy = false;
    }
  }

  function startEdit(category: RecipeCategory) {
    editingCategoryId = category.id;
    editingCategoryName = category.name;
    resetLocalError();
  }

  function cancelEdit() {
    editingCategoryId = null;
    editingCategoryName = '';
    resetLocalError();
  }

  async function confirmEdit() {
    if (!editingCategoryId) {
      return;
    }

    const name = editingCategoryName.trim();
    if (!name) {
      localError = 'Category name is required.';
      return;
    }

    isBusy = true;
    try {
      await recipeStore.updateCategory(editingCategoryId, name);
      editingCategoryId = null;
      editingCategoryName = '';
      resetLocalError();
    } finally {
      isBusy = false;
    }
  }

  function startDelete(category: RecipeCategory) {
    deleteTarget = category;
    resetLocalError();
  }

  function cancelDelete() {
    deleteTarget = null;
    resetLocalError();
  }

  async function confirmDelete() {
    if (!deleteTarget) {
      return;
    }

    isBusy = true;
    try {
      await recipeStore.deleteCategory(deleteTarget.id);
      deleteTarget = null;
      resetLocalError();
    } finally {
      isBusy = false;
    }
  }

  async function moveCategory(index: number, direction: -1 | 1) {
    const targetIndex = index + direction;
    if (targetIndex < 0 || targetIndex >= categories.length) {
      return;
    }

    const current = categories[index];
    const target = categories[targetIndex];
    if (!current || !target) {
      return;
    }

    const currentPosition = current.position;
    const targetPosition = target.position;
    isReordering = true;
    try {
      await Promise.all([
        recipeStore.updateCategory(current.id, undefined, targetPosition),
        recipeStore.updateCategory(target.id, undefined, currentPosition),
      ]);
    } finally {
      isReordering = false;
    }
  }
</script>

<section class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm">
  <div class="flex items-start justify-between gap-3">
    <div>
      <h3 class="text-sm font-semibold text-gray-900">Recipe Categories</h3>
      <p class="mt-1 text-xs text-gray-500">Organize your saved recipes by category.</p>
    </div>
    <button
      type="button"
      class="rounded-full border border-gray-200 px-3 py-1 text-xs font-semibold text-gray-700 hover:border-gray-300 hover:bg-gray-50"
      on:click={startAdd}
      disabled={isBusy}
      data-testid="add-category"
    >
      Add category
    </button>
  </div>

  {#if displayError}
    <div class="mt-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700" data-testid="category-error">
      {displayError}
    </div>
  {/if}

  <div class="mt-4 space-y-2" role="list">
    {#if categories.length === 0}
      <p class="rounded-lg border border-dashed border-gray-200 px-3 py-4 text-xs text-gray-500">
        No custom categories yet. Add one to start organizing your recipes.
      </p>
    {:else}
      {#each categories as category, index (category.id)}
        <div
          class="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-gray-100 px-3 py-2"
          data-testid="category-row"
        >
          <div class="min-w-0">
            {#if editingCategoryId === category.id}
              <div class="flex flex-wrap items-center gap-2">
                <input
                  class="w-44 rounded-md border border-gray-200 px-2 py-1 text-xs focus:border-blue-400 focus:outline-none"
                  type="text"
                  bind:value={editingCategoryName}
                  placeholder="Category name"
                  data-testid="category-edit-input"
                />
                <button
                  type="button"
                  class="rounded-md bg-blue-600 px-2 py-1 text-xs font-semibold text-white hover:bg-blue-700"
                  on:click={confirmEdit}
                  disabled={isBusy}
                  data-testid="category-edit-save"
                >
                  Save
                </button>
                <button
                  type="button"
                  class="rounded-md border border-gray-200 px-2 py-1 text-xs text-gray-600 hover:bg-gray-50"
                  on:click={cancelEdit}
                  disabled={isBusy}
                  data-testid="category-edit-cancel"
                >
                  Cancel
                </button>
              </div>
            {:else}
              <div class="flex items-center gap-2">
                <span class="truncate text-sm font-semibold text-gray-800" data-testid="category-name">
                  {category.name}
                </span>
                <span class="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600">
                  {recipeCounts.get(category.name) ?? 0}
                </span>
              </div>
            {/if}
          </div>

          {#if editingCategoryId !== category.id}
            <div class="flex items-center gap-1">
              <button
                type="button"
                class="rounded-full border border-gray-200 p-1 text-gray-500 hover:border-gray-300 hover:bg-gray-50"
                on:click={() => moveCategory(index, -1)}
                disabled={index === 0 || isReordering}
                aria-label="Move category up"
                data-testid={`category-move-up-${category.id}`}
              >
                <svg class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                  <path d="M10 4l5 6H5l5-6z" />
                </svg>
              </button>
              <button
                type="button"
                class="rounded-full border border-gray-200 p-1 text-gray-500 hover:border-gray-300 hover:bg-gray-50"
                on:click={() => moveCategory(index, 1)}
                disabled={index === categories.length - 1 || isReordering}
                aria-label="Move category down"
                data-testid={`category-move-down-${category.id}`}
              >
                <svg class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                  <path d="M10 16l-5-6h10l-5 6z" />
                </svg>
              </button>
              <button
                type="button"
                class="rounded-full border border-gray-200 p-1 text-gray-500 hover:border-gray-300 hover:bg-gray-50"
                on:click={() => startEdit(category)}
                disabled={isBusy}
                aria-label="Edit category"
                data-testid={`category-edit-${category.id}`}
              >
                <svg class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                  <path
                    d="M13.586 3.586a2 2 0 112.828 2.828l-9 9A2 2 0 016 16H4v-2a2 2 0 01.586-1.414l9-9z"
                  />
                </svg>
              </button>
              <button
                type="button"
                class="rounded-full border border-gray-200 p-1 text-red-500 hover:border-red-200 hover:bg-red-50"
                on:click={() => startDelete(category)}
                disabled={isBusy}
                aria-label="Delete category"
                data-testid={`category-delete-${category.id}`}
              >
                <svg class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                  <path
                    fill-rule="evenodd"
                    d="M8 3a1 1 0 00-1 1v1H4a1 1 0 000 2h12a1 1 0 100-2h-3V4a1 1 0 00-1-1H8zm1 6a1 1 0 10-2 0v5a1 1 0 102 0V9zm4 0a1 1 0 10-2 0v5a1 1 0 102 0V9z"
                    clip-rule="evenodd"
                  />
                </svg>
              </button>
            </div>
          {/if}
        </div>
      {/each}
    {/if}
  </div>

  {#if isAdding}
    <div class="mt-4 rounded-lg border border-gray-100 bg-gray-50 p-3">
      <label class="text-xs font-semibold text-gray-600" for="new-category">New category</label>
      <div class="mt-2 flex flex-wrap items-center gap-2">
        <input
          id="new-category"
          class="w-48 rounded-md border border-gray-200 px-2 py-1 text-xs focus:border-blue-400 focus:outline-none"
          type="text"
          bind:value={newCategoryName}
          placeholder="e.g. Weeknight"
          data-testid="category-add-input"
        />
        <button
          type="button"
          class="rounded-md bg-blue-600 px-3 py-1 text-xs font-semibold text-white hover:bg-blue-700"
          on:click={confirmAdd}
          disabled={isBusy}
          data-testid="category-add-save"
        >
          Add
        </button>
        <button
          type="button"
          class="rounded-md border border-gray-200 px-3 py-1 text-xs text-gray-600 hover:bg-white"
          on:click={cancelAdd}
          disabled={isBusy}
          data-testid="category-add-cancel"
        >
          Cancel
        </button>
      </div>
    </div>
  {/if}

  {#if deleteTarget}
    <div class="mt-4 rounded-lg border border-red-200 bg-red-50 px-3 py-3 text-xs text-red-700" data-testid="category-delete-confirm">
      <p class="font-semibold">Delete "{deleteTarget.name}"?</p>
      <p class="mt-1">Recipes in this category will move to Uncategorized.</p>
      <div class="mt-3 flex flex-wrap items-center gap-2">
        <button
          type="button"
          class="rounded-md bg-red-600 px-3 py-1 text-xs font-semibold text-white hover:bg-red-700"
          on:click={confirmDelete}
          disabled={isBusy}
          data-testid="category-delete-confirm-button"
        >
          Delete
        </button>
        <button
          type="button"
          class="rounded-md border border-red-200 px-3 py-1 text-xs text-red-700 hover:bg-red-100"
          on:click={cancelDelete}
          disabled={isBusy}
          data-testid="category-delete-cancel-button"
        >
          Cancel
        </button>
      </div>
    </div>
  {/if}
</section>
