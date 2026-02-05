<script lang="ts">
  import { onDestroy } from 'svelte';
  import { activeSection, searchStore } from '../../stores';
  import type { SearchScope } from '../../stores/searchStore';

  let query = '';
  let scope: SearchScope = 'section';
  const unsubscribe = searchStore.subscribe((state) => {
    query = state.query;
    scope = state.scope;
  });

  onDestroy(() => {
    unsubscribe();
  });

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
</script>

<form on:submit|preventDefault={handleSubmit} class="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
  <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
    <div class="flex-1">
      <label for="search-input" class="sr-only">Search</label>
      <input
        id="search-input"
        name="search-input"
        type="search"
        value={query}
        on:input={handleInput}
        placeholder="Search posts and comments..."
        class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary focus:border-transparent"
      />
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <select
        name="search-scope"
        value={scope}
        on:change={handleScopeChange}
        class="w-full sm:w-auto min-w-[10rem] px-3 py-2 pr-8 border border-gray-300 rounded-lg text-sm bg-white focus:ring-2 focus:ring-primary focus:border-transparent"
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
          class="px-3 py-2 text-sm text-gray-600 hover:text-gray-900"
        >
          Clear
        </button>
      {/if}

      <button
        type="submit"
        class="px-4 py-2 bg-primary text-white font-medium rounded-lg hover:bg-secondary transition-colors"
      >
        Search
      </button>
    </div>
  </div>
</form>
