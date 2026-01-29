<script lang="ts">
  import { onMount } from 'svelte';

  export let createdAt: string | null | undefined;
  export let updatedAt: string | null | undefined;

  let showTooltip = false;
  let tooltipId = '';

  $: isEdited = isEditedAt(createdAt, updatedAt);
  $: tooltipLabel = updatedAt ? `Edited ${formatEditedAt(updatedAt)}` : '';

  onMount(() => {
    tooltipId = `edited-tooltip-${Math.random().toString(36).slice(2, 10)}`;
  });

  function isEditedAt(created: string | null | undefined, updated: string | null | undefined): boolean {
    if (!created || !updated) return false;
    const createdTime = new Date(created).getTime();
    const updatedTime = new Date(updated).getTime();
    if (Number.isNaN(createdTime) || Number.isNaN(updatedTime)) return false;
    return updatedTime > createdTime;
  }

  function formatEditedAt(dateString: string): string {
    const date = new Date(dateString);
    if (Number.isNaN(date.getTime())) return '';
    const datePart = date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
    const timePart = date.toLocaleTimeString('en-US', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    });
    return `${datePart} ${timePart}`;
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      showTooltip = false;
    }
  }
</script>

{#if isEdited}
  <span class="relative inline-flex items-center">
    <button
      type="button"
      class="text-gray-400 text-xs hover:text-gray-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 rounded px-1"
      aria-describedby={tooltipId}
      on:mouseenter={() => (showTooltip = true)}
      on:mouseleave={() => (showTooltip = false)}
      on:focus={() => (showTooltip = true)}
      on:blur={() => (showTooltip = false)}
      on:click={(event) => {
        event.stopPropagation();
        showTooltip = true;
      }}
      on:keydown={handleKeydown}
    >
      (edited)
    </button>
    {#if showTooltip}
      <span
        id={tooltipId}
        role="tooltip"
        class="absolute left-1/2 top-full mt-1 -translate-x-1/2 whitespace-nowrap rounded bg-gray-900 px-2 py-1 text-xs text-white shadow-lg z-20"
      >
        {tooltipLabel}
      </span>
    {/if}
  </span>
{/if}
