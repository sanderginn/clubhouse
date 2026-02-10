<script lang="ts">
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import type { BookshelfCategory, BookshelfItem } from '../../services/api';
  import { api } from '../../services/api';
  import {
    allBookshelf,
    bookStore,
    bookStoreMeta,
    bookshelfCategories,
    myBookshelf,
  } from '../../stores/bookStore';
  import { currentUser } from '../../stores/authStore';
  import type { Link, LinkMetadata, Post } from '../../stores/postStore';
  import { postStore } from '../../stores/postStore';
  import { buildStandaloneThreadHref } from '../../services/routeNavigation';
  import BookCard from './BookCard.svelte';

  type TabKey = 'my' | 'all';

  type BookCardData = {
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

  type MetadataWithBook = LinkMetadata & {
    book?: Partial<BookCardData> & Record<string, unknown>;
  };

  type CategoryOption = {
    value: string;
    label: string;
    editable: boolean;
    category?: BookshelfCategory;
  };

  type BookListItem = {
    item: BookshelfItem;
    post?: Post;
    bookData: BookCardData;
    saverLabel: string;
    threadHref: string;
    createdAt: number;
  };

  const ALL_CATEGORY_VALUE = '__all__';
  const ALL_CATEGORY_LABEL = 'All';
  const UNCATEGORIZED_CATEGORY = 'Uncategorized';

  let activeTab: TabKey = 'my';
  let selectedCategory = ALL_CATEGORY_VALUE;
  let myHasMore = false;
  let allHasMore = false;
  let hasLoadedAllTab = false;

  let postsByID = new Map<string, Post>();
  let customCategories: BookshelfCategory[] = [];
  let categoryCounts = new Map<string, number>();
  let categoryOptions: CategoryOption[] = [];
  let totalSavedCount = 0;
  let mySelectedItems: BookshelfItem[] = [];
  let myBookItems: BookListItem[] = [];
  let allBookItems: BookListItem[] = [];
  let selectedCategoryLabel = ALL_CATEGORY_LABEL;
  let displayCategoryError: string | null = null;

  let isCreateCategoryOpen = false;
  let createCategoryName = '';
  let editingCategoryID: string | null = null;
  let editingCategoryName = '';
  let deleteCategoryID: string | null = null;
  let deleteCategoryName = '';
  let isCategoryActionBusy = false;
  let localCategoryError: string | null = null;
  let draggingCategoryID: string | null = null;

  onMount(() => {
    void initialize();
  });

  $: postsByID = new Map($postStore.posts.map((post) => [post.id, post]));
  $: customCategories = [...$bookshelfCategories].sort(
    (a, b) => a.position - b.position || a.name.localeCompare(b.name)
  );
  $: categoryCounts = buildCategoryCounts($myBookshelf);
  $: totalSavedCount = Array.from(categoryCounts.values()).reduce((sum, count) => sum + count, 0);
  $: categoryOptions = buildCategoryOptions(customCategories);
  $: selectedCategoryLabel =
    categoryOptions.find((option) => option.value === selectedCategory)?.label ?? ALL_CATEGORY_LABEL;
  $: displayCategoryError = localCategoryError ?? $bookStoreMeta.error;

  $: if (
    selectedCategory !== ALL_CATEGORY_VALUE &&
    !categoryOptions.some((option) => option.value === selectedCategory)
  ) {
    selectedCategory = ALL_CATEGORY_VALUE;
  }

  $: mySelectedItems = getItemsForCategory($myBookshelf, selectedCategory);
  $: myBookItems = buildBookListItems(mySelectedItems, postsByID, $currentUser?.id);
  $: allBookItems = buildBookListItems(
    getItemsForCategory($allBookshelf, ALL_CATEGORY_VALUE),
    postsByID,
    $currentUser?.id
  );

  async function initialize(): Promise<void> {
    await bookStore.loadBookshelfCategories();
    const nextCursor = await bookStore.loadMyBookshelf(undefined);
    myHasMore = Boolean(nextCursor);
  }

  function buildCategoryCounts(itemsByCategory: Map<string, BookshelfItem[]>): Map<string, number> {
    const counts = new Map<string, number>();
    for (const [categoryName, items] of itemsByCategory.entries()) {
      counts.set(categoryName, items.length);
    }
    return counts;
  }

  function buildCategoryOptions(categories: BookshelfCategory[]): CategoryOption[] {
    const options: CategoryOption[] = [
      {
        value: ALL_CATEGORY_VALUE,
        label: ALL_CATEGORY_LABEL,
        editable: false,
      },
    ];

    const seen = new Set<string>([ALL_CATEGORY_VALUE]);
    for (const category of categories) {
      if (!seen.has(category.name)) {
        options.push({
          value: category.name,
          label: category.name,
          editable: true,
          category,
        });
        seen.add(category.name);
      }
    }

    if (!seen.has(UNCATEGORIZED_CATEGORY)) {
      options.push({
        value: UNCATEGORIZED_CATEGORY,
        label: UNCATEGORIZED_CATEGORY,
        editable: false,
      });
    }

    return options;
  }

  function getItemsForCategory(
    itemsByCategory: Map<string, BookshelfItem[]>,
    category: string
  ): BookshelfItem[] {
    if (category === ALL_CATEGORY_VALUE) {
      const allItems: BookshelfItem[] = [];
      for (const items of itemsByCategory.values()) {
        allItems.push(...items);
      }
      return allItems;
    }

    return itemsByCategory.get(category) ?? [];
  }

  function normalizeStringArray(value: unknown): string[] | undefined {
    if (!Array.isArray(value)) {
      return undefined;
    }

    const normalized = value
      .filter((entry): entry is string => typeof entry === 'string')
      .map((entry) => entry.trim())
      .filter((entry) => entry.length > 0);

    return normalized.length > 0 ? normalized : undefined;
  }

  function parseAuthors(rawAuthor?: string): string[] | undefined {
    if (typeof rawAuthor !== 'string') {
      return undefined;
    }

    const normalized = rawAuthor
      .split(',')
      .map((entry) => entry.trim())
      .filter((entry) => entry.length > 0);

    return normalized.length > 0 ? normalized : undefined;
  }

  function findBookLink(post?: Post): Link | null {
    if (!post?.links?.length) {
      return null;
    }

    for (const link of post.links) {
      const metadata = link.metadata as MetadataWithBook | undefined;
      if (!metadata) {
        continue;
      }
      if (metadata.book) {
        return link;
      }
      if (metadata.type === 'book') {
        return link;
      }
      if (metadata.title || metadata.author || metadata.description || metadata.image) {
        return link;
      }
    }

    return post.links[0] ?? null;
  }

  function extractBookData(post?: Post): BookCardData {
    if (!post) {
      return { title: 'Book post' };
    }

    const link = findBookLink(post);
    const metadata = link?.metadata as MetadataWithBook | undefined;
    const rawBook =
      metadata?.book && typeof metadata.book === 'object'
        ? (metadata.book as Partial<BookCardData>)
        : undefined;

    const title =
      (typeof rawBook?.title === 'string' && rawBook.title.trim().length > 0
        ? rawBook.title.trim()
        : undefined) ??
      metadata?.title?.trim() ??
      post.content.trim() ??
      'Book post';

    return {
      title,
      ...(rawBook?.description ? { description: rawBook.description } : {}),
      ...(metadata?.description && !rawBook?.description ? { description: metadata.description } : {}),
      ...(rawBook?.coverUrl ? { coverUrl: rawBook.coverUrl } : {}),
      ...(rawBook?.cover_url ? { cover_url: rawBook.cover_url } : {}),
      ...(metadata?.image && !rawBook?.coverUrl && !rawBook?.cover_url
        ? { coverUrl: metadata.image }
        : {}),
      ...(typeof rawBook?.pageCount === 'number' ? { pageCount: rawBook.pageCount } : {}),
      ...(typeof rawBook?.page_count === 'number' ? { page_count: rawBook.page_count } : {}),
      ...(typeof rawBook?.publishDate === 'string' ? { publishDate: rawBook.publishDate } : {}),
      ...(typeof rawBook?.publish_date === 'string' ? { publish_date: rawBook.publish_date } : {}),
      ...(typeof rawBook?.openLibraryKey === 'string' ? { openLibraryKey: rawBook.openLibraryKey } : {}),
      ...(typeof rawBook?.open_library_key === 'string'
        ? { open_library_key: rawBook.open_library_key }
        : {}),
      ...(typeof rawBook?.goodreadsUrl === 'string' ? { goodreadsUrl: rawBook.goodreadsUrl } : {}),
      ...(typeof rawBook?.goodreads_url === 'string' ? { goodreads_url: rawBook.goodreads_url } : {}),
      ...(normalizeStringArray(rawBook?.genres) ? { genres: normalizeStringArray(rawBook?.genres) } : {}),
      ...(normalizeStringArray(rawBook?.authors)
        ? { authors: normalizeStringArray(rawBook?.authors) }
        : parseAuthors(metadata?.author)
          ? { authors: parseAuthors(metadata?.author) }
          : {}),
    };
  }

  function buildSaverLabel(item: BookshelfItem, viewerID?: string): string {
    if (viewerID && item.userId === viewerID) {
      return 'You';
    }
    return item.userId;
  }

  function buildBookListItems(
    items: BookshelfItem[],
    postMap: Map<string, Post>,
    viewerID?: string
  ): BookListItem[] {
    const sortedItems = [...items].sort(
      (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
    );

    return sortedItems.map((item) => {
      const post = postMap.get(item.postId);
      return {
        item,
        post,
        bookData: extractBookData(post),
        saverLabel: buildSaverLabel(item, viewerID),
        threadHref: buildStandaloneThreadHref(item.postId),
        createdAt: new Date(item.createdAt).getTime(),
      };
    });
  }

  async function loadMyBooks(reset: boolean): Promise<void> {
    if ($bookStoreMeta.loading.myBookshelf) {
      return;
    }

    const cursor = reset ? undefined : get(bookStoreMeta).cursors.myBookshelf ?? undefined;
    const nextCursor = await bookStore.loadMyBookshelf(undefined, cursor);
    myHasMore = Boolean(nextCursor);
  }

  async function loadAllBooks(reset: boolean): Promise<void> {
    if ($bookStoreMeta.loading.allBookshelf) {
      return;
    }

    if (reset) {
      hasLoadedAllTab = true;
    }

    const cursor = reset ? undefined : get(bookStoreMeta).cursors.allBookshelf ?? undefined;
    const nextCursor = await bookStore.loadAllBookshelf(undefined, cursor);
    allHasMore = Boolean(nextCursor);
  }

  function handleTabSelect(nextTab: TabKey): void {
    activeTab = nextTab;
    if (nextTab === 'all' && !hasLoadedAllTab) {
      void loadAllBooks(true);
    }
  }

  function handleCategorySelect(category: string): void {
    selectedCategory = category;
  }

  function clearCategoryError(): void {
    localCategoryError = null;
  }

  function resetCategoryEditingState(): void {
    editingCategoryID = null;
    editingCategoryName = '';
    deleteCategoryID = null;
    deleteCategoryName = '';
  }

  function startCreateCategory(): void {
    if (isCategoryActionBusy) {
      return;
    }
    resetCategoryEditingState();
    clearCategoryError();
    isCreateCategoryOpen = true;
    createCategoryName = '';
  }

  function cancelCreateCategory(): void {
    isCreateCategoryOpen = false;
    createCategoryName = '';
    clearCategoryError();
  }

  async function confirmCreateCategory(): Promise<void> {
    const name = createCategoryName.trim();
    if (!name) {
      localCategoryError = 'Category name is required.';
      return;
    }

    isCategoryActionBusy = true;
    clearCategoryError();
    try {
      await bookStore.createCategory(name);
      const createError = get(bookStoreMeta).error;
      if (createError) {
        localCategoryError = createError;
        return;
      }

      await bookStore.loadBookshelfCategories();
      const refreshError = get(bookStoreMeta).error;
      if (refreshError) {
        localCategoryError = refreshError;
        return;
      }

      isCreateCategoryOpen = false;
      createCategoryName = '';
      clearCategoryError();
    } finally {
      isCategoryActionBusy = false;
    }
  }

  function startEditCategory(category?: BookshelfCategory): void {
    if (isCategoryActionBusy || !category) {
      return;
    }

    isCreateCategoryOpen = false;
    deleteCategoryID = null;
    deleteCategoryName = '';
    editingCategoryID = category.id;
    editingCategoryName = category.name;
    clearCategoryError();
  }

  function cancelEditCategory(): void {
    editingCategoryID = null;
    editingCategoryName = '';
    clearCategoryError();
  }

  async function confirmEditCategory(): Promise<void> {
    if (!editingCategoryID) {
      return;
    }

    const existing = customCategories.find((category) => category.id === editingCategoryID);
    const nextName = editingCategoryName.trim();

    if (!nextName) {
      localCategoryError = 'Category name is required.';
      return;
    }

    if (existing && existing.name === nextName) {
      cancelEditCategory();
      return;
    }

    isCategoryActionBusy = true;
    clearCategoryError();
    try {
      await bookStore.updateCategory(editingCategoryID, nextName, existing?.position ?? 0);
      const updateError = get(bookStoreMeta).error;
      if (updateError) {
        localCategoryError = updateError;
        return;
      }

      await bookStore.loadBookshelfCategories();
      const refreshError = get(bookStoreMeta).error;
      if (refreshError) {
        localCategoryError = refreshError;
        return;
      }

      if (existing && selectedCategory === existing.name) {
        selectedCategory = nextName;
      }

      await loadMyBooks(true);
      cancelEditCategory();
      clearCategoryError();
    } finally {
      isCategoryActionBusy = false;
    }
  }

  function startDeleteCategory(category?: BookshelfCategory): void {
    if (isCategoryActionBusy || !category) {
      return;
    }

    isCreateCategoryOpen = false;
    editingCategoryID = null;
    editingCategoryName = '';
    deleteCategoryID = category.id;
    deleteCategoryName = category.name;
    clearCategoryError();
  }

  function cancelDeleteCategory(): void {
    deleteCategoryID = null;
    deleteCategoryName = '';
    clearCategoryError();
  }

  async function confirmDeleteCategory(): Promise<void> {
    if (!deleteCategoryID) {
      return;
    }

    const deletedCategoryName = deleteCategoryName;

    isCategoryActionBusy = true;
    clearCategoryError();
    try {
      await bookStore.deleteCategory(deleteCategoryID);
      const deleteError = get(bookStoreMeta).error;
      if (deleteError) {
        localCategoryError = deleteError;
        return;
      }

      await bookStore.loadBookshelfCategories();
      const refreshError = get(bookStoreMeta).error;
      if (refreshError) {
        localCategoryError = refreshError;
        return;
      }

      if (selectedCategory === deletedCategoryName) {
        selectedCategory = ALL_CATEGORY_VALUE;
      }

      await loadMyBooks(true);
      cancelDeleteCategory();
      clearCategoryError();
    } finally {
      isCategoryActionBusy = false;
    }
  }

  function handleCategoryDragStart(categoryID: string): void {
    if (isCategoryActionBusy) {
      return;
    }

    draggingCategoryID = categoryID;
    clearCategoryError();
  }

  function handleCategoryDragOver(event: DragEvent): void {
    event.preventDefault();
  }

  function handleCategoryDragEnd(): void {
    draggingCategoryID = null;
  }

  async function handleCategoryDrop(targetCategoryID: string): Promise<void> {
    if (!draggingCategoryID || draggingCategoryID === targetCategoryID) {
      draggingCategoryID = null;
      return;
    }

    const fromIndex = customCategories.findIndex((category) => category.id === draggingCategoryID);
    const toIndex = customCategories.findIndex((category) => category.id === targetCategoryID);
    if (fromIndex < 0 || toIndex < 0) {
      draggingCategoryID = null;
      return;
    }

    const reordered = [...customCategories];
    const [moved] = reordered.splice(fromIndex, 1);
    reordered.splice(toIndex, 0, moved);

    bookshelfCategories.set(reordered.map((category, index) => ({ ...category, position: index })));

    isCategoryActionBusy = true;
    clearCategoryError();
    try {
      await api.reorderBookshelfCategories(reordered.map((category) => category.id));
      await bookStore.loadBookshelfCategories();
      const refreshError = get(bookStoreMeta).error;
      if (refreshError) {
        localCategoryError = refreshError;
      }
    } catch (error) {
      localCategoryError = error instanceof Error ? error.message : 'Failed to reorder categories';
      await bookStore.loadBookshelfCategories();
    } finally {
      draggingCategoryID = null;
      isCategoryActionBusy = false;
    }
  }
</script>

<section class="rounded-2xl border border-gray-200 bg-white shadow-sm" data-testid="bookshelf">
  <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-4 py-3">
    <div>
      <h2 class="text-base font-semibold text-gray-900">Bookshelf</h2>
      <p class="text-xs text-gray-500">Save books you want to remember and discover community picks.</p>
    </div>

    <div class="flex items-center gap-2" role="tablist" aria-label="Bookshelf views">
      <button
        type="button"
        role="tab"
        class={`rounded-full px-3 py-1 text-xs font-semibold transition-colors ${
          activeTab === 'my' ? 'bg-blue-100 text-blue-700' : 'text-gray-600 hover:bg-gray-100'
        }`}
        aria-selected={activeTab === 'my'}
        on:click={() => handleTabSelect('my')}
        data-testid="bookshelf-tab-my"
      >
        My Books
      </button>
      <button
        type="button"
        role="tab"
        class={`rounded-full px-3 py-1 text-xs font-semibold transition-colors ${
          activeTab === 'all' ? 'bg-blue-100 text-blue-700' : 'text-gray-600 hover:bg-gray-100'
        }`}
        aria-selected={activeTab === 'all'}
        on:click={() => handleTabSelect('all')}
        data-testid="bookshelf-tab-all"
      >
        All Books
      </button>
    </div>
  </div>

  <div class="p-4">
    {#if activeTab === 'my'}
      <div class="flex flex-col gap-4 lg:flex-row">
        <aside class="lg:w-72" data-testid="bookshelf-category-panel">
          <div class="rounded-xl border border-gray-200 bg-white p-3 shadow-sm">
            <div class="flex items-center justify-between">
              <h3 class="text-sm font-semibold text-gray-900">Categories</h3>
              <span class="text-xs text-gray-500">{totalSavedCount} books</span>
            </div>

            <div class="mt-3 space-y-1">
              {#each categoryOptions as option}
                <div class="rounded-lg" data-testid={`bookshelf-category-row-${option.value}`}>
                  {#if editingCategoryID === option.category?.id}
                    <div class="flex flex-wrap items-center gap-2 rounded-lg border border-blue-200 bg-blue-50 px-3 py-2">
                      <input
                        class="min-w-[8rem] flex-1 rounded-md border border-blue-300 px-2 py-1 text-xs focus:border-blue-500 focus:outline-none"
                        type="text"
                        bind:value={editingCategoryName}
                        placeholder="Category name"
                        data-testid="bookshelf-category-edit-input"
                      />
                      <button
                        type="button"
                        class="rounded-md bg-blue-600 px-2 py-1 text-xs font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                        on:click={confirmEditCategory}
                        disabled={isCategoryActionBusy}
                        data-testid="bookshelf-category-edit-save"
                      >
                        {isCategoryActionBusy ? 'Saving...' : 'Save'}
                      </button>
                      <button
                        type="button"
                        class="rounded-md border border-blue-200 px-2 py-1 text-xs text-blue-700 hover:bg-blue-100 disabled:cursor-not-allowed disabled:opacity-60"
                        on:click={cancelEditCategory}
                        disabled={isCategoryActionBusy}
                        data-testid="bookshelf-category-edit-cancel"
                      >
                        Cancel
                      </button>
                    </div>
                  {:else}
                    <div
                      class="group flex items-center gap-1"
                      role="listitem"
                      data-testid={option.category ? `bookshelf-custom-category-${option.category.id}` : undefined}
                      draggable={Boolean(option.category)}
                      on:dragstart={() => option.category && handleCategoryDragStart(option.category.id)}
                      on:dragend={handleCategoryDragEnd}
                      on:dragover={handleCategoryDragOver}
                      on:drop={() => option.category && handleCategoryDrop(option.category.id)}
                    >
                      <button
                        type="button"
                        class={`flex flex-1 items-center justify-between rounded-lg px-3 py-2 text-xs font-semibold transition-colors ${
                          selectedCategory === option.value
                            ? 'bg-blue-50 text-blue-700'
                            : 'text-gray-600 hover:bg-gray-100'
                        }`}
                        on:click={() => handleCategorySelect(option.value)}
                        data-testid={`bookshelf-category-${option.value}`}
                      >
                        <span>{option.label}</span>
                        <span class="rounded-full bg-white px-2 py-0.5 text-[11px] text-gray-500">
                          {option.value === ALL_CATEGORY_VALUE
                            ? totalSavedCount
                            : (categoryCounts.get(option.value) ?? 0)}
                        </span>
                      </button>

                      {#if option.category}
                        <div class="hidden items-center gap-1 group-hover:flex">
                          <button
                            type="button"
                            class="rounded-md border border-gray-200 px-1.5 py-1 text-[11px] text-gray-500 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
                            on:click={() => startEditCategory(option.category)}
                            disabled={isCategoryActionBusy}
                            aria-label={`Edit ${option.category.name}`}
                            data-testid={`bookshelf-category-edit-${option.category.id}`}
                          >
                            Edit
                          </button>
                          <button
                            type="button"
                            class="rounded-md border border-gray-200 px-1.5 py-1 text-[11px] text-gray-500 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
                            on:click={() => startDeleteCategory(option.category)}
                            disabled={isCategoryActionBusy}
                            aria-label={`Delete ${option.category.name}`}
                            data-testid={`bookshelf-category-delete-${option.category.id}`}
                          >
                            Delete
                          </button>
                        </div>
                      {/if}
                    </div>
                  {/if}

                  {#if deleteCategoryID === option.category?.id}
                    <div
                      class="mt-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900"
                      data-testid="bookshelf-category-delete-confirm"
                    >
                      <p>
                        Delete <span class="font-semibold">{deleteCategoryName}</span>? Books in this category will
                        move to <span class="font-semibold">{UNCATEGORIZED_CATEGORY}</span>.
                      </p>
                      <div class="mt-2 flex items-center gap-2">
                        <button
                          type="button"
                          class="rounded-md bg-amber-600 px-2 py-1 text-xs font-semibold text-white hover:bg-amber-700 disabled:cursor-not-allowed disabled:opacity-60"
                          on:click={confirmDeleteCategory}
                          disabled={isCategoryActionBusy}
                          data-testid="bookshelf-category-delete-confirm-button"
                        >
                          {isCategoryActionBusy ? 'Deleting...' : 'Delete'}
                        </button>
                        <button
                          type="button"
                          class="rounded-md border border-amber-200 px-2 py-1 text-xs text-amber-900 hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-60"
                          on:click={cancelDeleteCategory}
                          disabled={isCategoryActionBusy}
                          data-testid="bookshelf-category-delete-cancel-button"
                        >
                          Cancel
                        </button>
                      </div>
                    </div>
                  {/if}
                </div>
              {/each}
            </div>

            {#if isCreateCategoryOpen}
              <div class="mt-3 rounded-lg border border-gray-200 bg-gray-50 p-2">
                <input
                  type="text"
                  class="w-full rounded-md border border-gray-300 px-2 py-1 text-xs"
                  placeholder="Category name"
                  bind:value={createCategoryName}
                  data-testid="bookshelf-category-create-input"
                />
                <div class="mt-2 flex items-center gap-2">
                  <button
                    type="button"
                    class="rounded-md bg-blue-600 px-2 py-1 text-xs font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                    on:click={confirmCreateCategory}
                    disabled={isCategoryActionBusy}
                    data-testid="bookshelf-category-create-save"
                  >
                    {isCategoryActionBusy ? 'Saving...' : 'Save'}
                  </button>
                  <button
                    type="button"
                    class="rounded-md border border-gray-300 px-2 py-1 text-xs text-gray-700 hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-60"
                    on:click={cancelCreateCategory}
                    disabled={isCategoryActionBusy}
                    data-testid="bookshelf-category-create-cancel"
                  >
                    Cancel
                  </button>
                </div>
              </div>
            {:else}
              <button
                type="button"
                class="mt-3 w-full rounded-lg border border-dashed border-gray-300 px-3 py-2 text-xs font-semibold text-gray-600 hover:border-gray-400 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
                on:click={startCreateCategory}
                disabled={isCategoryActionBusy}
                data-testid="bookshelf-category-create"
              >
                + Create category
              </button>
            {/if}

            {#if displayCategoryError}
              <div
                class="mt-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700"
                data-testid="bookshelf-category-error"
              >
                {displayCategoryError}
              </div>
            {/if}
          </div>
        </aside>

        <div class="min-w-0 flex-1">
          <div class="flex items-center justify-between">
            <div>
              <h3 class="text-sm font-semibold text-gray-900">{selectedCategoryLabel}</h3>
              <p class="text-xs text-gray-500">Books you have saved from book posts.</p>
            </div>
            <span class="text-xs text-gray-400">{myBookItems.length} books</span>
          </div>

          {#if myBookItems.length === 0}
            <div
              class="mt-3 rounded-xl border border-dashed border-gray-200 px-4 py-6 text-sm text-gray-500"
              data-testid="bookshelf-my-empty"
            >
              {#if totalSavedCount === 0}
                <p>No books saved yet</p>
                <p class="mt-1 text-xs text-gray-500">Save books from book posts to build your shelf.</p>
              {:else if selectedCategory !== ALL_CATEGORY_VALUE}
                <p>No books in this category</p>
              {:else}
                <p>No books saved yet</p>
              {/if}
            </div>
          {:else}
            <div class="mt-3 grid grid-cols-1 gap-3 md:grid-cols-2" data-testid="bookshelf-my-grid">
              {#each myBookItems as bookItem (bookItem.item.id)}
                <div
                  class="rounded-xl border border-gray-200 bg-white p-2 shadow-sm"
                  data-testid={`bookshelf-my-item-${bookItem.item.postId}`}
                >
                  <BookCard bookData={bookItem.bookData} compact={true} threadHref={bookItem.threadHref} />
                </div>
              {/each}
            </div>
          {/if}

          {#if myHasMore}
            <div class="mt-4 flex justify-center">
              <button
                type="button"
                class="rounded-lg border border-gray-200 px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
                on:click={() => void loadMyBooks(false)}
                disabled={$bookStoreMeta.loading.myBookshelf}
                data-testid="bookshelf-my-load-more"
              >
                {$bookStoreMeta.loading.myBookshelf ? 'Loading...' : 'Load more'}
              </button>
            </div>
          {/if}
        </div>
      </div>
    {:else}
      <div>
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-sm font-semibold text-gray-900">All Books</h3>
            <p class="text-xs text-gray-500">See what everyone in your community has saved.</p>
          </div>
          <span class="text-xs text-gray-400">{allBookItems.length} saved</span>
        </div>

        {#if $bookStoreMeta.loading.allBookshelf && allBookItems.length === 0}
          <div
            class="mt-3 rounded-xl border border-dashed border-gray-200 px-4 py-6 text-sm text-gray-500"
            data-testid="bookshelf-all-loading"
          >
            Loading books...
          </div>
        {:else if allBookItems.length === 0}
          <div
            class="mt-3 rounded-xl border border-dashed border-gray-200 px-4 py-6 text-sm text-gray-500"
            data-testid="bookshelf-all-empty"
          >
            No books saved yet
          </div>
        {:else}
          <div class="mt-3 grid grid-cols-1 gap-3 md:grid-cols-2" data-testid="bookshelf-all-grid">
            {#each allBookItems as bookItem (bookItem.item.id)}
              <div
                class="rounded-xl border border-gray-200 bg-white p-2 shadow-sm"
                data-testid={`bookshelf-all-item-${bookItem.item.postId}`}
              >
                <p class="mb-2 text-xs font-medium text-gray-500" data-testid={`bookshelf-saver-${bookItem.item.id}`}>
                  Saved by {bookItem.saverLabel}
                </p>
                <BookCard bookData={bookItem.bookData} compact={true} threadHref={bookItem.threadHref} />
              </div>
            {/each}
          </div>
        {/if}

        {#if allHasMore}
          <div class="mt-4 flex justify-center">
            <button
              type="button"
              class="rounded-lg border border-gray-200 px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
              on:click={() => void loadAllBooks(false)}
              disabled={$bookStoreMeta.loading.allBookshelf}
              data-testid="bookshelf-all-load-more"
            >
              {$bookStoreMeta.loading.allBookshelf ? 'Loading...' : 'Load more'}
            </button>
          </div>
        {/if}
      </div>
    {/if}
  </div>
</section>
