<script lang="ts">
  import { uiStore } from '../stores';
  import Nav from './Nav.svelte';

  let sidebarOpen: boolean;
  let isMobile: boolean;

  uiStore.subscribe(($ui) => {
    sidebarOpen = $ui.sidebarOpen;
    isMobile = $ui.isMobile;
  });

  function closeSidebar() {
    if (isMobile) {
      uiStore.setSidebarOpen(false);
    }
  }
</script>

<!-- Mobile overlay -->
{#if isMobile && sidebarOpen}
  <button
    class="fixed inset-0 bg-black/50 z-40 lg:hidden"
    on:click={closeSidebar}
    aria-label="Close sidebar"
  ></button>
{/if}

<!-- Sidebar -->
<aside
  class="fixed top-16 bottom-0 left-0 z-50 lg:z-30 w-64 bg-white border-r border-gray-200 transform transition-transform duration-200 ease-in-out
    {sidebarOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}"
  aria-label="Sidebar"
>
  <div class="h-full flex flex-col">
    <!-- Mobile header in sidebar -->
    <div class="lg:hidden flex items-center justify-between h-16 px-4 border-b border-gray-200">
      <span class="text-lg font-bold text-gray-900">Sections</span>
      <button
        on:click={closeSidebar}
        class="p-2 rounded-lg text-gray-600 hover:bg-gray-100"
        aria-label="Close sidebar"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M6 18L18 6M6 6l12 12"
          />
        </svg>
      </button>
    </div>

    <Nav />
  </div>
</aside>
