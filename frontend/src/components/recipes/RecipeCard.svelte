<script context="module" lang="ts">
  let nextInstanceId = 0;
</script>

<script lang="ts">
  import { onDestroy } from 'svelte';
  import { slide } from 'svelte/transition';
  import type { RecipeMetadata } from '../../stores/postStore';

  export let recipe: RecipeMetadata;
  export let fallbackImage: string | null = null;
  export let fallbackTitle: string | null = null;

  let isExpanded = false;
  let checkedIngredients = new Set<number>();
  let copiedIngredients = false;
  let copyTimeout: ReturnType<typeof setTimeout> | null = null;
  const instanceId = ++nextInstanceId;

  const DEFAULT_TITLE = 'Recipe';

  $: ingredients = recipe?.ingredients ?? [];
  $: instructions = recipe?.instructions ?? [];
  $: imageUrl = recipe?.image || fallbackImage;
  $: recipeTitle = recipe?.name || fallbackTitle || DEFAULT_TITLE;
  $: timeSegments = [
    recipe?.prep_time ? `Prep: ${recipe.prep_time}` : null,
    recipe?.cook_time ? `Cook: ${recipe.cook_time}` : null,
  ].filter(Boolean);
  $: timeLabel = timeSegments.length > 0 ? timeSegments.join(' • ') : null;
  $: totalTimeLabel = !timeLabel && recipe?.total_time ? `Total: ${recipe.total_time}` : null;
  $: yieldLabel = recipe?.yield ? `Serves ${recipe.yield}` : null;
  $: nutrition = recipe?.nutrition ?? null;

  function toggleIngredient(index: number) {
    const next = new Set(checkedIngredients);
    if (next.has(index)) {
      next.delete(index);
    } else {
      next.add(index);
    }
    checkedIngredients = next;
  }

  async function copyIngredients() {
    if (typeof window === 'undefined' || ingredients.length === 0) {
      return;
    }

    const text = ingredients.join('\n');
    let copied = false;

    if (navigator.clipboard?.writeText) {
      try {
        await navigator.clipboard.writeText(text);
        copied = true;
      } catch {
        copied = false;
      }
    }

    if (!copied && typeof document !== 'undefined' && typeof document.execCommand === 'function') {
      const textarea = document.createElement('textarea');
      textarea.value = text;
      textarea.setAttribute('readonly', '');
      textarea.style.position = 'absolute';
      textarea.style.left = '-9999px';
      document.body.appendChild(textarea);
      textarea.select();
      copied = document.execCommand('copy');
      document.body.removeChild(textarea);
    }

    if (copied) {
      copiedIngredients = true;
      if (copyTimeout) {
        clearTimeout(copyTimeout);
      }
      copyTimeout = setTimeout(() => {
        copiedIngredients = false;
      }, 2000);
    }
  }

  onDestroy(() => {
    if (copyTimeout) {
      clearTimeout(copyTimeout);
    }
  });
</script>

<article class="recipe-card mt-3 overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm">
  {#if imageUrl}
    <div class="h-48 w-full overflow-hidden bg-gray-100 sm:h-56">
      <img src={imageUrl} alt={recipeTitle} class="h-full w-full object-cover" loading="lazy" />
    </div>
  {/if}

  <div class="p-4 sm:p-5">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <h3 class="text-lg font-semibold text-gray-900" data-testid="recipe-title">{recipeTitle}</h3>
        {#if recipe?.description}
          <p class="mt-1 text-sm text-gray-600">{recipe.description}</p>
        {/if}
        {#if timeLabel || totalTimeLabel}
          <p class="mt-2 text-sm text-gray-700" data-testid="recipe-time">
            {timeLabel ?? totalTimeLabel}
          </p>
        {/if}
        {#if yieldLabel}
          <p class="mt-1 text-sm text-gray-700" data-testid="recipe-yield">{yieldLabel}</p>
        {/if}
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <button
          type="button"
          class="rounded-full border border-gray-200 px-3 py-1 text-xs font-medium text-gray-700 hover:border-gray-300 hover:bg-gray-50"
          on:click={() => (isExpanded = !isExpanded)}
          data-testid="recipe-toggle"
        >
          {isExpanded ? 'Collapse' : 'View Recipe'}
        </button>
      </div>
    </div>

    {#if isExpanded}
      <div class="mt-4 space-y-5" transition:slide>
        {#if ingredients.length > 0}
          <section>
            <div class="flex flex-wrap items-center justify-between gap-2">
              <h4 class="text-sm font-semibold uppercase tracking-wide text-gray-500">Ingredients</h4>
              <button
                type="button"
                class="relative rounded-full border border-blue-100 bg-blue-50 px-3 py-1 text-xs font-semibold text-blue-700 hover:bg-blue-100"
                on:click={copyIngredients}
                disabled={ingredients.length === 0}
                data-testid="recipe-copy"
              >
                Copy ingredients
                {#if copiedIngredients}
                  <span
                    class="absolute -top-6 right-0 rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] text-emerald-700 shadow"
                    role="status"
                    aria-live="polite"
                  >
                    Copied
                  </span>
                {/if}
              </button>
            </div>
            <ul class="mt-3 space-y-2">
              {#each ingredients as ingredient, index}
                <li class="flex items-start gap-2 rounded-lg border border-gray-100 bg-gray-50 px-3 py-2">
                  <input
                    id={`ingredient-${instanceId}-${index}`}
                    type="checkbox"
                    class="mt-1 h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                    checked={checkedIngredients.has(index)}
                    on:change={() => toggleIngredient(index)}
                  />
                  <label for={`ingredient-${instanceId}-${index}`} class="text-sm text-gray-800">
                    {ingredient}
                  </label>
                </li>
              {/each}
            </ul>
          </section>
        {/if}

        {#if instructions.length > 0}
          <section>
            <h4 class="text-sm font-semibold uppercase tracking-wide text-gray-500">Instructions</h4>
            <ol class="mt-3 space-y-3">
              {#each instructions as instruction}
                <li class="rounded-lg border border-gray-100 bg-white px-3 py-2 text-sm text-gray-800 shadow-sm">
                  {instruction}
                </li>
              {/each}
            </ol>
          </section>
        {/if}

        {#if nutrition && (nutrition.calories || nutrition.servings)}
          <section>
            <h4 class="text-sm font-semibold uppercase tracking-wide text-gray-500">Nutrition</h4>
            <div class="mt-2 grid gap-2 text-sm text-gray-700 sm:grid-cols-2">
              {#if nutrition.calories}
                <div class="rounded-lg border border-amber-100 bg-amber-50 px-3 py-2">
                  Calories: {nutrition.calories}
                </div>
              {/if}
              {#if nutrition.servings}
                <div class="rounded-lg border border-emerald-100 bg-emerald-50 px-3 py-2">
                  Servings: {nutrition.servings}
                </div>
              {/if}
            </div>
          </section>
        {/if}

      </div>
    {/if}
  </div>
</article>
