<script lang="ts">
  import { get } from 'svelte/store';
  import { onMount, onDestroy } from 'svelte';
  import './styles/globals.css';
  import { Layout, PostForm, SectionFeed, SearchResults, InstallPrompt } from './components';
  import MusicLinksContainer from './components/MusicLinksContainer.svelte';
  import ThreadView from './components/ThreadView.svelte';
  import UserProfile from './components/UserProfile.svelte';
  import { Login, Register, AdminPanel, PasswordReset, Settings } from './routes';
  import {
    authStore,
    isAuthenticated,
    activeSection,
    sections,
    lastSearchQuery,
    searchError,
    isSearching,
    searchStore,
    websocketStore,
    sectionStore,
    activeView,
    isAdmin,
    pwaStore,
    activeProfileUserId,
    uiStore,
    threadRouteStore,
    configStore,
    initNotifications,
    cleanupNotifications,
  } from './stores';
  import { parseProfileUserId } from './services/profileNavigation';
  import {
    buildFeedHref,
    buildThreadHref,
    getHistoryState,
    isAdminPath,
    isSettingsPath,
    parseStandaloneThreadPostId,
    parseSectionSlug,
    parseThreadCommentId,
    parseThreadPostId,
    replacePath,
  } from './services/routeNavigation';
  import { parseResetRoute } from './services/resetLink';
  import { findSectionByIdentifier, getSectionSlug } from './services/sectionSlug';
  import ErrorBoundary from './lib/components/ErrorBoundary.svelte';

  let unauthRoute: 'login' | 'register' | 'reset' = 'login';
  let resetToken: string | null = null;
  let sectionsLoadedForSession = false;
  let popstateHandler: (() => void) | null = null;
  let pendingSectionIdentifier: string | null = null;
  let pendingThreadPostId: string | null = null;
  let pendingAdminPath = false;
  let sectionNotFound: string | null = null;
  let highlightCommentId: string | null = null;

  onMount(() => {
    authStore.checkSession();
    websocketStore.init();
    initNotifications();
    pwaStore.init();
    configStore.load();
    syncRouteFromLocation();

    if (typeof window !== 'undefined') {
      const handler = () => syncRouteFromLocation();
      window.addEventListener('popstate', handler);
      popstateHandler = () => window.removeEventListener('popstate', handler);
    }
  });

  onDestroy(() => {
    websocketStore.cleanup();
    cleanupNotifications();
    popstateHandler?.();
  });

  function syncRouteFromLocation() {
    if (typeof window === 'undefined') return;
    const path = window.location.pathname;
    const historyState = getHistoryState();
    const searchState = historyState?.search;
    if (searchState?.query) {
      searchStore.setScope(searchState.scope);
      searchStore.setQuery(searchState.query);
    } else {
      searchStore.setQuery('');
    }
    highlightCommentId = parseThreadCommentId(window.location.search);
    sectionNotFound = null;
    const { isReset, token } = parseResetRoute(window.location);
    if (isReset) {
      unauthRoute = 'reset';
      resetToken = token;
      pendingSectionIdentifier = null;
      pendingThreadPostId = null;
      pendingAdminPath = false;
      highlightCommentId = null;
      return;
    }
    const profileUserId = parseProfileUserId(path);
    if (profileUserId) {
      uiStore.openProfile(profileUserId);
      threadRouteStore.clearTarget();
      pendingSectionIdentifier = null;
      pendingThreadPostId = null;
      pendingAdminPath = false;
      highlightCommentId = null;
    } else {
      const standaloneThreadPostId = parseStandaloneThreadPostId(path);
      if (standaloneThreadPostId) {
        uiStore.setActiveView('thread');
        threadRouteStore.setTarget(standaloneThreadPostId, null);
        pendingSectionIdentifier = null;
        pendingThreadPostId = null;
        pendingAdminPath = false;
        return;
      }
      const threadPostId = parseThreadPostId(path);
      const sectionIdentifier = parseSectionSlug(path);
      if (threadPostId && sectionIdentifier) {
        const availableSections = get(sections);
        if (availableSections.length > 0) {
          const match = findSectionByIdentifier(availableSections, sectionIdentifier);
          if (match) {
            threadRouteStore.setTarget(threadPostId, match.id);
            uiStore.setActiveView('thread');
          } else {
            threadRouteStore.clearTarget();
            sectionNotFound = sectionIdentifier;
          }
          pendingThreadPostId = null;
        } else {
          uiStore.setActiveView('thread');
          pendingThreadPostId = threadPostId;
          threadRouteStore.setTarget(threadPostId, null);
        }
      } else {
        threadRouteStore.clearTarget();
        highlightCommentId = null;
      }
      if (sectionIdentifier) {
        const availableSections = get(sections);
        if (availableSections.length > 0) {
          const match = findSectionByIdentifier(availableSections, sectionIdentifier);
          if (match) {
            sectionStore.setActiveSection(match);
            const slug = getSectionSlug(match);
            if (sectionIdentifier !== slug) {
              const targetPath = threadPostId
                ? buildThreadHref(slug, threadPostId)
                : buildFeedHref(slug);
              replacePath(targetPath);
            }
          } else {
            sectionNotFound = sectionIdentifier;
          }
          pendingSectionIdentifier = null;
        } else {
          pendingSectionIdentifier = sectionIdentifier;
        }
        pendingAdminPath = false;
        uiStore.setActiveView(threadPostId ? 'thread' : 'feed');
      } else if (isSettingsPath(path)) {
        pendingSectionIdentifier = null;
        pendingThreadPostId = null;
        pendingAdminPath = false;
        threadRouteStore.clearTarget();
        uiStore.setActiveView('settings');
        highlightCommentId = null;
      } else if (isAdminPath(path)) {
        pendingSectionIdentifier = null;
        pendingThreadPostId = null;
        highlightCommentId = null;
        if (get(authStore).isLoading) {
          pendingAdminPath = true;
          return;
        }
        pendingAdminPath = false;
        if (get(isAdmin)) {
          uiStore.setActiveView('admin');
        } else if (get(isAuthenticated)) {
          uiStore.setActiveView('feed');
          const fallbackSectionId =
            get(activeSection)?.id ?? get(sections)[0]?.id ?? null;
          const fallbackSection = get(sections).find(
            (section) => section.id === fallbackSectionId
          );
          replacePath(buildFeedHref(fallbackSection ? getSectionSlug(fallbackSection) : null));
        } else {
          uiStore.setActiveView('feed');
        }
      } else {
        pendingSectionIdentifier = null;
        pendingThreadPostId = null;
        pendingAdminPath = false;
        uiStore.setActiveView('feed');
        highlightCommentId = null;
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
    sectionStore.loadSections();
  }

  $: if (!$isAuthenticated && sectionsLoadedForSession) {
    sectionsLoadedForSession = false;
    sectionStore.setSections([]);
  }

  $: if (pendingSectionIdentifier && $sections.length > 0) {
    const match = findSectionByIdentifier($sections, pendingSectionIdentifier) ?? null;
    const hasThreadTarget =
      $threadRouteStore.postId && $threadRouteStore.sectionId === match?.id;
    if (match) {
      sectionStore.setActiveSection(match);
      if (pendingThreadPostId) {
        threadRouteStore.setTarget(pendingThreadPostId, match.id);
        uiStore.setActiveView('thread');
        replacePath(buildThreadHref(getSectionSlug(match), pendingThreadPostId));
        pendingThreadPostId = null;
      } else if (!hasThreadTarget) {
        uiStore.setActiveView('feed');
        replacePath(buildFeedHref(getSectionSlug(match)));
      }
    } else {
      sectionNotFound = pendingSectionIdentifier;
      if (pendingThreadPostId) {
        threadRouteStore.clearTarget();
        pendingThreadPostId = null;
      }
    }
    pendingSectionIdentifier = null;
  }

  $: if (pendingAdminPath && !$authStore.isLoading && typeof window !== 'undefined') {
    if (!isAdminPath(window.location.pathname)) {
      pendingAdminPath = false;
    } else if ($isAdmin) {
      uiStore.setActiveView('admin');
      pendingAdminPath = false;
    } else if ($isAuthenticated) {
      uiStore.setActiveView('feed');
      const fallbackSectionId =
        get(activeSection)?.id ?? get(sections)[0]?.id ?? null;
      const fallbackSection = get(sections).find(
        (section) => section.id === fallbackSectionId
      );
      replacePath(buildFeedHref(fallbackSection ? getSectionSlug(fallbackSection) : null));
      pendingAdminPath = false;
    } else {
      pendingAdminPath = false;
    }
  }

  $: if (sectionNotFound && $activeSection && typeof window !== 'undefined') {
    const activeSlug = getSectionSlug($activeSection);
    if (window.location.pathname.startsWith(`/sections/${activeSlug}`)) {
      sectionNotFound = null;
    }
  }
</script>

<ErrorBoundary>
  {#if $authStore.isLoading}
    <div class="min-h-screen flex items-center justify-center bg-gray-50">
      <div class="flex flex-col items-center">
        <svg
          class="animate-spin h-10 w-10 text-indigo-600 mb-4"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
        >
          <circle
            class="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            stroke-width="4"
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
        {:else if $activeView === 'settings'}
          <Settings />
        {:else if sectionNotFound}
          <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
            <h1 class="text-xl font-semibold text-gray-900 mb-2">Section not found</h1>
            <p class="text-gray-600">
              We couldn’t find a section named “{sectionNotFound}”. Check the URL or pick a section
              from the sidebar.
            </p>
          </div>
        {:else if $activeView === 'thread'}
          <ThreadView {highlightCommentId} />
        {:else if $activeSection}
          <div class="flex items-center gap-3">
            <span class="text-3xl">{$activeSection.icon}</span>
            <h1 class="text-2xl font-bold text-gray-900">{$activeSection.name}</h1>
          </div>

          <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
            <PostForm />
          </div>

          <MusicLinksContainer />

          {#if $isSearching || $searchError || $lastSearchQuery.trim().length > 0}
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
</ErrorBoundary>
