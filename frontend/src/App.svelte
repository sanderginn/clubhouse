<script lang="ts">
  import { get } from 'svelte/store';
  import { onMount, onDestroy } from 'svelte';
  import './styles/globals.css';
  import { Layout, PostForm, SectionFeed, SearchResults, InstallPrompt } from './components';
  import MusicLinksContainer from './components/MusicLinksContainer.svelte';
  import ThreadView from './components/ThreadView.svelte';
  import UserProfile from './components/UserProfile.svelte';
  import Watchlist from './components/movies/Watchlist.svelte';
  import Cookbook from './components/recipes/Cookbook.svelte';
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
    buildSectionWatchlistHref,
    buildThreadHref,
    getHistoryState,
    isAdminPath,
    isSettingsPath,
    isWatchlistPath,
    parseSectionWatchlistSlug,
    parseStandaloneThreadPostId,
    parseSectionSlug,
    parseThreadCommentId,
    parseThreadPostId,
    pushPath,
    replacePath,
  } from './services/routeNavigation';
  import { parseResetRoute } from './services/resetLink';
  import { findSectionByIdentifier, getSectionSlug } from './services/sectionSlug';
  import type { Section } from './stores/sectionStore';
  import ErrorBoundary from './lib/components/ErrorBoundary.svelte';

  let unauthRoute: 'login' | 'register' | 'reset' = 'login';
  let resetToken: string | null = null;
  let sectionsLoadedForSession = false;
  let popstateHandler: (() => void) | null = null;
  let pendingSectionIdentifier: string | null = null;
  let pendingSectionSubview: 'feed' | 'watchlist' = 'feed';
  let pendingThreadPostId: string | null = null;
  let pendingLegacyWatchlistRoute = false;
  let pendingAdminPath = false;
  let sectionNotFound: string | null = null;
  let highlightCommentId: string | null = null;
  let sectionSubview: 'feed' | 'watchlist' = 'feed';

  function isWatchlistSection(section: Pick<Section, 'type'> | null): boolean {
    if (!section) return false;
    return section.type === 'movie' || section.type === 'series';
  }

  function getPreferredWatchlistSection(sectionList: Section[]): Section | null {
    return (
      sectionList.find((section) => section.type === 'movie') ??
      sectionList.find((section) => section.type === 'series') ??
      null
    );
  }

  function openSectionFeedView() {
    const section = get(activeSection);
    if (!section) return;
    sectionSubview = 'feed';
    threadRouteStore.clearTarget();
    uiStore.setActiveView('feed');
    pushPath(buildFeedHref(getSectionSlug(section)));
  }

  function openSectionWatchlistView() {
    const section = get(activeSection);
    if (!section || !isWatchlistSection(section)) return;
    sectionSubview = 'watchlist';
    threadRouteStore.clearTarget();
    uiStore.setActiveView('feed');
    pushPath(buildSectionWatchlistHref(getSectionSlug(section)));
  }

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
    const sectionWatchlistIdentifier = parseSectionWatchlistSlug(path);
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
      pendingSectionSubview = 'feed';
      pendingThreadPostId = null;
      pendingLegacyWatchlistRoute = false;
      pendingAdminPath = false;
      highlightCommentId = null;
      sectionSubview = 'feed';
      return;
    }
    if (isWatchlistPath(path)) {
      unauthRoute = 'login';
      resetToken = null;
      threadRouteStore.clearTarget();
      pendingSectionIdentifier = null;
      pendingSectionSubview = 'feed';
      pendingThreadPostId = null;
      pendingLegacyWatchlistRoute = false;
      pendingAdminPath = false;
      highlightCommentId = null;
      uiStore.setActiveView('feed');
      const availableSections = get(sections);
      const preferredSection =
        getPreferredWatchlistSection(availableSections) ??
        (isWatchlistSection(get(activeSection)) ? get(activeSection) : null);
      if (preferredSection) {
        sectionStore.setActiveSection(preferredSection);
        sectionSubview = 'watchlist';
        replacePath(buildSectionWatchlistHref(getSectionSlug(preferredSection)));
      } else {
        sectionSubview = 'feed';
        pendingLegacyWatchlistRoute = true;
      }
      return;
    }
    const profileUserId = parseProfileUserId(path);
    if (profileUserId) {
      uiStore.openProfile(profileUserId);
      threadRouteStore.clearTarget();
      pendingSectionIdentifier = null;
      pendingSectionSubview = 'feed';
      pendingThreadPostId = null;
      pendingLegacyWatchlistRoute = false;
      pendingAdminPath = false;
      highlightCommentId = null;
      sectionSubview = 'feed';
    } else {
      const standaloneThreadPostId = parseStandaloneThreadPostId(path);
      if (standaloneThreadPostId) {
        uiStore.setActiveView('thread');
        threadRouteStore.setTarget(standaloneThreadPostId, null);
        pendingSectionIdentifier = null;
        pendingSectionSubview = 'feed';
        pendingThreadPostId = null;
        pendingLegacyWatchlistRoute = false;
        pendingAdminPath = false;
        sectionSubview = 'feed';
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
        sectionSubview = 'feed';
      } else {
        threadRouteStore.clearTarget();
        highlightCommentId = null;
      }
      if (sectionIdentifier) {
        const availableSections = get(sections);
        const wantsSectionWatchlist = sectionWatchlistIdentifier !== null;
        if (availableSections.length > 0) {
          const match = findSectionByIdentifier(availableSections, sectionIdentifier);
          if (match) {
            sectionStore.setActiveSection(match);
            const slug = getSectionSlug(match);
            const showWatchlistInSection = wantsSectionWatchlist && isWatchlistSection(match);
            sectionSubview = showWatchlistInSection ? 'watchlist' : 'feed';
            if (wantsSectionWatchlist && !isWatchlistSection(match)) {
              replacePath(buildFeedHref(slug));
            } else if (sectionIdentifier !== slug) {
              const targetPath = threadPostId
                ? buildThreadHref(slug, threadPostId)
                : showWatchlistInSection
                  ? buildSectionWatchlistHref(slug)
                  : buildFeedHref(slug);
              replacePath(targetPath);
            }
          } else {
            sectionNotFound = sectionIdentifier;
            sectionSubview = 'feed';
          }
          pendingSectionIdentifier = null;
          pendingSectionSubview = 'feed';
        } else {
          pendingSectionIdentifier = sectionIdentifier;
          pendingSectionSubview = wantsSectionWatchlist ? 'watchlist' : 'feed';
        }
        pendingLegacyWatchlistRoute = false;
        pendingAdminPath = false;
        uiStore.setActiveView(threadPostId ? 'thread' : 'feed');
      } else if (isSettingsPath(path)) {
        pendingSectionIdentifier = null;
        pendingSectionSubview = 'feed';
        pendingThreadPostId = null;
        pendingLegacyWatchlistRoute = false;
        pendingAdminPath = false;
        threadRouteStore.clearTarget();
        uiStore.setActiveView('settings');
        highlightCommentId = null;
        sectionSubview = 'feed';
      } else if (isAdminPath(path)) {
        pendingSectionIdentifier = null;
        pendingSectionSubview = 'feed';
        pendingThreadPostId = null;
        pendingLegacyWatchlistRoute = false;
        highlightCommentId = null;
        sectionSubview = 'feed';
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
        pendingSectionSubview = 'feed';
        pendingThreadPostId = null;
        pendingLegacyWatchlistRoute = false;
        pendingAdminPath = false;
        uiStore.setActiveView('feed');
        highlightCommentId = null;
        sectionSubview = 'feed';
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
        sectionSubview = 'feed';
        threadRouteStore.setTarget(pendingThreadPostId, match.id);
        uiStore.setActiveView('thread');
        replacePath(buildThreadHref(getSectionSlug(match), pendingThreadPostId));
        pendingThreadPostId = null;
      } else if (pendingSectionSubview === 'watchlist' && isWatchlistSection(match)) {
        sectionSubview = 'watchlist';
        uiStore.setActiveView('feed');
        replacePath(buildSectionWatchlistHref(getSectionSlug(match)));
      } else if (!hasThreadTarget) {
        sectionSubview = 'feed';
        uiStore.setActiveView('feed');
        replacePath(buildFeedHref(getSectionSlug(match)));
      }
    } else {
      sectionNotFound = pendingSectionIdentifier;
      sectionSubview = 'feed';
      if (pendingThreadPostId) {
        threadRouteStore.clearTarget();
        pendingThreadPostId = null;
      }
    }
    pendingSectionIdentifier = null;
    pendingSectionSubview = 'feed';
  }

  $: if (pendingLegacyWatchlistRoute && $sections.length > 0) {
    const preferred = getPreferredWatchlistSection($sections);
    pendingLegacyWatchlistRoute = false;
    if (preferred) {
      sectionStore.setActiveSection(preferred);
      sectionSubview = 'watchlist';
      uiStore.setActiveView('feed');
      replacePath(buildSectionWatchlistHref(getSectionSlug(preferred)));
    } else {
      sectionSubview = 'feed';
      const fallbackSection = $activeSection ?? $sections[0] ?? null;
      replacePath(buildFeedHref(fallbackSection ? getSectionSlug(fallbackSection) : null));
    }
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

  $: if (
    typeof window !== 'undefined' &&
    isWatchlistPath(window.location.pathname) &&
    $isAuthenticated
  ) {
    const preferredSection =
      getPreferredWatchlistSection($sections) ??
      (isWatchlistSection($activeSection) ? $activeSection : null);
    if (preferredSection) {
      if ($activeSection?.id !== preferredSection.id) {
        sectionStore.setActiveSection(preferredSection);
      }
      sectionSubview = 'watchlist';
      if ($activeView !== 'feed') {
        uiStore.setActiveView('feed');
      }
      threadRouteStore.clearTarget();
    }
  }

  $: if (sectionSubview === 'watchlist' && $activeSection && typeof window !== 'undefined') {
    const watchlistSlug = parseSectionWatchlistSlug(window.location.pathname);
    const isLegacyWatchlistRoute = isWatchlistPath(window.location.pathname);
    if (
      !isLegacyWatchlistRoute &&
      (!watchlistSlug || watchlistSlug !== getSectionSlug($activeSection))
    ) {
      sectionSubview = 'feed';
    }
  }

  $: if (typeof document !== 'undefined') {
    if (sectionSubview === 'watchlist' && $activeSection && isWatchlistSection($activeSection)) {
      document.title = `${$activeSection.name} Watchlist - Clubhouse`;
    } else {
      document.title = 'Clubhouse';
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
          {@const supportsWatchlist = isWatchlistSection($activeSection)}
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex items-center gap-3">
              <span class="text-3xl">{$activeSection.icon}</span>
              <h1 class="text-2xl font-bold text-gray-900">{$activeSection.name}</h1>
            </div>
            {#if supportsWatchlist}
              <div
                class="inline-flex items-center gap-1 rounded-full bg-gray-100 p-1"
                role="tablist"
                aria-label={`${$activeSection.name} section views`}
              >
                <button
                  type="button"
                  role="tab"
                  class={`rounded-full px-3 py-1 text-xs font-semibold transition-colors ${
                    sectionSubview === 'feed'
                      ? 'bg-white text-gray-900 shadow-sm'
                      : 'text-gray-600 hover:text-gray-800'
                  }`}
                  aria-selected={sectionSubview === 'feed'}
                  on:click={openSectionFeedView}
                  data-testid="section-tab-feed"
                >
                  Feed
                </button>
                <button
                  type="button"
                  role="tab"
                  class={`rounded-full px-3 py-1 text-xs font-semibold transition-colors ${
                    sectionSubview === 'watchlist'
                      ? 'bg-white text-gray-900 shadow-sm'
                      : 'text-gray-600 hover:text-gray-800'
                  }`}
                  aria-selected={sectionSubview === 'watchlist'}
                  on:click={openSectionWatchlistView}
                  data-testid="section-tab-watchlist"
                >
                  Watchlist
                </button>
              </div>
            {/if}
          </div>

          {#if supportsWatchlist && sectionSubview === 'watchlist'}
            <Watchlist />
          {:else}
            <!-- Section-specific components should render above PostForm for consistency. -->
            {#if $activeSection.type === 'recipe'}
              <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
                <Cookbook />
              </div>
            {/if}

            <MusicLinksContainer />

            <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
              <PostForm />
            </div>

            {#if $isSearching || $searchError || $lastSearchQuery.trim().length > 0}
              <SearchResults />
            {:else}
              <SectionFeed />
            {/if}
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
