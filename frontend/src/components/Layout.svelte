<script lang="ts">
  import { onMount } from 'svelte';
  import { uiStore } from '../stores';
  import Header from './Header.svelte';
  import Sidebar from './Sidebar.svelte';

  onMount(() => {
    const mediaQuery = window.matchMedia('(max-width: 1023px)');

    function handleResize(e: MediaQueryListEvent | MediaQueryList) {
      uiStore.setIsMobile(e.matches);
    }

    handleResize(mediaQuery);
    mediaQuery.addEventListener('change', handleResize);

    return () => {
      mediaQuery.removeEventListener('change', handleResize);
    };
  });
</script>

<div class="min-h-screen bg-gray-50">
  <Header />

  <div class="flex">
    <Sidebar />

    <main class="flex-1 lg:ml-0">
      <div class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
        <slot />
      </div>
    </main>
  </div>
</div>
