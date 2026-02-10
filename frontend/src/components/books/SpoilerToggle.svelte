<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let checked = false;

  const dispatch = createEventDispatcher<{ change: boolean }>();

  function handleChange(event: Event) {
    const input = event.currentTarget as HTMLInputElement | null;
    const nextChecked = Boolean(input?.checked);
    checked = nextChecked;
    dispatch('change', nextChecked);
  }
</script>

<label class="inline-flex cursor-pointer select-none items-center gap-3" data-testid="spoiler-toggle">
  <input
    type="checkbox"
    checked={checked}
    on:change={handleChange}
    aria-label="Contains spoiler"
    class="h-4 w-4 rounded border-gray-300 text-slate-700 focus:ring-slate-500"
    data-testid="spoiler-toggle-input"
  />
  <span class="text-sm font-medium text-slate-700">Contains spoiler</span>

  {#if checked}
    <span
      class="inline-flex h-5 w-5 items-center justify-center text-slate-600"
      data-testid="spoiler-toggle-eye-slash"
      aria-hidden="true"
    >
      <svg viewBox="0 0 24 24" class="h-4 w-4 fill-none stroke-current stroke-2">
        <path
          d="M3 12s3.5-6 9-6c2.1 0 3.9.6 5.4 1.5M21 12s-3.5 6-9 6c-2.1 0-3.9-.6-5.4-1.5M9.9 9.9A3 3 0 0 1 12 9a3 3 0 0 1 3 3c0 .8-.3 1.5-.8 2.1M3 3l18 18"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
      </svg>
    </span>
  {/if}
</label>
