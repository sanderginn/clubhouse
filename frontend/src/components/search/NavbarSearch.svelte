<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import { activeSection, searchStore } from '../../stores';
  import type { SearchScope } from '../../stores/searchStore';

  let query = '';
  let scope: SearchScope = 'section';
  let isExpanded = false;
  let inputEl: HTMLInputElement | null = null;
  let containerEl: HTMLDivElement | null = null;

  const unsubscribe = searchStore.subscribe((state) => {
    query = state.query;
    scope = state.scope;
  });

  onMount(() => {
    window.addEventListener('keydown', handleShortcut);
    window.addEventListener('click', handleOutsideClick);

    return () => {
      window.removeEventListener('keydown', handleShortcut);
      window.removeEventListener('click', handleOutsideClick);
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
</script>

<div class="relative" bind:this={containerEl}>
  <button
    class="lg:hidden p-2 rounded-lg text-gray-600 hover:bg-gray-100"
    type="button"
    aria-label="Open search"
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
    class={`absolute right-0 mt-2 w-[calc(100vw-2rem)] max-w-md bg-white border border-gray-200 shadow-lg rounded-xl p-3 space-y-3 ${
      isExpanded ? 'block' : 'hidden'
    } lg:static lg:block lg:mt-0 lg:w-96 lg:shadow-none lg:border-gray-200 lg:rounded-lg`}
  >
    <div class="relative">
      <label for="navbar-search-input" class="sr-only">Search</label>
      <input
        id="navbar-search-input"
        bind:this={inputEl}
        type="search"
        value={query}
        on:input={handleInput}
        placeholder="Search posts and comments..."
        class="w-full pl-10 pr-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-primary focus:border-transparent"
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

    <div class="flex flex-wrap items-center gap-2">
      <select
        value={scope}
        on:change={handleScopeChange}
        class="w-full sm:w-auto min-w-[10rem] px-3 py-2 pr-8 border border-gray-300 rounded-lg text-xs bg-white focus:ring-2 focus:ring-primary focus:border-transparent"
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
          class="px-3 py-2 text-xs text-gray-600 hover:text-gray-900"
        >
          Clear
        </button>
      {/if}

      <button
        type="submit"
        class="px-3 py-2 text-xs bg-primary text-white font-medium rounded-lg hover:bg-secondary transition-colors"
      >
        Search
      </button>
    </div>
  </form>
</div>
