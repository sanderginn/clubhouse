<script lang="ts">
  import type { Highlight } from '../../stores/postStore';

  const maxHighlights = 20;

  export let highlights: Highlight[] = [];
  export let disabled = false;

  let timestampInput = '';
  let labelInput = '';
  let error: string | null = null;

  const formatTimestamp = (seconds: number) => {
    const safeSeconds = Math.max(0, Math.floor(seconds));
    const minutes = Math.floor(safeSeconds / 60);
    const remainder = safeSeconds % 60;
    return `${minutes.toString().padStart(2, '0')}:${remainder.toString().padStart(2, '0')}`;
  };

  const parseTimestamp = (value: string) => {
    const match = value.trim().match(/^(\d{1,3}):([0-5]\d)$/);
    if (!match) return null;
    const minutes = Number(match[1]);
    const seconds = Number(match[2]);
    return minutes * 60 + seconds;
  };

  const isAtMax = () => highlights.length >= maxHighlights;

  const addHighlight = () => {
    if (disabled) return;
    error = null;

    if (isAtMax()) {
      error = `Maximum of ${maxHighlights} highlights reached.`;
      return;
    }

    const parsedSeconds = parseTimestamp(timestampInput);
    if (parsedSeconds === null) {
      error = 'Enter a timestamp in mm:ss format.';
      return;
    }

    const label = labelInput.trim();
    if (label.length > 100) {
      error = 'Label must be 100 characters or fewer.';
      return;
    }

    highlights = [...highlights, ...(label ? [{ timestamp: parsedSeconds, label }] : [{ timestamp: parsedSeconds }])];
    timestampInput = '';
    labelInput = '';
  };

  const removeHighlight = (index: number) => {
    highlights = highlights.filter((_, idx) => idx !== index);
    error = null;
  };
</script>

<div class="space-y-3">
  <div class="grid gap-3 md:grid-cols-2">
    <div class="space-y-1">
      <label class="text-xs font-medium text-gray-600" for="highlight-timestamp">Timestamp (mm:ss)</label>
      <input
        id="highlight-timestamp"
        type="text"
        inputmode="numeric"
        placeholder="03:15"
        class="w-full rounded-md border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none"
        bind:value={timestampInput}
        disabled={disabled}
        aria-invalid={error?.includes('timestamp')}
      />
    </div>

    <div class="space-y-1">
      <label class="text-xs font-medium text-gray-600" for="highlight-label">Label (optional)</label>
      <input
        id="highlight-label"
        type="text"
        maxlength="100"
        placeholder="Intro, drop, chorus"
        class="w-full rounded-md border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none"
        bind:value={labelInput}
        disabled={disabled}
      />
    </div>
  </div>

  <div class="flex flex-wrap items-center gap-3">
    <button
      type="button"
      class="rounded-md bg-gray-900 px-3 py-1.5 text-sm font-medium text-white transition disabled:bg-gray-300"
      on:click={addHighlight}
      disabled={disabled || isAtMax()}
    >
      Add highlight
    </button>
    {#if isAtMax()}
      <p class="text-xs text-amber-600">Maximum of {maxHighlights} highlights reached.</p>
    {/if}
    {#if error && !isAtMax()}
      <p class="text-xs text-red-500">{error}</p>
    {/if}
  </div>

  {#if highlights.length > 0}
    <ul class="space-y-2">
      {#each highlights as highlight, index}
        <li class="flex items-center justify-between rounded-md border border-gray-200 px-3 py-2 text-sm">
          <div class="flex items-center gap-2">
            <span class="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700">
              {formatTimestamp(highlight.timestamp)}
            </span>
            {#if highlight.label}
              <span class="text-gray-700">{highlight.label}</span>
            {/if}
          </div>
          <button
            type="button"
            class="text-xs text-gray-400 hover:text-gray-600"
            on:click={() => removeHighlight(index)}
            disabled={disabled}
            aria-label={`Remove highlight ${formatTimestamp(highlight.timestamp)}`}
          >
            Remove
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>
