<script lang="ts">
  import type { PostPodcastSaveInfo } from '../../services/api';
  import { api } from '../../services/api';
  import { podcastSaveInfoByPostId, podcastStore } from '../../stores/podcastStore';

  export let postId: string;
  export let initialSaved = false;
  export let initialSaveCount = 0;

  let isSaving = false;
  let errorMessage: string | null = null;
  let propSignature = '';
  let fallbackInfo = buildSaveInfo(initialSaved, initialSaveCount);

  $: nextSignature = `${postId}:${initialSaved}:${initialSaveCount}`;
  $: if (nextSignature !== propSignature) {
    propSignature = nextSignature;
    fallbackInfo = buildSaveInfo(initialSaved, initialSaveCount);
  }

  $: storeInfo = $podcastSaveInfoByPostId[postId];
  $: effectiveInfo = storeInfo ?? fallbackInfo;
  $: isSaved = Boolean(effectiveInfo.viewerSaved);
  $: displayedSaveCount = clampCount(effectiveInfo.saveCount);

  function clampCount(value: number): number {
    return Math.max(0, Number.isFinite(value) ? value : 0);
  }

  function buildSaveInfo(viewerSaved: boolean, saveCount: number): PostPodcastSaveInfo {
    return {
      saveCount: clampCount(saveCount),
      users: [],
      viewerSaved,
    };
  }

  function normalizeSaveInfo(info: PostPodcastSaveInfo): PostPodcastSaveInfo {
    return {
      saveCount: clampCount(info.saveCount),
      users: info.users ?? [],
      viewerSaved: Boolean(info.viewerSaved),
    };
  }

  function shiftSaveCount(count: number, wasSaved: boolean, willBeSaved: boolean): number {
    if (!wasSaved && willBeSaved) {
      return clampCount(count + 1);
    }
    if (wasSaved && !willBeSaved) {
      return clampCount(count - 1);
    }
    return clampCount(count);
  }

  function applySaveInfo(info: PostPodcastSaveInfo) {
    const normalized = normalizeSaveInfo(info);
    fallbackInfo = normalized;
    podcastStore.setPostSaveInfo(postId, normalized);
  }

  async function toggleSave() {
    if (!postId || isSaving) {
      return;
    }

    isSaving = true;
    errorMessage = null;

    const previous = normalizeSaveInfo(effectiveInfo);
    const nextSaved = !previous.viewerSaved;
    const optimistic: PostPodcastSaveInfo = {
      ...previous,
      viewerSaved: nextSaved,
      saveCount: shiftSaveCount(previous.saveCount, previous.viewerSaved, nextSaved),
    };

    applySaveInfo(optimistic);

    try {
      if (nextSaved) {
        await api.savePodcast(postId);
      } else {
        await api.unsavePodcast(postId);
      }

      const persisted = await api.getPostPodcastSaveInfo(postId);
      applySaveInfo(persisted);
      if (!persisted.viewerSaved) {
        podcastStore.removeSavedPost(postId);
      }
    } catch (error) {
      applySaveInfo(previous);
      errorMessage = error instanceof Error ? error.message : 'Failed to update podcast save.';
    } finally {
      isSaving = false;
    }
  }
</script>

<div class="inline-flex flex-col gap-2" data-testid="podcast-save-button">
  <button
    type="button"
    class={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-semibold transition-colors ${
      isSaved
        ? 'border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100'
        : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50'
    }`}
    on:click={toggleSave}
    aria-label={isSaved ? 'Remove podcast from saved for later' : 'Save podcast for later'}
    disabled={isSaving}
  >
    {#if isSaved}
      <svg class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
        <path d="M16.707 6.707a1 1 0 0 0-1.414-1.414L8.5 12.086 5.707 9.293a1 1 0 0 0-1.414 1.414l3.5 3.5a1 1 0 0 0 1.414 0l7.5-7.5Z" />
      </svg>
      <span>Saved</span>
    {:else}
      <svg class="h-4 w-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" aria-hidden="true">
        <path d="M10 4v12M4 10h12" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" />
      </svg>
      <span>Save for later</span>
    {/if}

    {#if displayedSaveCount > 0}
      <span
        class={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${
          isSaved ? 'bg-emerald-100 text-emerald-700' : 'bg-gray-100 text-gray-700'
        }`}
        data-testid="podcast-save-count"
      >
        {displayedSaveCount}
      </span>
    {/if}

    {#if isSaving}
      <span
        class="h-3 w-3 animate-spin rounded-full border border-current border-t-transparent"
        aria-hidden="true"
        data-testid="podcast-save-spinner"
      ></span>
    {/if}
  </button>

  {#if errorMessage}
    <p
      class="text-xs font-medium text-red-600"
      role="alert"
      aria-live="assertive"
      data-testid="podcast-save-error"
    >
      {errorMessage}
    </p>
  {/if}
</div>
