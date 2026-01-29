<script lang="ts">
  import { fatalError, clearFatalError } from '../observability/errorState';

  export let title = 'Something went wrong';
  export let message =
    'We hit an unexpected issue. Reload the page or try again in a moment.';

  const showDetails = import.meta.env.DEV;

  function handleReload() {
    if (typeof window !== 'undefined') {
      clearFatalError();
      window.location.reload();
    }
  }
</script>

{#if $fatalError}
  <div class="min-h-screen flex items-center justify-center bg-gray-50">
    <div class="max-w-lg w-full bg-white border border-gray-200 shadow-sm rounded-xl p-6 text-center">
      <div class="text-4xl mb-3">⚠️</div>
      <h1 class="text-xl font-semibold text-gray-900 mb-2">{title}</h1>
      <p class="text-gray-600 mb-4">{message}</p>
      {#if showDetails}
        <pre class="text-xs text-left bg-gray-100 rounded-md p-3 overflow-auto mb-4">
{$fatalError.message}
        </pre>
      {/if}
      <button
        type="button"
        class="inline-flex items-center justify-center rounded-md bg-indigo-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500"
        on:click={handleReload}
      >
        Reload page
      </button>
    </div>
  </div>
{:else}
  <slot />
{/if}
