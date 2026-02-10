<script lang="ts">
  export let isSpoiler = false;

  let revealed = false;
  let lastSpoilerState = isSpoiler;

  $: if (isSpoiler !== lastSpoilerState) {
    lastSpoilerState = isSpoiler;
    revealed = false;
  }

  $: isHidden = isSpoiler && !revealed;

  function revealContent() {
    if (!isHidden) {
      return;
    }

    revealed = true;
  }
</script>

<div class="relative overflow-hidden rounded-lg" data-testid="spoiler-wrapper">
  <div
    class="transition duration-300 ease-out"
    class:pointer-events-none={isHidden}
    style={`filter: ${isHidden ? 'blur(8px)' : 'none'}; opacity: ${isHidden ? 0.75 : 1};`}
    data-testid="spoiler-content"
  >
    <slot />
  </div>

  {#if isHidden}
    <button
      type="button"
      class="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-white/70 text-center"
      on:click={revealContent}
      data-testid="spoiler-overlay"
    >
      <span class="inline-flex h-8 w-8 items-center justify-center rounded-full bg-white/90 text-slate-700">
        <svg viewBox="0 0 24 24" class="h-5 w-5 fill-none stroke-current stroke-2" aria-hidden="true">
          <path
            d="M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6-10-6-10-6z"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
          <circle cx="12" cy="12" r="3" />
        </svg>
      </span>
      <span class="text-sm font-semibold text-slate-900">Spoiler &mdash; Click to reveal</span>
    </button>
  {/if}

  {#if isSpoiler && !isHidden}
    <span
      class="absolute right-2 top-2 rounded-full bg-slate-800/85 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-white"
      data-testid="spoiler-badge"
    >
      Spoiler
    </span>
  {/if}
</div>
