<script lang="ts">
  import { onMount } from 'svelte';
  import { displayTimezone } from '../stores';
  import { formatInTimezone, getYearInTimezone } from '../lib/time';

  export let dateString: string | null | undefined;
  export let className = '';

  let showTooltip = false;
  let tooltipId = '';
  let lastPointerType: string | null = null;

  $: relativeLabel = formatRelative(dateString, $displayTimezone);
  $: exactLabel = formatExact(dateString, $displayTimezone);

  onMount(() => {
    tooltipId = `timestamp-tooltip-${Math.random().toString(36).slice(2, 10)}`;
  });

  function formatRelative(value: string | null | undefined, timezone?: string | null): string {
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

    const nowYear = getYearInTimezone(now, timezone);
    const dateYear = getYearInTimezone(date, timezone);
    const options: Intl.DateTimeFormatOptions = { month: 'short', day: 'numeric' };
    if (dateYear !== nowYear) {
      options.year = 'numeric';
    }
    return formatInTimezone(date, options, timezone);
  }

  function formatExact(value: string | null | undefined, timezone?: string | null): string {
    if (!value) return '';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return '';

    const datePart = formatInTimezone(
      date,
      {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
      },
      timezone
    );
    const timePart = formatInTimezone(
      date,
      {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
      },
      timezone
    );

    return `${datePart} ${timePart}`;
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      showTooltip = false;
    }
  }

  function handlePointerDown(event: PointerEvent) {
    lastPointerType = event.pointerType;
  }

  function handleClick() {
    if (lastPointerType === 'touch') {
      showTooltip = true;
      return;
    }
    showTooltip = !showTooltip;
  }
</script>

<span class="relative inline-flex items-center">
  <button
    type="button"
    class={`inline-flex items-center bg-transparent border-0 p-0 ${className}`}
    aria-describedby={tooltipId}
    aria-label={
      exactLabel
        ? `${relativeLabel}. Exact time ${exactLabel}`
        : `${relativeLabel}. Exact time unavailable`
    }
    on:mouseenter={() => (showTooltip = true)}
    on:mouseleave={() => (showTooltip = false)}
    on:focus={() => (showTooltip = true)}
    on:blur={() => (showTooltip = false)}
    on:pointerdown={handlePointerDown}
    on:click={handleClick}
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
