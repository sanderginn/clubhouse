<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api, type ApiReactionUser, type CookLogUser } from '../../services/api';
  import CookButton from './CookButton.svelte';
  import RatingStars from './RatingStars.svelte';
  import RecipeSaveButton from './RecipeSaveButton.svelte';

  export let postId: string;
  export let saveCount = 0;
  export let cookCount = 0;
  export let averageRating: number | null = null;
  export let showEmpty = false;

  let saveTooltipContainer: HTMLDivElement | null = null;
  let cookTooltipContainer: HTMLDivElement | null = null;
  let showSaveTooltip = false;
  let showCookTooltip = false;
  let saveTooltipLoading = false;
  let cookTooltipLoading = false;
  let saveTooltipError: string | null = null;
  let cookTooltipError: string | null = null;
  let saveTooltipUsers: ApiReactionUser[] = [];
  let cookTooltipUsers: CookLogUser[] = [];
  let saveTooltipTimeout: ReturnType<typeof setTimeout> | null = null;
  let cookTooltipTimeout: ReturnType<typeof setTimeout> | null = null;
  let lastSaveKey = '';
  let lastCookKey = '';

  const TOOLTIP_DELAY = 150;

  const clampRating = (value: number) => Math.min(5, Math.max(0, value));

  $: normalizedSaveCount = Math.max(0, saveCount);
  $: normalizedCookCount = Math.max(0, cookCount);
  $: normalizedRating = averageRating === null ? null : clampRating(averageRating);
  $: hasSaves = normalizedSaveCount > 0;
  $: hasCooks = normalizedCookCount > 0;
  $: hasRating = normalizedRating !== null && normalizedRating > 0;
  $: hasStats = hasSaves || hasCooks || hasRating;
  $: formattedRating =
    normalizedRating === null
      ? ''
      : Number.isInteger(normalizedRating)
        ? normalizedRating.toFixed(0)
        : normalizedRating.toFixed(1);

  $: saveKey = `${postId}:${normalizedSaveCount}`;
  $: cookKey = `${postId}:${normalizedCookCount}:${normalizedRating ?? 'none'}`;

  function getInitial(username?: string | null) {
    const trimmed = username?.trim();
    return trimmed && trimmed.length > 0 ? trimmed.charAt(0).toUpperCase() : '?';
  }

  function handleFocusOut(event: FocusEvent, container: HTMLDivElement | null, hide: () => void) {
    if (container && event.relatedTarget) {
      const target = event.relatedTarget as Node;
      if (container.contains(target)) {
        return;
      }
    }
    hide();
  }

  function hideSaveTooltip() {
    showSaveTooltip = false;
    if (saveTooltipTimeout) {
      clearTimeout(saveTooltipTimeout);
      saveTooltipTimeout = null;
    }
  }

  function hideCookTooltip() {
    showCookTooltip = false;
    if (cookTooltipTimeout) {
      clearTimeout(cookTooltipTimeout);
      cookTooltipTimeout = null;
    }
  }

  async function loadSaveTooltipData(force = false) {
    if (!postId || !hasSaves) return;
    if (!force && saveTooltipUsers.length > 0 && saveKey === lastSaveKey) return;

    saveTooltipLoading = true;
    saveTooltipError = null;

    try {
      const response = await api.getPostSaves(postId);
      saveTooltipUsers = response.users ?? [];
      lastSaveKey = saveKey;
    } catch (error) {
      saveTooltipError = error instanceof Error ? error.message : 'Failed to load saves.';
    } finally {
      saveTooltipLoading = false;
    }
  }

  async function loadCookTooltipData(force = false) {
    if (!postId || !hasCooks) return;
    if (!force && cookTooltipUsers.length > 0 && cookKey === lastCookKey) return;

    cookTooltipLoading = true;
    cookTooltipError = null;

    try {
      const response = await api.getPostCookLogs(postId);
      cookTooltipUsers = response.users ?? [];
      lastCookKey = cookKey;
    } catch (error) {
      cookTooltipError = error instanceof Error ? error.message : 'Failed to load cooks.';
    } finally {
      cookTooltipLoading = false;
    }
  }

  function showSaveTooltipWithDelay() {
    if (!postId || !hasSaves) return;
    if (saveTooltipTimeout) {
      clearTimeout(saveTooltipTimeout);
    }
    saveTooltipTimeout = setTimeout(async () => {
      showSaveTooltip = true;
      await loadSaveTooltipData();
    }, TOOLTIP_DELAY);
  }

  function showCookTooltipWithDelay() {
    if (!postId || !hasCooks) return;
    if (cookTooltipTimeout) {
      clearTimeout(cookTooltipTimeout);
    }
    cookTooltipTimeout = setTimeout(async () => {
      showCookTooltip = true;
      await loadCookTooltipData();
    }, TOOLTIP_DELAY);
  }

  onDestroy(() => {
    if (saveTooltipTimeout) {
      clearTimeout(saveTooltipTimeout);
    }
    if (cookTooltipTimeout) {
      clearTimeout(cookTooltipTimeout);
    }
  });
