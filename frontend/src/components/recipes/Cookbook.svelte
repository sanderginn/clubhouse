<script lang="ts">
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import type { Post, Link } from '../../stores/postStore';
  import type { CookLog, SavedRecipe } from '../../stores/recipeStore';
  import { posts } from '../../stores/postStore';
  import {
    recipeStore,
    savedRecipesByCategory,
    sortedCategories,
  } from '../../stores/recipeStore';
  import { buildStandaloneThreadHref, pushPath } from '../../services/routeNavigation';
  import CategoryManager from './CategoryManager.svelte';
  import RatingStars from './RatingStars.svelte';

  type TabKey = 'my' | 'all';
  type SortKey = 'rating' | 'date' | 'cooked';

  const ALL_CATEGORY_VALUE = '__all__';
  const ALL_CATEGORY_LABEL = 'All recipes';

  let activeTab: TabKey = 'my';
  let selectedCategory = ALL_CATEGORY_VALUE;
  let isCollapsed = false;
  let sortKey: SortKey = 'rating';

  const tabOptions: Array<{ key: TabKey; label: string }> = [
    { key: 'my', label: 'My recipes' },
    { key: 'all', label: 'All recipes' },
  ];

  const sortOptions: Array<{ key: SortKey; label: string }> = [
    { key: 'rating', label: 'By rating' },
    { key: 'date', label: 'By date shared' },
    { key: 'cooked', label: 'By times cooked' },
  ];

  onMount(() => {
    const state = get(recipeStore);
    if (!state.isLoadingCategories && state.categories.length === 0) {
      recipeStore.loadCategories();
    }
    if (!state.isLoadingSaved && state.savedRecipes.size === 0) {
      recipeStore.loadSavedRecipes();
    }
    if (!state.isLoadingCookLogs && state.cookLogs.length === 0) {
      recipeStore.loadCookLogs();
    }
  });

  $: categories = $sortedCategories;
  $: categoryOptions = [
    { value: ALL_CATEGORY_VALUE, label: ALL_CATEGORY_LABEL },
    ...categories.map((category) => ({ value: category.name, label: category.name })),
  ];
  $: categoryCounts = buildCategoryCounts($savedRecipesByCategory);
  $: totalSavedCount = Array.from(categoryCounts.values()).reduce((total, count) => total + count, 0);
  $: selectedSavedRecipes = getSavedRecipesForCategory($savedRecipesByCategory, selectedCategory);
  $: cookLogRatings = buildCookLogRatings($recipeStore.cookLogs);
  $: myRecipeItems = buildSavedRecipeItems(selectedSavedRecipes, cookLogRatings);
  $: sortedMyRecipes = [...myRecipeItems].sort((a, b) => b.createdAt - a.createdAt);
  $: allRecipeItems = buildAllRecipeItems($posts, cookLogRatings);
  $: sortedAllRecipes = sortAllRecipes(allRecipeItems, sortKey);
  $: selectedCategoryLabel =
    categoryOptions.find((option) => option.value === selectedCategory)?.label ?? ALL_CATEGORY_LABEL;

  $: if (
    selectedCategory !== ALL_CATEGORY_VALUE &&
    !categoryOptions.some((option) => option.value === selectedCategory)
  ) {
    selectedCategory = ALL_CATEGORY_VALUE;
  }

  function buildCategoryCounts(map: Map<string, SavedRecipe[]>): Map<string, number> {
    const counts = new Map<string, number>();
    for (const [name, recipes] of map.entries()) {
      counts.set(name, recipes.length);
    }
    return counts;
  }

  function getSavedRecipesForCategory(
    map: Map<string, SavedRecipe[]>,
    category: string
  ): SavedRecipe[] {
    if (category === ALL_CATEGORY_VALUE) {
      const all: SavedRecipe[] = [];
      for (const recipes of map.values()) {
        all.push(...recipes);
      }
      return all;
    }
    return map.get(category) ?? [];
  }

  type RecipeListItem = {
    postId: string;
    title: string;
    image: string | null;
    author: string | null;
    summary: string | null;
    source: string | null;
    cookTime: string | null;
    averageRating: number | null;
    cookCount: number;
    saveCount: number;
    userRating: number | null;
    createdAt: number;
  };

  type CookStats = { avgRating: number | null; cookCount: number };

  function normalizeNumber(value: unknown, fallback: number): number {
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

  function extractCookStats(post: Post): CookStats {
    const withCookInfo = post as Post & {
      cookInfo?: { avgRating?: number | null; cookCount?: number | null };
      cook_info?: { avg_rating?: number | null; cook_count?: number | null };
    };
    const recipeStats = post.recipeStats ?? post.recipe_stats ?? null;

    const avgRating =
      withCookInfo.cookInfo?.avgRating ??
      withCookInfo.cook_info?.avg_rating ??
      recipeStats?.averageRating ??
      null;
    const cookCount =
      withCookInfo.cookInfo?.cookCount ??
      withCookInfo.cook_info?.cook_count ??
      recipeStats?.cookCount ??
      0;

    return {
      avgRating: avgRating === null ? null : normalizeNumber(avgRating, 0),
      cookCount: normalizeNumber(cookCount, 0),
    };
  }

  function buildCookLogRatings(logs: CookLog[]): Map<string, number> {
    const ratings = new Map<string, number>();
    for (const log of logs) {
      if (log.deletedAt) {
        continue;
      }
      const rating = normalizeNumber(log.rating, 0);
      if (rating > 0) {
        ratings.set(log.postId, rating);
      }
    }
    return ratings;
  }

  function extractSaveCount(post: Post): number {
    const withSaveInfo = post as Post & {
      saveInfo?: { saveCount?: number | null };
      save_info?: { save_count?: number | null };
    };

    const recipeStats = post.recipeStats ?? post.recipe_stats ?? null;
    const saveCount =
      withSaveInfo.saveInfo?.saveCount ?? withSaveInfo.save_info?.save_count ?? recipeStats?.saveCount ?? 0;
    return normalizeNumber(saveCount, 0);
  }

  function findRecipeLink(post: Post): Link | null {
    return (
      post.links?.find((link) => {
        const metadata = link.metadata;
        return !!metadata?.recipe || metadata?.type === 'recipe';
      }) ??
      post.links?.find((link) => !!link.metadata) ??
      null
    );
  }

  function buildRecipeTitle(post: Post, link: Link | null): string {
    const metadata = link?.metadata;
    return (
      metadata?.recipe?.name ??
      metadata?.title ??
      post.content?.trim() ??
      'Recipe'
    );
  }

  function buildRecipeImage(link: Link | null): string | null {
    const metadata = link?.metadata;
    return metadata?.recipe?.image ?? metadata?.image ?? null;
  }

  function buildRecipeAuthor(post: Post, link: Link | null): string | null {
    const metadata = link?.metadata;
    return metadata?.recipe?.author ?? post.user?.username ?? null;
  }

  function buildRecipeSummary(link: Link | null): string | null {
    const metadata = link?.metadata;
    return metadata?.recipe?.description ?? metadata?.description ?? null;
  }

  function buildRecipeSource(link: Link | null): string | null {
    const metadata = link?.metadata;
    const url = metadata?.url ?? link?.url;
    if (!url) {
      return null;
    }
    try {
      return new URL(url).hostname.replace(/^www\./, '');
    } catch {
      return null;
    }
  }

  function buildRecipeCookTime(link: Link | null): string | null {
    const metadata = link?.metadata;
    return metadata?.recipe?.cook_time ?? metadata?.recipe?.total_time ?? null;
  }

  function buildSavedRecipeItems(
    recipes: SavedRecipe[],
    ratingLookup: Map<string, number>
  ): RecipeListItem[] {
    return recipes.map((saved) => {
      const post = saved.post;
      const link = post ? findRecipeLink(post) : null;
      const cookStats = post ? extractCookStats(post) : { avgRating: null, cookCount: 0 };
      const userRating = ratingLookup.get(saved.postId) ?? null;
      return {
        postId: saved.postId,
        title: post ? buildRecipeTitle(post, link) : 'Recipe',
        image: post ? buildRecipeImage(link) : null,
        author: post ? buildRecipeAuthor(post, link) : null,
        summary: post ? buildRecipeSummary(link) : null,
        source: post ? buildRecipeSource(link) : null,
        cookTime: post ? buildRecipeCookTime(link) : null,
        averageRating: cookStats.avgRating,
        cookCount: cookStats.cookCount,
        saveCount: post ? extractSaveCount(post) : 0,
        userRating,
        createdAt: new Date(saved.createdAt).getTime(),
      };
    });
  }

  function buildAllRecipeItems(
    postList: Post[],
    ratingLookup: Map<string, number>
  ): RecipeListItem[] {
    return postList
      .map((post) => {
        const link = findRecipeLink(post);
        if (!link) {
          return null;
        }
        const cookStats = extractCookStats(post);
        const userRating = ratingLookup.get(post.id) ?? null;
        return {
          postId: post.id,
          title: buildRecipeTitle(post, link),
          image: buildRecipeImage(link),
          author: buildRecipeAuthor(post, link),
          summary: buildRecipeSummary(link),
          source: buildRecipeSource(link),
          cookTime: buildRecipeCookTime(link),
          averageRating: cookStats.avgRating,
          cookCount: cookStats.cookCount,
          saveCount: extractSaveCount(post),
          userRating,
          createdAt: new Date(post.createdAt).getTime(),
        };
      })
      .filter(Boolean) as RecipeListItem[];
  }

  function sortAllRecipes(items: RecipeListItem[], sort: SortKey): RecipeListItem[] {
    const next = [...items];
    switch (sort) {
      case 'rating':
        return next.sort((a, b) => (b.averageRating ?? 0) - (a.averageRating ?? 0));
      case 'cooked':
        return next.sort((a, b) => b.cookCount - a.cookCount);
      case 'date':
      default:
        return next.sort((a, b) => b.createdAt - a.createdAt);
    }
  }

  function navigateToPost(postId: string) {
    const href = buildStandaloneThreadHref(postId);
    pushPath(href);
    if (typeof window !== 'undefined') {
      window.dispatchEvent(new PopStateEvent('popstate', { state: window.history.state }));
    }
  }
</script>

<section class="rounded-2xl border border-gray-200 bg-white shadow-sm" data-testid="cookbook">
  <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-4 py-3">
    <div class="flex items-center gap-3">
      <div>
        <h2 class="text-base font-semibold text-gray-900">Cookbook</h2>
        <p class="text-xs text-gray-500">Your personalized recipe hub.</p>
      </div>
      <button
        type="button"
        class="rounded-full border border-gray-200 px-3 py-1 text-xs font-semibold text-gray-600 hover:border-gray-300 hover:bg-gray-50"
        on:click={() => (isCollapsed = !isCollapsed)}
        data-testid="cookbook-collapse"
        aria-expanded={!isCollapsed}
      >
        {isCollapsed ? 'Expand' : 'Collapse'}
      </button>
    </div>
    <div class="flex items-center gap-2" role="tablist" aria-label="Cookbook views">
      {#each tabOptions as tab}
        <button
          type="button"
          role="tab"
          class={`rounded-full px-3 py-1 text-xs font-semibold transition-colors ${
            activeTab === tab.key
              ? 'bg-emerald-100 text-emerald-700'
              : 'text-gray-600 hover:bg-gray-100'
          }`}
          aria-selected={activeTab === tab.key}
          on:click={() => (activeTab = tab.key)}
          data-testid={`cookbook-tab-${tab.key}`}
        >
          {tab.label}
        </button>
      {/each}
    </div>
  </div>

  {#if !isCollapsed}
    <div class="p-4">
      {#if activeTab === 'my'}
        <div class="flex flex-col gap-4 sm:flex-row">
          <div class="sm:w-64" data-testid="cookbook-category-panel">
            <div class="rounded-xl border border-gray-200 bg-white p-3 shadow-sm">
              <div class="flex items-center justify-between">
                <h3 class="text-sm font-semibold text-gray-900">Categories</h3>
                <span class="text-xs text-gray-500">{totalSavedCount} saved</span>
              </div>

              {#if categories.length === 0}
                <p class="mt-3 rounded-lg border border-dashed border-gray-200 px-3 py-3 text-xs text-gray-500">
                  Create your first category
                </p>
              {/if}

              <div class="mt-3 sm:hidden">
                <label class="text-xs font-semibold text-gray-600" for="cookbook-category-select">Category</label>
                <select
                  id="cookbook-category-select"
                  class="mt-1 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm"
                  bind:value={selectedCategory}
                  data-testid="cookbook-category-select"
                >
                  {#each categoryOptions as option}
                    <option value={option.value}>
                      {option.label} {option.value === ALL_CATEGORY_VALUE ? `(${totalSavedCount})` : `(${categoryCounts.get(option.value) ?? 0})`}
                    </option>
                  {/each}
                </select>
              </div>

              <div class="mt-3 hidden space-y-1 sm:block">
                {#each categoryOptions as option}
                  <button
                    type="button"
                    class={`flex w-full items-center justify-between rounded-lg px-3 py-2 text-xs font-semibold transition-colors ${
                      selectedCategory === option.value
                        ? 'bg-emerald-50 text-emerald-700'
                        : 'text-gray-600 hover:bg-gray-100'
                    }`}
                    on:click={() => (selectedCategory = option.value)}
                    data-testid={`cookbook-category-${option.value}`}
                  >
                    <span>{option.label}</span>
                    <span class="rounded-full bg-white px-2 py-0.5 text-[11px] text-gray-500">
                      {option.value === ALL_CATEGORY_VALUE
                        ? totalSavedCount
                        : categoryCounts.get(option.value) ?? 0}
                    </span>
                  </button>
                {/each}
              </div>
            </div>

            <div class="mt-4" data-testid="cookbook-category-manager">
              <CategoryManager />
            </div>
          </div>

          <div class="flex-1">
            <div class="flex items-center justify-between">
              <div>
                <h3 class="text-sm font-semibold text-gray-900">{selectedCategoryLabel}</h3>
                <p class="text-xs text-gray-500">Saved recipes at a glance.</p>
              </div>
              <span class="text-xs text-gray-400">{sortedMyRecipes.length} recipes</span>
            </div>

            <div class="mt-3 max-h-[520px] space-y-3 overflow-y-auto pr-1" data-testid="cookbook-my-list">
              {#if sortedMyRecipes.length === 0}
                <div class="rounded-xl border border-dashed border-gray-200 px-4 py-5 text-sm text-gray-500">
                  No recipes saved here yet
                </div>
              {:else}
                {#each sortedMyRecipes as recipe}
                  <button
                    type="button"
                    class="flex w-full items-center gap-3 rounded-xl border border-gray-200 bg-white p-3 text-left shadow-sm transition hover:border-gray-300"
                    on:click={() => navigateToPost(recipe.postId)}
                    data-testid={`my-recipe-item-${recipe.postId}`}
                  >
                    <div class="h-16 w-16 overflow-hidden rounded-lg bg-gray-100">
                      {#if recipe.image}
                        <img src={recipe.image} alt={recipe.title} class="h-full w-full object-cover" loading="lazy" />
                      {:else}
                        <div class="flex h-full w-full items-center justify-center text-xs text-gray-400">
                          No image
                        </div>
                      {/if}
                    </div>
                    <div class="min-w-0 flex-1">
                      <h4 class="text-sm font-semibold text-gray-900">{recipe.title}</h4>
                      {#if recipe.summary}
                        <p class="mt-1 text-xs text-gray-500">{recipe.summary}</p>
                      {/if}
                      {#if recipe.source || recipe.cookTime}
                        <div class="mt-1 flex flex-wrap items-center gap-2 text-[11px] text-gray-500">
                          {#if recipe.source}
                            <span class="rounded-full bg-gray-100 px-2 py-0.5">{recipe.source}</span>
                          {/if}
                          {#if recipe.cookTime}
                            <span class="rounded-full bg-gray-100 px-2 py-0.5">{recipe.cookTime}</span>
                          {/if}
                        </div>
                      {/if}
                      {#if recipe.author}
                        <p class="mt-0.5 text-xs text-gray-500">by {recipe.author}</p>
                      {/if}
                      <div class="mt-1 flex flex-wrap items-center gap-3 text-xs text-gray-500">
                        <div class="flex items-center gap-2">
                          {#if recipe.averageRating !== null}
                            <RatingStars value={recipe.averageRating} readonly size="sm" ariaLabel="Average rating" />
                            <span>{recipe.averageRating.toFixed(1)}</span>
                          {:else}
                            <span>No ratings yet</span>
                          {/if}
                        </div>
                        <span>Cooked {recipe.cookCount}</span>
                      </div>
                      {#if recipe.userRating !== null}
                        <div class="mt-1 flex items-center gap-2 text-xs text-amber-700">
                          <span class="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide">
                            Your rating
                          </span>
                          <RatingStars value={recipe.userRating} readonly size="sm" ariaLabel="Your rating" />
                          <span class="font-semibold">{recipe.userRating.toFixed(1)}</span>
                        </div>
                      {/if}
                    </div>
                  </button>
                {/each}
              {/if}
            </div>
          </div>
        </div>
      {:else}
        <div>
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h3 class="text-sm font-semibold text-gray-900">All recipes</h3>
              <p class="text-xs text-gray-500">Explore everything your community shared.</p>
            </div>
            <div class="flex items-center gap-2">
              <label class="text-xs font-semibold text-gray-600" for="cookbook-sort">Sort</label>
              <select
                id="cookbook-sort"
                class="rounded-lg border border-gray-200 px-3 py-2 text-xs"
                bind:value={sortKey}
                data-testid="cookbook-sort"
              >
                {#each sortOptions as option}
                  <option value={option.key}>{option.label}</option>
                {/each}
              </select>
            </div>
          </div>

          <div class="mt-3 max-h-[520px] space-y-3 overflow-y-auto pr-1" data-testid="cookbook-all-list">
            {#if sortedAllRecipes.length === 0}
              <div class="rounded-xl border border-dashed border-gray-200 px-4 py-5 text-sm text-gray-500">
                No recipes shared yet
              </div>
            {:else}
              {#each sortedAllRecipes as recipe}
                <button
                  type="button"
                  class="flex w-full items-center gap-3 rounded-xl border border-gray-200 bg-white p-3 text-left shadow-sm transition hover:border-gray-300"
                  on:click={() => navigateToPost(recipe.postId)}
                  data-testid={`all-recipe-item-${recipe.postId}`}
                >
                  <div class="h-16 w-16 overflow-hidden rounded-lg bg-gray-100">
                    {#if recipe.image}
                      <img src={recipe.image} alt={recipe.title} class="h-full w-full object-cover" loading="lazy" />
                    {:else}
                      <div class="flex h-full w-full items-center justify-center text-xs text-gray-400">
                        No image
                      </div>
                    {/if}
                  </div>
                  <div class="min-w-0 flex-1">
                    <h4 class="text-sm font-semibold text-gray-900">{recipe.title}</h4>
                    {#if recipe.summary}
                      <p class="mt-1 text-xs text-gray-500">{recipe.summary}</p>
                    {/if}
                    <p class="mt-0.5 text-xs text-gray-500">{recipe.author ?? 'Unknown author'}</p>
                    {#if recipe.source || recipe.cookTime}
                      <div class="mt-1 flex flex-wrap items-center gap-2 text-[11px] text-gray-500">
                        {#if recipe.source}
                          <span class="rounded-full bg-gray-100 px-2 py-0.5">{recipe.source}</span>
                        {/if}
                        {#if recipe.cookTime}
                          <span class="rounded-full bg-gray-100 px-2 py-0.5">{recipe.cookTime}</span>
                        {/if}
                      </div>
                    {/if}
                    <div class="mt-1 flex flex-wrap items-center gap-3 text-xs text-gray-500">
                      <div class="flex items-center gap-2">
                        {#if recipe.averageRating !== null}
                          <RatingStars value={recipe.averageRating} readonly size="sm" ariaLabel="Average rating" />
                          <span>{recipe.averageRating.toFixed(1)}</span>
                        {:else}
                          <span>No ratings yet</span>
                        {/if}
                      </div>
                      <span>Saved {recipe.saveCount}</span>
                      <span>Cooked {recipe.cookCount}</span>
                    </div>
                  </div>
                </button>
              {/each}
            {/if}
          </div>
        </div>
      {/if}
    </div>
  {/if}
</section>
