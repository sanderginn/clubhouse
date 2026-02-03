<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  type RatingSize = 'sm' | 'md' | 'lg';

  export let value = 0;
  export let readonly = false;
  export let size: RatingSize = 'md';
  export let ariaLabel = 'Rating';
  export let onChange: ((nextValue: number) => void) | null = null;

  const dispatch = createEventDispatcher<{ change: number }>();
  const stars = [1, 2, 3, 4, 5];

  let hoverValue: number | null = null;
  let keyboardValue: number | null = null;

  const sizeClasses: Record<RatingSize, string> = {
    sm: 'h-4 w-4',
    md: 'h-5 w-5',
    lg: 'h-6 w-6',
  };

  function roundToHalf(input: number) {
    return Math.round(input * 2) / 2;
  }

  $: displayValue = readonly ? roundToHalf(value) : value;
  $: activeValue = hoverValue ?? keyboardValue ?? displayValue;
  $: sizeClass = sizeClasses[size] ?? sizeClasses.md;
  $: ariaValueText = `${displayValue} out of 5`;
  $: labelText = readonly ? `${ariaLabel}: ${ariaValueText}` : ariaLabel;

  function getFillPercent(star: number) {
    const fill = Math.max(0, Math.min(1, activeValue - (star - 1)));
    return fill * 100;
  }

  function handleSelect(nextValue: number) {
    if (readonly) {
      return;
    }

    const finalValue = nextValue === value ? 0 : nextValue;
    value = finalValue;
    onChange?.(finalValue);
    dispatch('change', finalValue);
  }

  function handleStarEnter(star: number) {
    if (readonly) {
      return;
    }

    hoverValue = star;
  }

  function handleMouseMove(event: MouseEvent) {
    if (readonly) {
      return;
    }

    const target = event.target as HTMLElement | null;
    const starElement = target?.closest('[data-star]');
    const starValue = starElement ? Number(starElement.getAttribute('data-star')) : null;

    if (starValue && !Number.isNaN(starValue)) {
      handleStarEnter(starValue);
    }
  }

  function handleMouseLeave() {
    if (readonly) {
      return;
    }

    hoverValue = null;
  }

  function handleKeyDown(event: KeyboardEvent) {
    if (readonly) {
      return;
    }

    const current = keyboardValue ?? value;
    let next = current;

    switch (event.key) {
      case 'ArrowRight':
      case 'ArrowUp':
        next = Math.min(5, Math.max(0, current) + 1);
        keyboardValue = next;
        event.preventDefault();
        break;
      case 'ArrowLeft':
      case 'ArrowDown':
        next = Math.max(0, Math.max(0, current) - 1);
        keyboardValue = next;
        event.preventDefault();
        break;
      case 'Home':
        keyboardValue = 0;
        event.preventDefault();
        break;
      case 'End':
        keyboardValue = 5;
        event.preventDefault();
        break;
      case 'Enter':
      case ' ':
        handleSelect(Math.max(0, current));
        keyboardValue = null;
        event.preventDefault();
        break;
    }
  }
</script>

<!-- svelte-ignore a11y-no-noninteractive-tabindex -->
<div
  class="inline-flex items-center gap-1"
  role={readonly ? 'img' : 'slider'}
  aria-label={labelText}
  aria-valuemin={readonly ? undefined : 0}
  aria-valuemax={readonly ? undefined : 5}
  aria-valuenow={readonly ? undefined : value}
  aria-valuetext={readonly ? undefined : ariaValueText}
  tabindex={readonly ? undefined : 0}
  on:keydown={handleKeyDown}
  on:mousemove={handleMouseMove}
  on:mouseleave={handleMouseLeave}
  on:blur={() => (keyboardValue = null)}
  class:focus-visible={!readonly}
>
  {#each stars as star}
    <div
      class={`relative ${sizeClass} ${readonly ? '' : 'cursor-pointer'} text-gray-300`}
      data-testid={`rating-star-${star}`}
      data-star={star}
      on:click={() => handleSelect(star)}
      aria-hidden="true"
    >
      <svg viewBox="0 0 20 20" class={`absolute inset-0 ${sizeClass}`} fill="currentColor">
        <path
          d="M10 1.5l2.47 5.4 5.88.5-4.42 3.83 1.33 5.77L10 13.9 4.74 17l1.33-5.77L1.65 7.4l5.88-.5L10 1.5z"
        />
      </svg>
      <div
        class="absolute inset-0 overflow-hidden text-amber-400"
        style={`width: ${getFillPercent(star)}%`}
        data-testid={`rating-star-fill-${star}`}
      >
        <svg viewBox="0 0 20 20" class={`absolute inset-0 ${sizeClass}`} fill="currentColor">
          <path
            d="M10 1.5l2.47 5.4 5.88.5-4.42 3.83 1.33 5.77L10 13.9 4.74 17l1.33-5.77L1.65 7.4l5.88-.5L10 1.5z"
          />
        </svg>
      </div>
    </div>
  {/each}

  <span class="sr-only">{displayValue} out of 5 stars</span>
</div>

<style>
  .focus-visible:focus-visible {
    outline: 2px solid rgba(251, 191, 36, 0.9);
    outline-offset: 4px;
    border-radius: 9999px;
  }
</style>
