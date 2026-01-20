<script lang="ts">
  import { onMount } from 'svelte';
  import { pwaStore, isInstallable, isInstalled, updateAvailable } from '../stores/pwaStore';

  let showPrompt = false;
  let dismissed = false;

  // Show install prompt after a delay if app is installable and not already installed
  $: if ($isInstallable && !$isInstalled && !dismissed) {
    setTimeout(() => {
      showPrompt = true;
    }, 5000);
  }

  async function handleInstall() {
    const installed = await pwaStore.promptInstall();
    if (installed) {
      showPrompt = false;
    }
  }

  function handleDismiss() {
    showPrompt = false;
    dismissed = true;
    // Remember dismissal for this session
    sessionStorage.setItem('pwa-prompt-dismissed', 'true');
  }

  function handleUpdate() {
    pwaStore.applyUpdate();
  }

  function handleDismissUpdate() {
    pwaStore.dismissUpdate();
  }

  onMount(() => {
    // Check if user already dismissed the prompt this session
    dismissed = sessionStorage.getItem('pwa-prompt-dismissed') === 'true';
  });
</script>

{#if showPrompt && !$isInstalled}
  <div
    class="fixed bottom-4 left-4 right-4 md:left-auto md:right-4 md:w-96 bg-white rounded-lg shadow-lg border border-gray-200 p-4 z-50 animate-slide-up"
  >
    <div class="flex items-start gap-3">
      <div class="flex-shrink-0">
        <div class="w-10 h-10 bg-indigo-100 rounded-lg flex items-center justify-center">
          <svg
            class="w-6 h-6 text-indigo-600"
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
            />
          </svg>
        </div>
      </div>
      <div class="flex-1 min-w-0">
        <h3 class="text-sm font-semibold text-gray-900">Install Clubhouse</h3>
        <p class="text-sm text-gray-600 mt-1">
          Add Clubhouse to your home screen for quick access and offline support.
        </p>
        <div class="flex gap-2 mt-3">
          <button
            on:click={handleInstall}
            class="px-3 py-1.5 bg-indigo-600 text-white text-sm font-medium rounded-md hover:bg-indigo-700 transition-colors"
          >
            Install
          </button>
          <button
            on:click={handleDismiss}
            class="px-3 py-1.5 text-gray-600 text-sm font-medium hover:text-gray-900 transition-colors"
          >
            Not now
          </button>
        </div>
      </div>
      <button
        on:click={handleDismiss}
        class="flex-shrink-0 text-gray-400 hover:text-gray-500"
        aria-label="Dismiss"
      >
        <svg class="w-5 h-5" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
          <path
            fill-rule="evenodd"
            d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
            clip-rule="evenodd"
          />
        </svg>
      </button>
    </div>
  </div>
{/if}

{#if $updateAvailable}
  <div
    class="fixed top-4 left-4 right-4 md:left-auto md:right-4 md:w-96 bg-indigo-600 text-white rounded-lg shadow-lg p-4 z-50 animate-slide-down"
  >
    <div class="flex items-start gap-3">
      <div class="flex-shrink-0">
        <svg class="w-6 h-6" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
          />
        </svg>
      </div>
      <div class="flex-1 min-w-0">
        <h3 class="text-sm font-semibold">Update available</h3>
        <p class="text-sm text-indigo-100 mt-1">
          A new version of Clubhouse is available.
        </p>
        <div class="flex gap-2 mt-3">
          <button
            on:click={handleUpdate}
            class="px-3 py-1.5 bg-white text-indigo-600 text-sm font-medium rounded-md hover:bg-indigo-50 transition-colors"
          >
            Update now
          </button>
          <button
            on:click={handleDismissUpdate}
            class="px-3 py-1.5 text-indigo-100 text-sm font-medium hover:text-white transition-colors"
          >
            Later
          </button>
        </div>
      </div>
      <button
        on:click={handleDismissUpdate}
        class="flex-shrink-0 text-indigo-200 hover:text-white"
        aria-label="Dismiss"
      >
        <svg class="w-5 h-5" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
          <path
            fill-rule="evenodd"
            d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
            clip-rule="evenodd"
          />
        </svg>
      </button>
    </div>
  </div>
{/if}

<style>
  @keyframes slide-up {
    from {
      transform: translateY(100%);
      opacity: 0;
    }
    to {
      transform: translateY(0);
      opacity: 1;
    }
  }

  @keyframes slide-down {
    from {
      transform: translateY(-100%);
      opacity: 0;
    }
    to {
      transform: translateY(0);
      opacity: 1;
    }
  }

  .animate-slide-up {
    animation: slide-up 0.3s ease-out;
  }

  .animate-slide-down {
    animation: slide-down 0.3s ease-out;
  }
</style>
