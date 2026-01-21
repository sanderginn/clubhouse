<script lang="ts">
  import { pwaStore, isPushSubscribed } from '../stores/pwaStore';
  import { derived, get } from 'svelte/store';

  // Derived stores for push notification state
  const isPushSupported = derived(pwaStore, ($pwa) => $pwa.isPushSupported);
  const pushPermission = derived(pwaStore, ($pwa) => $pwa.pushPermission);
  const isServiceWorkerReady = derived(pwaStore, ($pwa) => $pwa.isServiceWorkerReady);

  let isLoading = false;
  let error = '';

  async function handleToggle() {
    isLoading = true;
    error = '';

    try {
      if (get(isPushSubscribed)) {
        const success = await pwaStore.unsubscribeFromPush();
        if (!success) {
          error = 'Failed to disable notifications. Please try again.';
        }
      } else {
        const success = await pwaStore.subscribeToPush();
        if (!success) {
          // Check if permission was denied
          const permission = get(pushPermission);
          if (permission === 'denied') {
            error = 'Permission denied. Please enable notifications in your browser settings.';
          } else {
            error = 'Failed to enable notifications. Please try again.';
          }
        }
      }
    } catch (e) {
      error = 'An unexpected error occurred.';
    } finally {
      isLoading = false;
    }
  }
</script>

<div class="notification-settings">
  {#if !$isPushSupported}
    <div class="flex items-center gap-3 px-3 py-2 text-sm text-gray-500">
      <span class="text-lg" aria-hidden="true">🔕</span>
      <span>Notifications not supported</span>
    </div>
  {:else if !$isServiceWorkerReady}
    <div class="flex items-center gap-3 px-3 py-2 text-sm text-gray-500">
      <span class="text-lg" aria-hidden="true">⏳</span>
      <span>Loading...</span>
    </div>
  {:else}
    <button
      on:click={handleToggle}
      disabled={isLoading || $pushPermission === 'denied'}
      class="w-full flex items-center justify-between gap-3 px-3 py-2 text-sm font-medium rounded-lg transition-colors
        {$pushPermission === 'denied'
        ? 'text-gray-400 cursor-not-allowed'
        : 'text-gray-700 hover:bg-gray-100'}"
      title={$pushPermission === 'denied'
        ? 'Enable notifications in browser settings'
        : $isPushSubscribed
          ? 'Disable push notifications'
          : 'Enable push notifications'}
    >
      <div class="flex items-center gap-3">
        <span class="text-lg" aria-hidden="true">
          {#if $pushPermission === 'denied'}
            🚫
          {:else if $isPushSubscribed}
            🔔
          {:else}
            🔕
          {/if}
        </span>
        <span>
          {#if $pushPermission === 'denied'}
            Notifications blocked
          {:else if isLoading}
            {$isPushSubscribed ? 'Disabling...' : 'Enabling...'}
          {:else if $isPushSubscribed}
            Notifications enabled
          {:else}
            Enable notifications
          {/if}
        </span>
      </div>

      {#if $pushPermission !== 'denied'}
        <div
          class="relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none
            {$isPushSubscribed ? 'bg-primary' : 'bg-gray-200'}"
          role="switch"
          aria-checked={$isPushSubscribed}
        >
          <span
            class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out
              {$isPushSubscribed ? 'translate-x-4' : 'translate-x-0'}"
          />
        </div>
      {/if}
    </button>

    {#if error}
      <p class="px-3 mt-1 text-xs text-red-600">{error}</p>
    {/if}

    {#if $pushPermission === 'denied'}
      <p class="px-3 mt-1 text-xs text-gray-500">
        To enable, allow notifications in your browser settings.
      </p>
    {/if}
  {/if}
</div>