</script>

{#if hasStats || showEmpty}
  <div
    class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-gray-200 bg-white px-3 py-2 text-xs text-gray-600"
    data-testid="recipe-stats-bar"
  >
    <div class="flex flex-wrap items-center gap-3">
      {#if hasStats}
        {#if hasSaves}
          <div
            class="relative"
            role="group"
            bind:this={saveTooltipContainer}
            on:mouseenter={showSaveTooltipWithDelay}
            on:mouseleave={hideSaveTooltip}
            on:focusin={showSaveTooltipWithDelay}
            on:focusout={(event) => handleFocusOut(event, saveTooltipContainer, hideSaveTooltip)}
            data-testid="recipe-save-stat"
          >
            <div class="inline-flex items-center gap-1.5">
              <span class="inline-flex h-6 w-6 items-center justify-center rounded-full bg-amber-50 text-amber-600">
                <svg
                  viewBox="0 0 20 20"
                  class="h-3.5 w-3.5"
                  fill="currentColor"
                  aria-hidden="true"
                >
                  <path d="M5 2.5A2.5 2.5 0 0 0 2.5 5v12l7.5-3.5L17.5 17V5A2.5 2.5 0 0 0 15 2.5H5z" />
                </svg>
              </span>
              <span class="font-semibold text-gray-800">{normalizedSaveCount}</span>
              <span>saved</span>
            </div>

            {#if showSaveTooltip}
              <div
                class="absolute left-0 top-full z-20 mt-2 w-64 rounded-lg border border-gray-200 bg-white p-3 text-xs shadow-lg"
                role="tooltip"
                data-testid="recipe-save-tooltip"
              >
                {#if saveTooltipLoading}
                  <p class="text-gray-500">Loading saves...</p>
                {:else if saveTooltipError}
                  <p class="text-red-500">{saveTooltipError}</p>
                {:else if saveTooltipUsers.length === 0}
                  <p class="text-gray-500">No saves yet.</p>
                {:else}
                  <div class="space-y-2">
                    {#each saveTooltipUsers as user}
                      <div class="flex items-center gap-2">
                        {#if user.profile_picture_url}
                          <img
                            src={user.profile_picture_url}
                            alt={user.username}
                            class="h-7 w-7 rounded-full object-cover"
                          />
                        {:else}
                          <div class="h-7 w-7 rounded-full bg-gray-200 text-gray-500 flex items-center justify-center text-xs font-semibold">
                            {getInitial(user.username)}
                          </div>
                        {/if}
                        <span class="text-gray-700">{user.username || 'Unknown'}</span>
                      </div>
                    {/each}
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        {/if}

        {#if hasCooks}
          <div
            class="relative"
            role="group"
            bind:this={cookTooltipContainer}
            on:mouseenter={showCookTooltipWithDelay}
            on:mouseleave={hideCookTooltip}
            on:focusin={showCookTooltipWithDelay}
            on:focusout={(event) => handleFocusOut(event, cookTooltipContainer, hideCookTooltip)}
            data-testid="recipe-cook-stat"
          >
            <div class="inline-flex items-center gap-1.5">
              <span class="inline-flex h-6 w-6 items-center justify-center rounded-full bg-emerald-50 text-emerald-600">
                <svg
                  viewBox="0 0 24 24"
                  class="h-3.5 w-3.5"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.8"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  aria-hidden="true"
                >
                  <path d="M4 10h16" />
                  <path d="M6 10l1.2 9h9.6l1.2-9" />
                  <path d="M9 5h6" />
                  <path d="M10 5v3" />
                  <path d="M14 5v3" />
                </svg>
              </span>
              <span class="font-semibold text-gray-800">{normalizedCookCount}</span>
              <span>cooked</span>
              {#if hasRating}
                <span class="text-gray-300">•</span>
                <span class="inline-flex items-center gap-1 text-gray-700">
                  <svg
                    viewBox="0 0 20 20"
                    class="h-3.5 w-3.5 text-amber-400"
                    fill="currentColor"
                    aria-hidden="true"
                  >
                    <path d="M10 1.5l2.47 5.4 5.88.5-4.42 3.83 1.33 5.77L10 13.9 4.74 17l1.33-5.77L1.65 7.4l5.88-.5L10 1.5z" />
                  </svg>
                  <span class="font-semibold">{formattedRating}</span>
                </span>
              {/if}
            </div>

            {#if showCookTooltip}
              <div
                class="absolute left-0 top-full z-20 mt-2 w-64 rounded-lg border border-gray-200 bg-white p-3 text-xs shadow-lg"
                role="tooltip"
                data-testid="recipe-cook-tooltip"
              >
                {#if cookTooltipLoading}
                  <p class="text-gray-500">Loading cooks...</p>
                {:else if cookTooltipError}
                  <p class="text-red-500">{cookTooltipError}</p>
                {:else if cookTooltipUsers.length === 0}
                  <p class="text-gray-500">No cooks yet.</p>
                {:else}
                  <div class="space-y-2">
                    {#each cookTooltipUsers as user}
                      <div class="flex items-center justify-between gap-2">
                        <div class="flex items-center gap-2 min-w-0">
                          {#if user.profile_picture_url}
                            <img
                              src={user.profile_picture_url}
                              alt={user.username}
                              class="h-7 w-7 rounded-full object-cover"
                            />
                          {:else}
                            <div class="h-7 w-7 rounded-full bg-gray-200 text-gray-500 flex items-center justify-center text-xs font-semibold">
                              {getInitial(user.username)}
                            </div>
                          {/if}
                          <span class="text-gray-700 truncate">{user.username || 'Unknown'}</span>
                        </div>
                        <div class="flex items-center gap-1">
                          <RatingStars value={user.rating} readonly size="sm" ariaLabel="Cook rating" />
                          <span class="text-gray-500 font-semibold">{user.rating.toFixed(1)}</span>
                        </div>
                      </div>
                    {/each}
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        {/if}
      {:else}
        <div class="inline-flex items-center gap-2 text-gray-500">
          <span class="inline-flex h-6 w-6 items-center justify-center rounded-full bg-gray-100 text-gray-500">
            <svg
              viewBox="0 0 20 20"
              class="h-3.5 w-3.5"
              fill="currentColor"
              aria-hidden="true"
            >
              <path d="M10 1.5l2.47 5.4 5.88.5-4.42 3.83 1.33 5.77L10 13.9 4.74 17l1.33-5.77L1.65 7.4l5.88-.5L10 1.5z" />
            </svg>
          </span>
          <span>No recipe stats yet</span>
        </div>
      {/if}
    </div>

    <div class="flex items-center gap-2">
      <RecipeSaveButton postId={postId} />
      <CookButton postId={postId} />
    </div>
  </div>
{/if}
