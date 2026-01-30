<script lang="ts">
  import { uiStore, currentUser, isAuthenticated, authStore } from '../stores';
  import { buildProfileHref, handleProfileNavigation } from '../services/profileNavigation';
  import { buildSettingsHref } from '../services/routeNavigation';
  import { handleSettingsNavigation } from '../services/settingsNavigation';
  import NavbarSearch from './search/NavbarSearch.svelte';

  let menuOpen = false;

  function toggleSidebar() {
    uiStore.toggleSidebar();
  }

  function toggleMenu() {
    menuOpen = !menuOpen;
  }

  function closeMenu() {
    menuOpen = false;
  }

  async function handleLogout() {
    menuOpen = false;
    await authStore.logout();
  }
</script>

<svelte:window on:click={closeMenu} />

<header class="fixed top-0 left-0 right-0 z-40 bg-white border-b border-gray-200">
  <div class="flex items-center justify-between h-16 px-4">
    <div class="flex items-center gap-4">
      <button
        on:click={toggleSidebar}
        class="p-2 rounded-lg text-gray-600 hover:bg-gray-100 lg:hidden"
        aria-label="Toggle sidebar"
      >
        <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
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
      <NavbarSearch />
      {#if $isAuthenticated && $currentUser}
        <div class="relative">
          <button
            class="flex items-center gap-2 text-sm text-gray-700 hover:text-gray-900"
            on:click|stopPropagation={toggleMenu}
            aria-haspopup="true"
            aria-expanded={menuOpen}
            aria-label={`Open user menu for ${$currentUser.username}`}
            type="button"
          >
            <span class="hidden sm:block font-medium">{$currentUser.username}</span>
            <span class="sr-only">{$currentUser.username}</span>
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
            <svg class="w-4 h-4 text-gray-400" viewBox="0 0 20 20" fill="currentColor">
              <path
                fill-rule="evenodd"
                d="M5.23 7.21a.75.75 0 011.06.02L10 10.94l3.71-3.71a.75.75 0 011.08 1.04l-4.25 4.25a.75.75 0 01-1.06 0L5.21 8.27a.75.75 0 01.02-1.06z"
                clip-rule="evenodd"
              />
            </svg>
          </button>

          {#if menuOpen}
            <div
              class="absolute right-0 mt-2 w-48 rounded-lg border border-gray-200 bg-white shadow-lg py-2 z-50"
              role="menu"
            >
              <a
                href={buildProfileHref($currentUser.id)}
                class="block px-4 py-2 text-sm text-gray-700 hover:bg-gray-100"
                on:click={(event) => {
                  closeMenu();
                  handleProfileNavigation(event, $currentUser.id);
                }}
                role="menuitem"
              >
                Profile
              </a>
              <a
                href={buildSettingsHref()}
                class="block px-4 py-2 text-sm text-gray-700 hover:bg-gray-100"
                on:click={(event) => {
                  closeMenu();
                  handleSettingsNavigation(event);
                }}
                role="menuitem"
              >
                Settings
              </a>
              <div class="my-2 border-t border-gray-100"></div>
              <button
                on:click={handleLogout}
                class="w-full text-left px-4 py-2 text-sm text-gray-700 hover:bg-gray-100"
                role="menuitem"
                type="button"
              >
                Log out
              </button>
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </div>
</header>
