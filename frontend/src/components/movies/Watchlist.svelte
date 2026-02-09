<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import { get } from 'svelte/store';
  import type { Link, LinkMetadata, Post } from '../../stores/postStore';
  import type { WatchlistCategory, WatchlistItem } from '../../stores/movieStore';
  import { movieStore, sortedCategories, watchlistByCategory } from '../../stores/movieStore';
  import { sections } from '../../stores/sectionStore';
  import { api } from '../../services/api';
  import { buildStandaloneThreadHref, pushPath } from '../../services/routeNavigation';

  type TabKey = 'my' | 'all';
  type SortKey = 'rating' | 'date' | 'watch_count' | 'watchlist_count';
  type WatchlistSectionType = 'movie' | 'series';

  type MovieListItem = {
    postId: string;
    title: string;
    poster: string | null;
    rating: number | null;
    watchCount: number;
    watchlistCount: number;
    watched: boolean;
    createdAt: number;
  };

  type MovieStatsLike = {
    averageRating?: number | null;
    avgRating?: number | null;
    avg_rating?: number | null;
    watchCount?: number;
    watch_count?: number;
    watchlistCount?: number;
    watchlist_count?: number;
  };

  type MovieMetadataLike = {
    title?: string;
    poster?: string;
    backdrop?: string;
    tmdb_rating?: number;
    tmdbRating?: number;
  };

  type MetadataWithMovie = LinkMetadata & {
    movie?: MovieMetadataLike;
  };

  const ALL_CATEGORY_VALUE = '__all__';
  const ALL_CATEGORY_LABEL = 'All';
  const ALL_MOVIES_PAGE_SIZE = 20;
  const DEFAULT_WATCHLIST_CATEGORY = 'Uncategorized';

  const dispatch = createEventDispatcher<{
    createCategory: undefined;
  }>();

  export let sectionType: WatchlistSectionType = 'movie';

  let activeTab: TabKey = 'my';
  let selectedCategory = ALL_CATEGORY_VALUE;
  let sortKey: SortKey = 'rating';
  let searchTerm = '';
  let allMoviePosts: Post[] = [];
  let allMoviesHasMore = false;
  let allMoviesNextCursor: string | null = null;
  let allMoviesLoading = false;
  let allMoviesAutoLoadAttempted = false;
  let allMoviesError: string | null = null;
  let editingCategoryId: string | null = null;
  let editingCategoryName = '';
  let deleteCategoryId: string | null = null;
  let deleteCategoryName = '';
  let isCategoryActionBusy = false;
  let localCategoryError: string | null = null;
  let lastLoadedSectionType: WatchlistSectionType | null = null;

  let tabOptions: Array<{ key: TabKey; label: string }> = [];

  const sortOptions: Array<{ key: SortKey; label: string }> = [
    { key: 'rating', label: 'Rating' },
    { key: 'date', label: 'Date posted' },
    { key: 'watch_count', label: 'Watch count' },
    { key: 'watchlist_count', label: 'Watchlist count' },
  ];

  onMount(() => {
    const state = get(movieStore);

    if (!state.isLoadingCategories && state.categories.length === 0) {
      movieStore.loadWatchlistCategories();
    }

    if (!state.isLoadingWatchLogs && state.watchLogs.length === 0) {
      movieStore.loadWatchLogs();
    }
  });

  $: mediaLabelPlural = sectionType === 'series' ? 'series' : 'movies';
  $: mediaLabelTitle = sectionType === 'series' ? 'Series' : 'Movies';
  $: tabOptions = [
    { key: 'my', label: 'My List' },
    { key: 'all', label: `All ${mediaLabelTitle}` },
  ];
  $: sectionTypeBySectionID = new Map($sections.map((section) => [section.id, section.type]));
  $: filteredWatchlistByCategory = filterWatchlistBySectionType(
    $watchlistByCategory,
    sectionTypeBySectionID,
    sectionType
  );
  $: categoryCounts = buildCategoryCounts(filteredWatchlistByCategory);
  $: postWatchlistCounts = buildPostWatchlistCounts(filteredWatchlistByCategory);
  $: totalSavedCount = Array.from(categoryCounts.values()).reduce(
    (total, count) => total + count,
    0
  );
  $: editableCategoriesByName = new Map<string, WatchlistCategory>(
    $sortedCategories.map((category) => [category.name, category])
  );
  $: editableCategoriesByID = new Map<string, WatchlistCategory>(
    $sortedCategories.map((category) => [category.id, category])
  );
  $: displayCategoryError = localCategoryError ?? $movieStore.error;
  $: watchedPostIDs = new Set($movieStore.watchLogs.map((log) => log.postId));
  $: categoryOptions = buildCategoryOptions($sortedCategories, filteredWatchlistByCategory);
  $: selectedCategoryLabel =
    categoryOptions.find((option) => option.value === selectedCategory)?.label ??
    ALL_CATEGORY_LABEL;

  $: if (
    selectedCategory !== ALL_CATEGORY_VALUE &&
    !categoryOptions.some((option) => option.value === selectedCategory)
  ) {
    selectedCategory = ALL_CATEGORY_VALUE;
  }

  $: selectedWatchlistItems = getWatchlistItemsForCategory(filteredWatchlistByCategory, selectedCategory);
  $: myMovies = buildWatchlistMovieItems(
    selectedWatchlistItems,
    watchedPostIDs,
    postWatchlistCounts
  );
  $: allMovies = sortMovies(
    filterMoviesBySearch(
      buildAllMovieItems(
        allMoviePosts,
        watchedPostIDs,
        postWatchlistCounts,
        sectionTypeBySectionID,
        sectionType
      ),
      searchTerm
    ),
    sortKey
  );
  $: if (lastLoadedSectionType !== sectionType) {
    lastLoadedSectionType = sectionType;
    selectedCategory = ALL_CATEGORY_VALUE;
    searchTerm = '';
    allMoviePosts = [];
    allMoviesHasMore = false;
    allMoviesNextCursor = null;
    allMoviesAutoLoadAttempted = false;
    allMoviesError = null;
    if (typeof window !== 'undefined') {
      void movieStore.loadWatchlist(sectionType);
    }
  }
  $: if (activeTab === 'all' && !allMoviesAutoLoadAttempted && !allMoviesLoading) {
    allMoviesAutoLoadAttempted = true;
    void loadAllMovies(true);
  }

  function isPostInSectionType(
    post: Post | undefined,
    sectionTypesBySectionID: Map<string, string>,
    targetSectionType: WatchlistSectionType
  ): boolean {
    if (!post) {
      return true;
    }

    const postSectionType = sectionTypesBySectionID.get(post.sectionId);
    if (!postSectionType) {
      return true;
    }

    return postSectionType === targetSectionType;
  }

  function filterWatchlistBySectionType(
    watchlistMap: Map<string, WatchlistItem[]>,
    sectionTypesBySectionID: Map<string, string>,
    targetSectionType: WatchlistSectionType
  ): Map<string, WatchlistItem[]> {
    const filtered = new Map<string, WatchlistItem[]>();
    for (const [categoryName, items] of watchlistMap.entries()) {
      const matching = items.filter((item) =>
        isPostInSectionType(item.post, sectionTypesBySectionID, targetSectionType)
      );
      if (matching.length > 0) {
        filtered.set(categoryName, matching);
      }
    }

    return filtered;
  }

  function buildCategoryCounts(map: Map<string, WatchlistItem[]>): Map<string, number> {
    const counts = new Map<string, number>();
    for (const [categoryName, items] of map.entries()) {
      counts.set(categoryName, items.length);
    }
    return counts;
  }

  function buildCategoryOptions(
    categories: Array<{ name: string }>,
    watchlistMap: Map<string, WatchlistItem[]>
  ): Array<{ value: string; label: string }> {
    const options: Array<{ value: string; label: string }> = [
      { value: ALL_CATEGORY_VALUE, label: ALL_CATEGORY_LABEL },
    ];
    const seen = new Set<string>([ALL_CATEGORY_VALUE]);

    for (const category of categories) {
      if (!seen.has(category.name)) {
        options.push({ value: category.name, label: category.name });
        seen.add(category.name);
      }
    }

    for (const categoryName of watchlistMap.keys()) {
      if (!seen.has(categoryName)) {
        options.push({ value: categoryName, label: categoryName });
        seen.add(categoryName);
      }
    }

    return options;
  }

  function buildPostWatchlistCounts(
    watchlistMap: Map<string, WatchlistItem[]>
  ): Map<string, number> {
    const counts = new Map<string, number>();
    for (const items of watchlistMap.values()) {
      for (const item of items) {
        counts.set(item.postId, (counts.get(item.postId) ?? 0) + 1);
      }
    }
    return counts;
  }

  function getWatchlistItemsForCategory(
    watchlistMap: Map<string, WatchlistItem[]>,
    category: string
  ): WatchlistItem[] {
    if (category === ALL_CATEGORY_VALUE) {
      const allItems: WatchlistItem[] = [];
      for (const items of watchlistMap.values()) {
        allItems.push(...items);
      }
      return allItems;
    }

    return watchlistMap.get(category) ?? [];
  }

  function normalizeNumber(value: unknown, fallback = 0): number {
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value;
    }

    if (typeof value === 'string') {
      const parsed = Number(value);
      if (Number.isFinite(parsed)) {
        return parsed;
      }
    }

    return fallback;
  }

  function extractMovieStats(post?: Post): MovieStatsLike | null {
    if (!post || typeof post !== 'object') {
      return null;
    }

    const withStats = post as Post & {
      movieStats?: MovieStatsLike;
      movie_stats?: MovieStatsLike;
    };

    return withStats.movieStats ?? withStats.movie_stats ?? null;
  }

  function findMovieLink(post?: Post): Link | null {
    if (!post?.links?.length) {
      return null;
    }

    for (const link of post.links) {
      const metadata = link.metadata as MetadataWithMovie | undefined;
      if (!metadata) {
        continue;
      }
      if (metadata.movie) {
        return link;
      }
      if (metadata.type === 'movie' || metadata.type === 'series') {
        return link;
      }
    }

    return null;
  }

  function isMovieOrSeriesPost(post: Post): boolean {
    if (extractMovieStats(post)) {
      return true;
    }
    return findMovieLink(post) !== null;
  }

  function extractMovieMetadata(link: Link | null): MovieMetadataLike | null {
    if (!link?.metadata) {
      return null;
    }

    const metadata = link.metadata as MetadataWithMovie;
    return metadata.movie ?? null;
  }

  function buildMovieTitle(post?: Post, link?: Link | null): string {
    const metadata = link?.metadata as MetadataWithMovie | undefined;
    const movieMetadata = extractMovieMetadata(link ?? null);

    return movieMetadata?.title ?? metadata?.title ?? post?.content?.trim() ?? 'Movie';
  }

  function buildPoster(link?: Link | null): string | null {
    const metadata = link?.metadata as MetadataWithMovie | undefined;
    const movieMetadata = extractMovieMetadata(link ?? null);

    return movieMetadata?.poster ?? movieMetadata?.backdrop ?? metadata?.image ?? null;
  }

  function buildRating(post?: Post, link?: Link | null): number | null {
    const stats = extractMovieStats(post);
    const movieMetadata = extractMovieMetadata(link ?? null);

    const statsRating = stats?.averageRating ?? stats?.avgRating ?? stats?.avg_rating ?? null;

    if (typeof statsRating === 'number' && Number.isFinite(statsRating)) {
      return statsRating;
    }

    const metadataRating = movieMetadata?.tmdbRating ?? movieMetadata?.tmdb_rating;
    if (typeof metadataRating === 'number' && Number.isFinite(metadataRating)) {
      return metadataRating;
    }

    return null;
  }

  function buildWatchCount(post?: Post): number {
    const stats = extractMovieStats(post);
    return normalizeNumber(stats?.watchCount ?? stats?.watch_count, 0);
  }

  function buildWatchlistCount(
    post: Post | undefined,
    fallbackCounts: Map<string, number>
  ): number {
    const stats = extractMovieStats(post);
    const fromStats = normalizeNumber(stats?.watchlistCount ?? stats?.watchlist_count, -1);

    if (fromStats >= 0) {
      return fromStats;
    }

    if (!post) {
      return 0;
    }

    return fallbackCounts.get(post.id) ?? 0;
  }

  function buildWatchlistMovieItems(
    items: WatchlistItem[],
    watchedIDs: Set<string>,
    fallbackCounts: Map<string, number>
  ): MovieListItem[] {
    return items.map((watchlistItem) => {
      const post = watchlistItem.post;
      const link = findMovieLink(post);

      return {
        postId: watchlistItem.postId,
        title: buildMovieTitle(post, link),
        poster: buildPoster(link),
        rating: buildRating(post, link),
        watchCount: buildWatchCount(post),
        watchlistCount: buildWatchlistCount(post, fallbackCounts),
        watched: watchedIDs.has(watchlistItem.postId),
        createdAt: new Date(watchlistItem.createdAt).getTime(),
      };
    });
  }

  function buildAllMovieItems(
    postList: Post[],
    watchedIDs: Set<string>,
    fallbackCounts: Map<string, number>,
    sectionTypesBySectionID: Map<string, string>,
    targetSectionType: WatchlistSectionType
  ): MovieListItem[] {
    return postList
      .filter(
        (post) =>
          isMovieOrSeriesPost(post) &&
          isPostInSectionType(post, sectionTypesBySectionID, targetSectionType)
      )
      .map((post) => {
        const link = findMovieLink(post);
        return {
          postId: post.id,
          title: buildMovieTitle(post, link),
          poster: buildPoster(link),
          rating: buildRating(post, link),
          watchCount: buildWatchCount(post),
          watchlistCount: buildWatchlistCount(post, fallbackCounts),
          watched: watchedIDs.has(post.id),
          createdAt: new Date(post.createdAt).getTime(),
        };
      });
  }

  function mergeMovies(existing: Post[], incoming: Post[]): Post[] {
    const byID = new Map<string, Post>();
    for (const post of existing) {
      byID.set(post.id, post);
    }
    for (const post of incoming) {
      byID.set(post.id, post);
    }
    return Array.from(byID.values()).sort(
      (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
    );
  }

  async function loadAllMovies(reset = false): Promise<void> {
    if (allMoviesLoading) {
      return;
    }

    allMoviesLoading = true;
    allMoviesError = null;

    try {
      const response = await api.getMoviePosts(
        ALL_MOVIES_PAGE_SIZE,
        reset ? undefined : (allMoviesNextCursor ?? undefined),
        sectionType
      );

      allMoviePosts = reset ? response.posts : mergeMovies(allMoviePosts, response.posts);
      allMoviesHasMore = response.hasMore;
      allMoviesNextCursor = response.nextCursor ?? null;
      allMoviesError = null;
    } catch (error) {
      allMoviesError = error instanceof Error ? error.message : `Failed to load ${mediaLabelPlural}`;
    } finally {
      allMoviesLoading = false;
    }
  }

  function filterMoviesBySearch(items: MovieListItem[], search: string): MovieListItem[] {
    const query = search.trim().toLowerCase();
    if (!query) {
      return items;
    }

    return items.filter((item) => item.title.toLowerCase().includes(query));
  }

  function sortMovies(items: MovieListItem[], sort: SortKey): MovieListItem[] {
    const next = [...items];

    switch (sort) {
      case 'watch_count':
        return next.sort((a, b) => b.watchCount - a.watchCount || b.createdAt - a.createdAt);
      case 'watchlist_count':
        return next.sort(
          (a, b) => b.watchlistCount - a.watchlistCount || b.createdAt - a.createdAt
        );
      case 'date':
        return next.sort((a, b) => b.createdAt - a.createdAt);
      case 'rating':
      default:
        return next.sort((a, b) => (b.rating ?? 0) - (a.rating ?? 0) || b.createdAt - a.createdAt);
    }
  }

  function navigateToPost(postID: string) {
    const href = buildStandaloneThreadHref(postID);
    pushPath(href);
    if (typeof window !== 'undefined') {
      window.dispatchEvent(new PopStateEvent('popstate', { state: window.history.state }));
    }
  }

  function handleCreateCategory() {
    dispatch('createCategory', undefined);
  }

  function clearCategoryError() {
    localCategoryError = null;
  }

  function findEditableCategory(categoryName: string): WatchlistCategory | null {
    return editableCategoriesByName.get(categoryName) ?? null;
  }

  function resetCategoryActionState() {
    editingCategoryId = null;
    editingCategoryName = '';
    deleteCategoryId = null;
    deleteCategoryName = '';
  }

  async function refreshWatchlistView(): Promise<void> {
    await Promise.all([movieStore.loadWatchlistCategories(), movieStore.loadWatchlist(sectionType)]);
  }

  function startEditCategory(categoryName: string) {
    const category = findEditableCategory(categoryName);
    if (!category || isCategoryActionBusy) {
      return;
    }

    deleteCategoryId = null;
    deleteCategoryName = '';
    editingCategoryId = category.id;
    editingCategoryName = category.name;
    clearCategoryError();
  }

  function cancelEditCategory() {
    editingCategoryId = null;
    editingCategoryName = '';
    clearCategoryError();
  }

  async function confirmEditCategory() {
    if (!editingCategoryId) {
      return;
    }

    const trimmedName = editingCategoryName.trim();
    if (!trimmedName) {
      localCategoryError = 'Category name is required.';
      return;
    }

    const existing = editableCategoriesByID.get(editingCategoryId) ?? null;
    if (existing && existing.name === trimmedName) {
      cancelEditCategory();
      return;
    }

    isCategoryActionBusy = true;
    clearCategoryError();
    try {
      await movieStore.updateCategory(editingCategoryId, { name: trimmedName });
      const updateError = get(movieStore).error;
      if (updateError) {
        localCategoryError = updateError;
        return;
      }

      await refreshWatchlistView();
      const refreshError = get(movieStore).error;
      if (refreshError) {
        localCategoryError = refreshError;
        return;
      }

      if (existing && selectedCategory === existing.name) {
        selectedCategory = trimmedName;
      }

      resetCategoryActionState();
      clearCategoryError();
    } finally {
      isCategoryActionBusy = false;
    }
  }

  function startDeleteCategory(categoryName: string) {
    const category = findEditableCategory(categoryName);
    if (!category || isCategoryActionBusy) {
      return;
    }

    editingCategoryId = null;
    editingCategoryName = '';
    deleteCategoryId = category.id;
    deleteCategoryName = category.name;
    clearCategoryError();
  }

  function cancelDeleteCategory() {
    deleteCategoryId = null;
    deleteCategoryName = '';
    clearCategoryError();
  }

  async function confirmDeleteCategory() {
    if (!deleteCategoryId) {
      return;
    }

    const existing = editableCategoriesByID.get(deleteCategoryId) ?? null;
    const deletedCategoryName = deleteCategoryName || existing?.name || '';

    isCategoryActionBusy = true;
    clearCategoryError();
    try {
      await movieStore.deleteCategory(deleteCategoryId);
      const deleteError = get(movieStore).error;
      if (deleteError) {
        localCategoryError = deleteError;
        return;
      }

      await refreshWatchlistView();
      const refreshError = get(movieStore).error;
      if (refreshError) {
        localCategoryError = refreshError;
        return;
      }

      if (deletedCategoryName && selectedCategory === deletedCategoryName) {
        selectedCategory = ALL_CATEGORY_VALUE;
      }

      resetCategoryActionState();
      clearCategoryError();
    } finally {
      isCategoryActionBusy = false;
    }
  }
</script>

<section class="rounded-2xl border border-gray-200 bg-white shadow-sm" data-testid="watchlist">
  <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-4 py-3">
    <div>
      <h2 class="text-base font-semibold text-gray-900">Watchlist</h2>
      <p class="text-xs text-gray-500">
        Track what you want to watch and what your club is rating.
      </p>
    </div>

    <div class="flex items-center gap-2" role="tablist" aria-label="Watchlist views">
      {#each tabOptions as tab}
        <button
          type="button"
          role="tab"
          class={`rounded-full px-3 py-1 text-xs font-semibold transition-colors ${
            activeTab === tab.key ? 'bg-blue-100 text-blue-700' : 'text-gray-600 hover:bg-gray-100'
          }`}
          aria-selected={activeTab === tab.key}
          on:click={() => (activeTab = tab.key)}
          data-testid={`watchlist-tab-${tab.key}`}
        >
          {tab.label}
        </button>
      {/each}
    </div>
  </div>

  <div class="p-4">
    {#if activeTab === 'my'}
      <div class="flex flex-col gap-4 lg:flex-row">
        <aside class="lg:w-64" data-testid="watchlist-category-panel">
          <div class="rounded-xl border border-gray-200 bg-white p-3 shadow-sm">
            <div class="flex items-center justify-between">
              <h3 class="text-sm font-semibold text-gray-900">Categories</h3>
              <span class="text-xs text-gray-500">{totalSavedCount} saved</span>
            </div>

            <div class="mt-3 space-y-1">
              {#each categoryOptions as option}
                {@const editableCategory = findEditableCategory(option.value)}
                <div
                  class="group rounded-lg"
                  data-testid={`watchlist-category-row-${option.value}`}
                >
                  {#if editingCategoryId === editableCategory?.id}
                    <div
                      class="flex flex-wrap items-center gap-2 rounded-lg border border-blue-200 bg-blue-50 px-3 py-2"
                    >
                      <input
                        class="min-w-[8rem] flex-1 rounded-md border border-blue-300 px-2 py-1 text-xs focus:border-blue-500 focus:outline-none"
                        type="text"
                        bind:value={editingCategoryName}
                        placeholder="Category name"
                        data-testid="watchlist-category-edit-input"
                      />
                      <button
                        type="button"
                        class="rounded-md bg-blue-600 px-2 py-1 text-xs font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                        on:click={confirmEditCategory}
                        disabled={isCategoryActionBusy}
                        data-testid="watchlist-category-edit-save"
                      >
                        {isCategoryActionBusy ? 'Saving...' : 'Save'}
                      </button>
                      <button
                        type="button"
                        class="rounded-md border border-blue-200 px-2 py-1 text-xs text-blue-700 hover:bg-blue-100 disabled:cursor-not-allowed disabled:opacity-60"
                        on:click={cancelEditCategory}
                        disabled={isCategoryActionBusy}
                        data-testid="watchlist-category-edit-cancel"
                      >
                        Cancel
                      </button>
                    </div>
                  {:else}
                    <div class="flex items-center gap-1">
                      <button
                        type="button"
                        class={`flex flex-1 items-center justify-between rounded-lg px-3 py-2 text-xs font-semibold transition-colors ${
                          selectedCategory === option.value
                            ? 'bg-blue-50 text-blue-700'
                            : 'text-gray-600 hover:bg-gray-100'
                        }`}
                        on:click={() => (selectedCategory = option.value)}
                        data-testid={`watchlist-category-${option.value}`}
                      >
                        <span>{option.label}</span>
                        <span class="rounded-full bg-white px-2 py-0.5 text-[11px] text-gray-500">
                          {option.value === ALL_CATEGORY_VALUE
                            ? totalSavedCount
                            : (categoryCounts.get(option.value) ?? 0)}
                        </span>
                      </button>

                      {#if editableCategory}
                        <div
                          class="hidden items-center gap-1 group-hover:flex"
                          data-testid={`watchlist-category-actions-${editableCategory.id}`}
                        >
                          <button
                            type="button"
                            class="rounded-md border border-gray-200 px-1.5 py-1 text-[11px] text-gray-500 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
                            aria-label={`Edit ${editableCategory.name}`}
                            on:click={() => startEditCategory(editableCategory.name)}
                            disabled={isCategoryActionBusy}
                            data-testid={`watchlist-category-edit-${editableCategory.id}`}
                          >
                            ⚙
                          </button>
                          <button
                            type="button"
                            class="rounded-md border border-gray-200 px-1.5 py-1 text-[11px] text-gray-500 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
                            aria-label={`Delete ${editableCategory.name}`}
                            on:click={() => startDeleteCategory(editableCategory.name)}
                            disabled={isCategoryActionBusy}
                            data-testid={`watchlist-category-delete-${editableCategory.id}`}
                          >
                            ×
                          </button>
                        </div>
                      {/if}
                    </div>
                  {/if}

                  {#if deleteCategoryId === editableCategory?.id}
                    <div
                      class="mt-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900"
                      data-testid="watchlist-category-delete-confirm"
                    >
                      <p>
                        Delete <span class="font-semibold">{deleteCategoryName}</span>? Saved items
                        will move to
                        <span class="font-semibold"> {DEFAULT_WATCHLIST_CATEGORY}</span>.
                      </p>
                      <div class="mt-2 flex items-center gap-2">
                        <button
                          type="button"
                          class="rounded-md bg-amber-600 px-2 py-1 text-xs font-semibold text-white hover:bg-amber-700 disabled:cursor-not-allowed disabled:opacity-60"
                          on:click={confirmDeleteCategory}
                          disabled={isCategoryActionBusy}
                          data-testid="watchlist-category-delete-confirm-button"
                        >
                          {isCategoryActionBusy ? 'Deleting...' : 'Delete'}
                        </button>
                        <button
                          type="button"
                          class="rounded-md border border-amber-200 px-2 py-1 text-xs text-amber-900 hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-60"
                          on:click={cancelDeleteCategory}
                          disabled={isCategoryActionBusy}
                          data-testid="watchlist-category-delete-cancel-button"
                        >
                          Cancel
                        </button>
                      </div>
                    </div>
                  {/if}
                </div>
              {/each}
            </div>

            {#if displayCategoryError}
              <div
                class="mt-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700"
                data-testid="watchlist-category-error"
              >
                {displayCategoryError}
              </div>
            {/if}

            <button
              type="button"
              class="mt-3 w-full rounded-lg border border-dashed border-gray-300 px-3 py-2 text-xs font-semibold text-gray-600 hover:border-gray-400 hover:bg-gray-50"
              on:click={handleCreateCategory}
              data-testid="watchlist-create-category"
            >
              + Create category
            </button>
          </div>
        </aside>

        <div class="min-w-0 flex-1">
          <div class="flex items-center justify-between">
            <div>
              <h3 class="text-sm font-semibold text-gray-900">{selectedCategoryLabel}</h3>
              <p class="text-xs text-gray-500">{mediaLabelTitle} saved to your list.</p>
            </div>
            <span class="text-xs text-gray-400">{myMovies.length} {mediaLabelPlural}</span>
          </div>

          {#if myMovies.length === 0}
            <div
              class="mt-3 rounded-xl border border-dashed border-gray-200 px-4 py-6 text-sm text-gray-500"
              data-testid="watchlist-empty"
            >
              {#if totalSavedCount === 0}
                No {mediaLabelPlural} saved yet
              {:else if selectedCategory !== ALL_CATEGORY_VALUE}
                No {mediaLabelPlural} in this category
              {:else}
                No {mediaLabelPlural} saved yet
              {/if}
            </div>
          {:else}
            <div
              class="mt-3 grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4"
              data-testid="watchlist-my-grid"
            >
              {#each myMovies as movie}
                <button
                  type="button"
                  class="group relative overflow-hidden rounded-xl border border-gray-200 bg-white text-left shadow-sm transition hover:-translate-y-0.5 hover:border-gray-300"
                  on:click={() => navigateToPost(movie.postId)}
                  data-testid={`watchlist-my-item-${movie.postId}`}
                >
                  {#if movie.watched}
                    <span
                      class="absolute right-2 top-2 z-10 rounded-full bg-green-100 px-2 py-0.5 text-[11px] font-semibold text-green-700"
                      data-testid={`watchlist-watched-${movie.postId}`}
                    >
                      ✓ Watched
                    </span>
                  {/if}

                  <div class="aspect-[2/3] w-full overflow-hidden bg-gray-100">
                    {#if movie.poster}
                      <img
                        src={movie.poster}
                        alt={movie.title}
                        class="h-full w-full object-cover"
                        loading="lazy"
                      />
                    {:else}
                      <div
                        class="flex h-full w-full items-center justify-center px-3 text-center text-xs text-gray-400"
                      >
                        No poster
                      </div>
                    {/if}
                  </div>

                  <div class="space-y-1 px-3 py-3">
                    <h4 class="line-clamp-2 text-sm font-semibold text-gray-900">{movie.title}</h4>
                    <p class="text-xs text-gray-500">
                      {#if movie.rating !== null}
                        ★ {movie.rating.toFixed(1)}
                      {:else}
                        No rating yet
                      {/if}
                    </p>
                  </div>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      </div>
    {:else}
      <div>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h3 class="text-sm font-semibold text-gray-900">All {mediaLabelTitle}</h3>
            <p class="text-xs text-gray-500">
              Browse everything shared in the {mediaLabelPlural} section.
            </p>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <label class="text-xs font-semibold text-gray-600" for="watchlist-search">Search</label>
            <input
              id="watchlist-search"
              type="search"
              class="rounded-lg border border-gray-200 px-3 py-2 text-xs"
              placeholder="Search titles"
              bind:value={searchTerm}
              data-testid="watchlist-search"
            />

            <label class="text-xs font-semibold text-gray-600" for="watchlist-sort">Sort</label>
            <select
              id="watchlist-sort"
              class="rounded-lg border border-gray-200 px-3 py-2 text-xs"
              bind:value={sortKey}
              data-testid="watchlist-sort"
            >
              {#each sortOptions as option}
                <option value={option.key}>{option.label}</option>
              {/each}
            </select>
          </div>
        </div>

        {#if allMoviesLoading && allMovies.length === 0}
          <div
            class="mt-3 rounded-xl border border-dashed border-gray-200 px-4 py-6 text-sm text-gray-500"
            data-testid="watchlist-all-loading"
          >
            Loading {mediaLabelPlural}...
          </div>
        {:else if allMoviesError && allMovies.length === 0}
          <div
            class="mt-3 rounded-xl border border-dashed border-red-200 bg-red-50 px-4 py-6 text-sm text-red-700"
            data-testid="watchlist-all-error"
          >
            <p>{allMoviesError}</p>
            <div class="mt-3">
              <button
                type="button"
                class="rounded-lg border border-red-300 bg-white px-3 py-1.5 text-xs font-semibold text-red-700 hover:bg-red-100"
                on:click={() => void loadAllMovies(true)}
                disabled={allMoviesLoading}
                data-testid="watchlist-all-retry"
              >
                Retry
              </button>
            </div>
          </div>
        {:else if allMovies.length === 0}
          <div
            class="mt-3 rounded-xl border border-dashed border-gray-200 px-4 py-6 text-sm text-gray-500"
            data-testid="watchlist-all-empty"
          >
            No {mediaLabelPlural} available
          </div>
        {:else}
          <div
            class="mt-3 grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4"
            data-testid="watchlist-all-grid"
          >
            {#each allMovies as movie}
              <button
                type="button"
                class="group relative overflow-hidden rounded-xl border border-gray-200 bg-white text-left shadow-sm transition hover:-translate-y-0.5 hover:border-gray-300"
                on:click={() => navigateToPost(movie.postId)}
                data-testid={`watchlist-all-item-${movie.postId}`}
              >
                {#if movie.watched}
                  <span
                    class="absolute right-2 top-2 z-10 rounded-full bg-green-100 px-2 py-0.5 text-[11px] font-semibold text-green-700"
                    data-testid={`watchlist-watched-${movie.postId}`}
                  >
                    ✓ Watched
                  </span>
                {/if}

                <div class="aspect-[2/3] w-full overflow-hidden bg-gray-100">
                  {#if movie.poster}
                    <img
                      src={movie.poster}
                      alt={movie.title}
                      class="h-full w-full object-cover"
                      loading="lazy"
                    />
                  {:else}
                    <div
                      class="flex h-full w-full items-center justify-center px-3 text-center text-xs text-gray-400"
                    >
                      No poster
                    </div>
                  {/if}
                </div>

                <div class="space-y-1 px-3 py-3">
                  <h4 class="line-clamp-2 text-sm font-semibold text-gray-900">{movie.title}</h4>
                  <p class="text-xs text-gray-500">
                    {#if movie.rating !== null}
                      ★ {movie.rating.toFixed(1)}
                    {:else}
                      No rating yet
                    {/if}
                  </p>
                  <p class="text-[11px] text-gray-500">
                    Watched {movie.watchCount} · In lists {movie.watchlistCount}
                  </p>
                </div>
              </button>
            {/each}
          </div>
        {/if}

        {#if allMoviesHasMore}
          <div class="mt-4 flex justify-center">
            <button
              type="button"
              class="rounded-lg border border-gray-200 px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
              on:click={() => void loadAllMovies(false)}
              disabled={allMoviesLoading}
              data-testid="watchlist-all-load-more"
            >
              {allMoviesLoading ? 'Loading...' : 'Load more'}
            </button>
          </div>
        {/if}

        {#if allMoviesError && allMovies.length > 0}
          <p class="mt-3 text-center text-xs text-red-600" data-testid="watchlist-all-error-inline">
            {allMoviesError}
          </p>
        {/if}
      </div>
    {/if}
  </div>
</section>
