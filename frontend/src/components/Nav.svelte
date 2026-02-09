<script lang="ts">
  import {
    sections,
    activeSection,
    sectionStore,
    uiStore,
    isAdmin,
    activeView,
    isAuthenticated,
    threadRouteStore,
    unreadRegistrationCount,
  } from '../stores';
  import {
    buildAdminHref,
    buildSectionHref,
    buildWatchlistHref,
    pushPath,
  } from '../services/routeNavigation';
  import type { Section } from '../stores/sectionStore';
  import NotificationSettings from './NotificationSettings.svelte';

  function handleSectionClick(section: Section) {
    sectionStore.setActiveSection(section);
    uiStore.setActiveView('feed');
    threadRouteStore.clearTarget();
    pushPath(buildSectionHref(section.slug));
  }

  function handleAdminClick() {
    uiStore.setActiveView('admin');
    threadRouteStore.clearTarget();
    pushPath(buildAdminHref());
  }

  function handleWatchlistClick() {
    uiStore.setActiveView('watchlist');
    threadRouteStore.clearTarget();
    pushPath(buildWatchlistHref());
  }
</script>

<nav class="flex flex-col h-full" aria-label="Main navigation">
  <div class="flex-1 overflow-y-auto py-4">
    <div class="px-3 mb-2">
      <h2 class="text-xs font-semibold text-gray-500 uppercase tracking-wider">Sections</h2>
    </div>
    <ul class="space-y-1 px-2">
      {#each $sections as section (section.id)}
        <li>
          <button
            on:click={() => handleSectionClick(section)}
            class="w-full flex items-center gap-3 px-3 py-2 text-sm font-medium rounded-lg transition-colors
              {$activeView === 'feed' && $activeSection?.id === section.id
              ? 'bg-primary text-white'
              : 'text-gray-700 hover:bg-gray-100'}"
            aria-current={$activeView === 'feed' && $activeSection?.id === section.id ? 'page' : undefined}
          >
            <span class="text-lg" aria-hidden="true">{section.icon}</span>
            <span>{section.name}</span>
          </button>
        </li>
      {/each}
    </ul>

    {#if $isAuthenticated}
      <div class="mt-4 border-t border-gray-200 px-2 pt-4">
        <button
          on:click={handleWatchlistClick}
          class="w-full flex items-center gap-3 px-3 py-2 text-sm font-medium rounded-lg transition-colors
            {$activeView === 'watchlist'
            ? 'bg-indigo-600 text-white'
            : 'text-gray-700 hover:bg-indigo-50'}"
          aria-current={$activeView === 'watchlist' ? 'page' : undefined}
        >
          <span class="text-lg" aria-hidden="true">🎬</span>
          <span>My Movies</span>
        </button>
      </div>
    {/if}
  </div>

  {#if $isAuthenticated}
    <div class="border-t border-gray-200 px-3 py-4">
      <h2 class="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">Settings</h2>
      <NotificationSettings />
    </div>
  {/if}

  {#if $isAdmin}
    <div class="border-t border-gray-200 px-3 py-4">
      <h2 class="text-xs font-semibold text-gray-500 uppercase tracking-wider">Admin</h2>
      <button
        on:click={handleAdminClick}
        class="mt-3 w-full flex items-center gap-3 px-3 py-2 text-sm font-medium rounded-lg transition-colors
          {$activeView === 'admin'
          ? 'bg-amber-500 text-white'
          : 'text-gray-700 hover:bg-amber-50'}"
        aria-current={$activeView === 'admin' ? 'page' : undefined}
      >
        <span class="text-lg" aria-hidden="true">🛡️</span>
        <span>Moderation</span>
        {#if $unreadRegistrationCount > 0}
          <span
            class="ml-auto rounded-full bg-amber-500 px-2 py-0.5 text-xs font-semibold text-white"
            aria-label="Pending registrations"
          >
            {$unreadRegistrationCount > 99 ? '99+' : $unreadRegistrationCount}
          </span>
        {/if}
      </button>
    </div>
  {/if}
</nav>
