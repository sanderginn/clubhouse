<script lang="ts">
  import { onMount } from 'svelte';

  export let dateString: string | null | undefined;
  export let className = '';

  let showTooltip = false;
  let tooltipId = '';

  $: relativeLabel = formatRelative(dateString);
  $: exactLabel = formatExact(dateString);

  onMount(() => {
    tooltipId = `timestamp-tooltip-${Math.random().toString(36).slice(2, 10)}`;
  });

  function formatRelative(value?: string | null): string {
    if (!value) return 'Unknown date';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return 'Unknown date';

    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    if (diffMins < 1) return 'just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;

    return date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: date.getFullYear() !== now.getFullYear() ? 'numeric' : undefined,
    });
  }

  function formatExact(value?: string | null): string {
    if (!value) return '';
    const date = new Date(value);
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

<span class="relative inline-flex items-center">
  <button
    type="button"
    class={`inline-flex items-center bg-transparent border-0 p-0 ${className}`}
    aria-describedby={tooltipId}
    aria-label={exactLabel ? `Exact time ${exactLabel}` : 'Exact time unavailable'}
    on:mouseenter={() => (showTooltip = true)}
    on:mouseleave={() => (showTooltip = false)}
    on:focus={() => (showTooltip = true)}
    on:blur={() => (showTooltip = false)}
    on:click={() => (showTooltip = !showTooltip)}
    on:keydown={handleKeydown}
  >
    <time datetime={dateString ?? ''}>{relativeLabel}</time>
  </button>
  {#if showTooltip && exactLabel}
    <span
      id={tooltipId}
      role="tooltip"
      class="absolute left-1/2 top-full mt-1 -translate-x-1/2 whitespace-nowrap rounded bg-gray-900 px-2 py-1 text-xs text-white shadow-lg z-20"
    >
      {exactLabel}
    </span>
  {/if}
</span>
