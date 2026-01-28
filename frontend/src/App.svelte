<script lang="ts">
  import { get } from 'svelte/store';
  import { onMount, onDestroy } from 'svelte';
  import './styles/globals.css';
  import { Layout, PostForm, SectionFeed, SearchBar, SearchResults, InstallPrompt } from './components';
  import UserProfile from './components/UserProfile.svelte';
  import { Login, Register, AdminPanel, PasswordReset } from './routes';
  import {
    authStore,
    isAuthenticated,
    activeSection,
    sections,
    searchQuery,
    websocketStore,
    sectionStore,
    activeView,
    isAdmin,
    pwaStore,
    activeProfileUserId,
    uiStore,
    threadRouteStore,
  } from './stores';
  import { parseProfileUserId } from './services/profileNavigation';
  import {
    buildFeedHref,
    isAdminPath,
    parseSectionId,
    parseThreadPostId,
    replacePath,
  } from './services/routeNavigation';
  import { parseResetRoute } from './services/resetLink';

  let unauthRoute: 'login' | 'register' | 'reset' = 'login';
  let resetToken: string | null = null;
  let sectionsLoadedForSession = false;
  let popstateHandler: (() => void) | null = null;
  let pendingSectionId: string | null = null;

  onMount(() => {
    authStore.checkSession();
    websocketStore.init();
    pwaStore.init();
    syncRouteFromLocation();

    if (typeof window !== 'undefined') {
      const handler = () => syncRouteFromLocation();
      window.addEventListener('popstate', handler);
      popstateHandler = () => window.removeEventListener('popstate', handler);
    }
  });

  onDestroy(() => {
    websocketStore.cleanup();
    popstateHandler?.();
  });

  function syncRouteFromLocation() {
    if (typeof window === 'undefined') return;
    const path = window.location.pathname;
    const { isReset, token } = parseResetRoute(window.location);
    if (isReset) {
      unauthRoute = 'reset';
      resetToken = token;
      pendingSectionId = null;
      return;
    }
    const profileUserId = parseProfileUserId(path);
    if (profileUserId) {
      uiStore.openProfile(profileUserId);
      threadRouteStore.clearTarget();
      pendingSectionId = null;
    } else {
      const threadPostId = parseThreadPostId(path);
      const sectionId = parseSectionId(path);
      if (threadPostId && sectionId) {
        threadRouteStore.setTarget(threadPostId, sectionId);
      } else {
        threadRouteStore.clearTarget();
      }
      if (sectionId) {
        const availableSections = get(sections);
        if (availableSections.length > 0) {
          const match = availableSections.find((section) => section.id === sectionId);
          const fallback = availableSections[0] ?? null;
          if (match) {
            sectionStore.setActiveSection(match);
          } else {
            sectionStore.setActiveSection(fallback);
            replacePath(buildFeedHref(fallback?.id ?? null));
          }
          pendingSectionId = null;
        } else {
          pendingSectionId = sectionId;
        }
        uiStore.setActiveView('feed');
      } else if (isAdminPath(path)) {
        pendingSectionId = null;
        if (get(isAdmin)) {
          uiStore.setActiveView('admin');
        } else {
          uiStore.setActiveView('feed');
          const fallbackSectionId =
            get(activeSection)?.id ?? get(sections)[0]?.id ?? null;
          replacePath(buildFeedHref(fallbackSectionId));
        }
      } else {
        pendingSectionId = null;
        uiStore.setActiveView('feed');
      }
    }
    resetToken = null;
    unauthRoute = 'login';
  }

  function handleNavigate(page: 'login' | 'register') {
    unauthRoute = page;
    resetToken = null;
    if (typeof window !== 'undefined') {
      window.history.replaceState(null, '', '/');
    }
  }

  $: if ($isAuthenticated && !sectionsLoadedForSession) {
    sectionsLoadedForSession = true;
    sectionStore.loadSections(pendingSectionId);
  }

  $: if (!$isAuthenticated && sectionsLoadedForSession) {
    sectionsLoadedForSession = false;
    sectionStore.setSections([]);
  }

  $: if (pendingSectionId && $sections.length > 0) {
    const match = $sections.find((section) => section.id === pendingSectionId) ?? null;
    const fallback = $sections[0] ?? null;
    if (!match && $activeSection?.id !== fallback?.id) {
      sectionStore.setActiveSection(fallback);
    }
    const hasThreadTarget =
      $threadRouteStore.postId && $threadRouteStore.sectionId === pendingSectionId;
    if (hasThreadTarget && match) {
      // Preserve thread deep-link URL once sections are loaded.
    } else {
      if (hasThreadTarget && !match) {
        threadRouteStore.clearTarget();
      }
      replacePath(buildFeedHref(match?.id ?? fallback?.id ?? null));
    }
    pendingSectionId = null;
  }
</script>

{#if $authStore.isLoading}
  <div class="min-h-screen flex items-center justify-center bg-gray-50">
    <div class="flex flex-col items-center">
      <svg
        class="animate-spin h-10 w-10 text-indigo-600 mb-4"
        xmlns="http://www.w3.org/2000/svg"
        fill="none"
        viewBox="0 0 24 24"
      >
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"
        ></circle>
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
        ></path>
      </svg>
      <p class="text-gray-600">Loading...</p>
    </div>
  </div>
{:else if !$isAuthenticated}
  {#if unauthRoute === 'reset'}
    <PasswordReset token={resetToken} onNavigate={handleNavigate} />
  {:else if unauthRoute === 'login'}
    <Login onNavigate={handleNavigate} />
  {:else}
    <Register onNavigate={handleNavigate} />
  {/if}
{:else}
  <Layout>
    <div class="space-y-6">
      {#if $activeView === 'admin' && $isAdmin}
        <AdminPanel />
      {:else if $activeView === 'profile'}
        {#if $activeProfileUserId}
          <UserProfile userId={$activeProfileUserId} />
        {:else}
          <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
            <h1 class="text-xl font-semibold text-gray-900 mb-2">User not found</h1>
            <p class="text-gray-600">We couldn’t load that profile. Try selecting a user again.</p>
          </div>
        {/if}
      {:else if $activeSection}
        <div class="flex items-center gap-3">
          <span class="text-3xl">{$activeSection.icon}</span>
          <h1 class="text-2xl font-bold text-gray-900">{$activeSection.name}</h1>
        </div>

        <SearchBar />

        <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
          <PostForm />
        </div>

        {#if $searchQuery.trim().length > 0}
          <SearchResults />
        {:else}
          <SectionFeed />
        {/if}
      {:else}
        <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
          <h1 class="text-2xl font-bold text-gray-900 mb-4">Welcome to Clubhouse</h1>
          <p class="text-gray-600">Select a section from the sidebar to get started.</p>
        </div>
      {/if}
    </div>
  </Layout>
{/if}

<InstallPrompt />
