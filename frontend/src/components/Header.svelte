<script lang="ts">
  import { uiStore, currentUser, isAuthenticated } from '../stores';

  function toggleSidebar() {
    uiStore.toggleSidebar();
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
        </div>
      {:else}
        <a
          href="/login"
          class="text-sm font-medium text-gray-700 hover:text-primary"
        >
          Log in
        </a>
        <a
          href="/register"
          class="text-sm font-medium text-white bg-primary hover:bg-secondary px-4 py-2 rounded-lg transition-colors"
        >
          Register
        </a>
      {/if}
    </div>
  </div>
</header>
