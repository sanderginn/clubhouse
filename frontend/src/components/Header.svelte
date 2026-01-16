<script lang="ts">
  import { uiStore, currentUser, isAuthenticated, authStore } from '../stores';

  function toggleSidebar() {
    uiStore.toggleSidebar();
  }

  async function handleLogout() {
    await authStore.logout();
  }
</script>

<header class="bg-white border-b border-gray-200 sticky top-0 z-40">
  <div class="flex items-center justify-between h-16 px-4">
    <div class="flex items-center gap-4">
      <button
        on:click={toggleSidebar}
        class="p-2 rounded-lg text-gray-600 hover:bg-gray-100 lg:hidden"
        aria-label="Toggle sidebar"
      >
        <svg
          class="w-6 h-6"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 6h16M4 12h16M4 18h16"
          />
        </svg>
      </button>

      <a href="/" class="flex items-center gap-2">
        <span class="text-2xl">🏠</span>
        <span class="text-xl font-bold text-gray-900">Clubhouse</span>
      </a>
    </div>

    <div class="flex items-center gap-4">
      {#if $isAuthenticated && $currentUser}
        <div class="flex items-center gap-3">
          <span class="text-sm text-gray-700 hidden sm:block">
            {$currentUser.username}
          </span>
          {#if $currentUser.profilePictureUrl}
            <img
              src={$currentUser.profilePictureUrl}
              alt={$currentUser.username}
              class="w-8 h-8 rounded-full object-cover"
            />
          {:else}
            <div
              class="w-8 h-8 rounded-full bg-primary text-white flex items-center justify-center text-sm font-medium"
            >
              {$currentUser.username.charAt(0).toUpperCase()}
            </div>
          {/if}
          <button
            on:click={handleLogout}
            class="text-sm font-medium text-gray-600 hover:text-gray-900 ml-2"
            title="Logout"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
            </svg>
          </button>
        </div>
      {/if}
    </div>
  </div>
</header>
