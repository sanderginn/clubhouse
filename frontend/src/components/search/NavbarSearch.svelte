<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import { activeSection, searchStore } from '../../stores';
  import type { SearchScope } from '../../stores/searchStore';

  let query = '';
  let scope: SearchScope = 'section';
  let isExpanded = false;
  let isDesktop = false;
  let inputEl: HTMLInputElement | null = null;
  let containerEl: HTMLDivElement | null = null;

  const unsubscribe = searchStore.subscribe((state) => {
    query = state.query;
    scope = state.scope;
  });

  onMount(() => {
    const cleanup: Array<() => void> = [];
    if (typeof window !== 'undefined') {
      const mediaQuery = window.matchMedia('(min-width: 1024px)');
      const handleMediaChange = () => {
        isDesktop = mediaQuery.matches;
      };
      handleMediaChange();
      mediaQuery.addEventListener('change', handleMediaChange);
      cleanup.push(() => {
        mediaQuery.removeEventListener('change', handleMediaChange);
      });
    }

    window.addEventListener('keydown', handleShortcut);
    window.addEventListener('click', handleOutsideClick);
    cleanup.push(() => {
      window.removeEventListener('keydown', handleShortcut);
      window.removeEventListener('click', handleOutsideClick);
    });

    return () => {
      cleanup.forEach((fn) => fn());
    };
  });

  onDestroy(() => {
    unsubscribe();
  });

  async function focusSearch() {
    isExpanded = true;
    await tick();
    inputEl?.focus();
    inputEl?.select();
  }

  function handleShortcut(event: KeyboardEvent) {
    if (!(event.metaKey || event.ctrlKey) || event.key.toLowerCase() !== 'k') {
      return;
    }

    const target = event.target as HTMLElement | null;
    if (
      target &&
      (target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.isContentEditable)
    ) {
      return;
    }

    event.preventDefault();
    focusSearch();
  }

  function handleOutsideClick(event: MouseEvent) {
    if (!containerEl) return;
    if (containerEl.contains(event.target as Node)) return;
    isExpanded = false;
  }

  function handleSubmit() {
    searchStore.search();
  }

  function handleInput(event: Event) {
    const value = (event.target as HTMLInputElement).value;
    searchStore.setQuery(value);
  }

  function handleScopeChange(event: Event) {
    const value = (event.target as HTMLSelectElement).value as SearchScope;
    searchStore.setScope(value);
  }

  function handleClear() {
    searchStore.clear();
  }

  function toggleExpanded(event: MouseEvent) {
    event.stopPropagation();
    if (isExpanded) {
      isExpanded = false;
      return;
    }
    focusSearch();
  }

  $: expandedClasses = isExpanded
    ? 'opacity-100 translate-y-0 scale-100 pointer-events-auto'
    : 'opacity-0 -translate-y-2 scale-95 pointer-events-none';
  $: isPanelActive = isExpanded || isDesktop;
</script>

<div class="relative" bind:this={containerEl}>
  <button
    class="lg:hidden p-2 rounded-lg text-gray-600 hover:bg-gray-100"
    type="button"
    aria-label="Open search"
    aria-expanded={isExpanded}
    on:click={toggleExpanded}
  >
    <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor">
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="2"
        d="M21 21l-4.35-4.35M11 19a8 8 0 100-16 8 8 0 000 16z"
      />
    </svg>
  </button>

  <form
    on:submit|preventDefault={handleSubmit}
    class={`absolute left-1/2 -translate-x-1/2 mt-2 w-[calc(100vw-2rem)] max-w-md bg-white border border-gray-200 shadow-lg rounded-xl p-3 space-y-3 transform transition-all duration-200 ease-out origin-top-right ${expandedClasses} lg:static lg:mt-0 lg:mx-auto lg:w-[28rem] lg:shadow-none lg:rounded-lg lg:opacity-100 lg:translate-y-0 lg:scale-100 lg:pointer-events-auto lg:flex lg:items-center lg:gap-3 lg:space-y-0 lg:p-2`}
    aria-hidden={!isPanelActive}
    inert={!isPanelActive}
  >
    <div class="relative lg:flex-1">
      <label for="navbar-search-input" class="sr-only">Search</label>
      <input
        id="navbar-search-input"
        bind:this={inputEl}
        type="search"
        value={query}
        on:input={handleInput}
        placeholder="Search posts and comments..."
        disabled={!isPanelActive}
        tabindex={isPanelActive ? 0 : -1}
        class="w-full pl-10 pr-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-primary focus:border-transparent lg:text-xs lg:py-1.5"
      />
      <svg
        class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M21 21l-4.35-4.35M11 19a8 8 0 100-16 8 8 0 000 16z"
        />
      </svg>
    </div>

    <div class="flex flex-wrap items-center gap-2 lg:flex-nowrap">
      <select
        value={scope}
        on:change={handleScopeChange}
        disabled={!isPanelActive}
        tabindex={isPanelActive ? 0 : -1}
        class="w-full sm:w-auto min-w-[10rem] px-3 py-2 pr-8 border border-gray-300 rounded-lg text-xs bg-white focus:ring-2 focus:ring-primary focus:border-transparent lg:py-1.5"
      >
        <option value="section">
          {#if $activeSection}
            In {$activeSection.name}
          {:else}
            In section
          {/if}
        </option>
        <option value="global">Everywhere</option>
      </select>

      {#if query.trim().length > 0}
        <button
          type="button"
          on:click={handleClear}
          disabled={!isPanelActive}
          tabindex={isPanelActive ? 0 : -1}
          class="px-3 py-2 text-xs text-gray-600 hover:text-gray-900 lg:py-1.5"
        >
          Clear
        </button>
      {/if}

      <button
        type="submit"
        disabled={!isPanelActive}
        tabindex={isPanelActive ? 0 : -1}
        class="px-3 py-2 text-xs bg-primary text-white font-medium rounded-lg hover:bg-secondary transition-colors lg:py-1.5"
      >
        Search
      </button>
    </div>
  </form>
</div>
