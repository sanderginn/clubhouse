<script lang="ts">
  import { onDestroy } from 'svelte';
  import { uiStore } from '../stores';
  import Header from './Header.svelte';
  import Sidebar from './Sidebar.svelte';

  let mediaQuery: MediaQueryList | null = null;
  let cleanupListener: (() => void) | null = null;

  if (typeof window !== 'undefined') {
    mediaQuery = window.matchMedia('(max-width: 1023px)');
    uiStore.setIsMobile(mediaQuery.matches);

    const handleResize = (event?: MediaQueryListEvent) => {
      uiStore.setIsMobile(event?.matches ?? mediaQuery.matches);
    };

    handleResize();
    if ('addEventListener' in mediaQuery) {
      mediaQuery.addEventListener('change', handleResize);
      cleanupListener = () => mediaQuery?.removeEventListener('change', handleResize);
    } else {
      mediaQuery.addListener(handleResize);
      cleanupListener = () => mediaQuery?.removeListener(handleResize);
    }
  }

  onDestroy(() => {
    cleanupListener?.();
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
