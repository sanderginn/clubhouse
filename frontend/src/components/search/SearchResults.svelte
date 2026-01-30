<script lang="ts">
  import {
    searchResults,
    isSearching,
    searchError,
    searchQuery,
    lastSearchQuery,
    lastSearchScope,
    searchScope,
    activeSection,
    searchStore,
    postStore,
    sections,
    sectionStore,
    uiStore,
    threadRouteStore,
  } from '../../stores';
  import PostCard from '../PostCard.svelte';
  import ReactionBar from '../reactions/ReactionBar.svelte';
  import LinkifiedText from '../LinkifiedText.svelte';
  import EditedBadge from '../EditedBadge.svelte';
  import RelativeTime from '../RelativeTime.svelte';
  import { api } from '../../services/api';
  import { buildProfileHref, handleProfileNavigation } from '../../services/profileNavigation';
  import { buildThreadHref, pushPath } from '../../services/routeNavigation';
  import { getSectionSlug } from '../../services/sectionSlug';
  import { getImageLinkUrl, isInternalUploadUrl, stripInternalUploadUrls } from '../../services/linkUtils';
  import type { Post } from '../../stores/postStore';
  import type { CommentResult, SearchResult } from '../../stores/searchStore';
  import { logError } from '../../lib/observability/logger';

  // Track pending reactions to prevent double-clicks
  let pendingReactions = new Set<string>();

  async function toggleCommentReaction(comment: CommentResult, emoji: string) {
    const key = `${comment.id}-${emoji}`;
    if (pendingReactions.has(key)) return;

    const userReactions = new Set(comment.viewerReactions ?? []);
    const hasReacted = userReactions.has(emoji);

    // Optimistic update
    pendingReactions.add(key);
    if (hasReacted) {
      comment.viewerReactions = (comment.viewerReactions ?? []).filter((e) => e !== emoji);
      if (comment.reactionCounts && comment.reactionCounts[emoji]) {
        comment.reactionCounts[emoji]--;
        if (comment.reactionCounts[emoji] <= 0) {
          delete comment.reactionCounts[emoji];
        }
      }
    } else {
      comment.viewerReactions = [...(comment.viewerReactions ?? []), emoji];
      comment.reactionCounts = { ...(comment.reactionCounts ?? {}), [emoji]: (comment.reactionCounts?.[emoji] ?? 0) + 1 };
    }
    // Trigger reactivity
    $searchResults = $searchResults;

    try {
      if (hasReacted) {
        await api.removeCommentReaction(comment.id, emoji);
      } else {
        await api.addCommentReaction(comment.id, emoji);
      }
    } catch (e) {
      logError('Failed to toggle comment reaction', { commentId: comment.id, emoji }, e);
      // Revert on error
      if (hasReacted) {
        comment.viewerReactions = [...(comment.viewerReactions ?? []), emoji];
        comment.reactionCounts = { ...(comment.reactionCounts ?? {}), [emoji]: (comment.reactionCounts?.[emoji] ?? 0) + 1 };
      } else {
        comment.viewerReactions = (comment.viewerReactions ?? []).filter((e) => e !== emoji);
        if (comment.reactionCounts && comment.reactionCounts[emoji]) {
          comment.reactionCounts[emoji]--;
          if (comment.reactionCounts[emoji] <= 0) {
            delete comment.reactionCounts[emoji];
          }
        }
      }
      $searchResults = $searchResults;
    } finally {
      pendingReactions.delete(key);
    }
  }

  function openPostThread(post: Post | undefined) {
    if (!post) return;
    const targetSection = $sections.find((section) => section.id === post.sectionId);
    const targetSectionId = targetSection?.id ?? post.sectionId;
    const targetSectionSlug = targetSection ? getSectionSlug(targetSection) : post.sectionId;
    const switchingSection = targetSection && $activeSection?.id !== targetSection.id;
    uiStore.setActiveView('thread');
    threadRouteStore.setTarget(post.id, targetSectionId);
    pushPath(buildThreadHref(targetSectionSlug, post.id), { fromSearch: true });
    if (switchingSection) {
      sectionStore.setActiveSection(targetSection);
    }
    if (switchingSection) {
      let resolved = false;
      let sawLoading = false;
      let shouldUnsubscribe = false;
      let unsubscribe: (() => void) | null = null;
      const maybeUnsubscribe = () => {
        if (unsubscribe) {
          unsubscribe();
          unsubscribe = null;
        } else {
          shouldUnsubscribe = true;
        }
      };
      unsubscribe = postStore.subscribe((state) => {
        if (resolved) return;
        if (state.isLoading) {
          sawLoading = true;
          return;
        }
        if (!sawLoading) return;
        resolved = true;
        postStore.upsertPost(post);
        maybeUnsubscribe();
      });
      if (shouldUnsubscribe && unsubscribe) {
        unsubscribe();
        unsubscribe = null;
      }
    } else {
      postStore.upsertPost(post);
    }
    searchStore.setQuery('');
    if (typeof window !== 'undefined') {
      window.scrollTo({ top: 0, behavior: 'smooth' });
    }
  }

  type SearchResultWithLink = SearchResult & { linkMetadata?: { id?: string } };

  function resultKey(result: SearchResult, index: number): string {
    if (result.type === 'comment') {
      return `comment-${result.comment?.id ?? index}`;
    }
    if (result.type === 'post') {
      return `post-${result.post?.id ?? index}`;
    }
    const linkId = (result as SearchResultWithLink).linkMetadata?.id;
    return linkId ? `link-${linkId}` : `${result.type}-${index}`;
  }

  type SectionGroup = {
    id: string | null;
    name: string;
    icon: string | null;
    results: SearchResult[];
  };

  function resolveSectionId(result: SearchResult, fallbackSectionId: string | null): string | null {
    return (
      (result.type === 'post' && result.post?.sectionId) ||
      (result.type === 'comment' && (result.comment?.sectionId || result.post?.sectionId)) ||
      fallbackSectionId ||
      null
    );
  }

  function resolveSectionNameById(
    sectionId: string | null,
    fallbackName: string | null,
    availableSections: typeof $sections,
  ): string | null {
    if (!sectionId) return fallbackName;
    const section = availableSections.find((item) => item.id === sectionId);
    return section?.name ?? fallbackName;
  }

  function resolveSectionIconById(
    sectionId: string | null,
    fallbackIcon: string | null,
    availableSections: typeof $sections,
  ): string | null {
    if (!sectionId) return fallbackIcon;
    const section = availableSections.find((item) => item.id === sectionId);
    return section?.icon ?? fallbackIcon;
  }

  function resolveSectionName(result: SearchResult, fallbackSectionId: string | null, fallbackName: string | null): string | null {
    const sectionId = resolveSectionId(result, fallbackSectionId);
    return resolveSectionNameById(sectionId, fallbackName, $sections);
  }

  function resolveSectionIcon(
    result: SearchResult,
    fallbackSectionId: string | null,
    fallbackIcon: string | null,
  ): string | null {
    const sectionId = resolveSectionId(result, fallbackSectionId);
    return resolveSectionIconById(sectionId, fallbackIcon, $sections);
  }

  function buildSectionGroups(results: SearchResult[], availableSections: typeof $sections): SectionGroup[] {
    const groups: SectionGroup[] = [];
    const seen = new Map<string, SectionGroup>();
    for (const result of results) {
      const sectionId = resolveSectionId(result, null);
      const key = sectionId ?? 'unknown-section';
      let group = seen.get(key);
      if (!group) {
        const name = resolveSectionNameById(sectionId, null, availableSections) ?? 'Unknown section';
        const icon = resolveSectionIconById(sectionId, '📁', availableSections);
        group = { id: sectionId, name, icon, results: [] };
        seen.set(key, group);
        groups.push(group);
      }
      group.results.push(result);
    }
    return groups;
  }

  const sectionPillClass =
    'inline-flex items-center gap-1.5 rounded-full border border-gray-200 bg-gray-100 px-3 py-1 text-sm font-semibold text-gray-600 max-w-full min-w-0';
  const sectionPillIconClass = 'text-base leading-none';
  const sectionPillTextClass = 'truncate';

  $: normalizedQuery = $searchQuery.trim();
  $: hasQuery = normalizedQuery.length > 0;
  $: showResults = $lastSearchQuery && $lastSearchQuery === normalizedQuery;
  $: displayScope = showResults && $lastSearchScope ? $lastSearchScope : $searchScope;
  $: isGlobalScope = displayScope === 'global';
  $: showParentSectionPill = displayScope === 'global';
  $: sectionGroups = isGlobalScope ? buildSectionGroups($searchResults, $sections) : [];
</script>

<section class="space-y-4">
  {#if !hasQuery}
    <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6 text-center">
      <p class="text-gray-500">Start typing to search posts and comments.</p>
    </div>
  {:else if $isSearching}
    <div class="flex justify-center py-8">
      <div class="flex items-center gap-2 text-gray-500">
        <svg class="animate-spin h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          />
        </svg>
        <span>Searching...</span>
      </div>
    </div>
  {:else if $searchError}
    <div class="bg-red-50 border border-red-200 rounded-lg p-4 text-center">
      <p class="text-red-600">{$searchError}</p>
    </div>
  {:else if showResults && $searchResults.length === 0}
    <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6 text-center">
      <p class="text-gray-500">No results for "{$lastSearchQuery}".</p>
    </div>
  {:else if !showResults}
    <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6 text-center">
      <p class="text-gray-500">Press Search to see results.</p>
      {#if $lastSearchScope && $lastSearchScope !== $searchScope}
        <p class="text-sm text-gray-400 mt-2">
          Scope changed to {$searchScope === 'global' ? 'Everywhere' : 'current section'}.
        </p>
      {/if}
    </div>
  {:else}
    <div class="flex items-center justify-between text-sm text-gray-500">
      <span>
        Showing {$searchResults.length} result{$searchResults.length === 1 ? '' : 's'} for
        "{$lastSearchQuery}"
      </span>
      <span>
        {#if displayScope === 'global'}
          All sections
        {:else if $activeSection}
          {$activeSection.name}
        {:else}
          Section
        {/if}
      </span>
    </div>

    {#if $lastSearchScope && $lastSearchScope !== $searchScope}
      <div class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-2 text-sm text-amber-700">
        Scope changed to {$searchScope === 'global' ? 'Everywhere' : 'current section'}. Press Search to refresh.
      </div>
    {/if}

    {#if isGlobalScope}
      {#each sectionGroups as group}
        <div class="border border-gray-200 rounded-lg bg-white shadow-sm">
          <div class="flex items-center justify-between px-4 py-2 border-b border-gray-200 bg-gray-50 rounded-t-lg">
            <span class={sectionPillClass}>
              {#if group.icon}
                <span class={sectionPillIconClass} aria-hidden="true">{group.icon}</span>
              {/if}
              <span class={sectionPillTextClass}>{group.name}</span>
            </span>
            <span class="text-xs text-gray-400">{group.results.length} result{group.results.length === 1 ? '' : 's'}</span>
          </div>
          <div class="space-y-4 p-4">
            {#each group.results as result, index (resultKey(result, index))}
              {#if result.type === 'post' && result.post}
                <PostCard post={result.post} />
              {:else if result.type === 'comment' && result.comment}
                {@const comment = result.comment}
                {@const parentPost = result.post}
                <article class="bg-white rounded-lg shadow-sm border border-gray-200 p-4 space-y-4">
                  {#if parentPost}
                    {@const parentPostHasInternalImage =
                      parentPost.links?.some((link) => isInternalUploadUrl(link.url) && getImageLinkUrl(link)) ??
                      false}
                    {@const parentPostContent = parentPostHasInternalImage
                      ? stripInternalUploadUrls(parentPost.content)
                      : parentPost.content}
                    {@const parentPostLink =
                      parentPost.links?.find((link) => !isInternalUploadUrl(link.url)) ?? null}
                    <div class="rounded-lg border border-gray-200 bg-gray-50 p-3">
                      <div class="flex items-center justify-between text-xs text-gray-500 mb-2">
                        <div class="flex items-center gap-2">
                          <span>Parent post</span>
                        </div>
                        <button
                          type="button"
                          class="text-blue-600 hover:text-blue-800 underline"
                          on:click={() => openPostThread(parentPost)}
                        >
                          View full thread
                        </button>
                      </div>
                      <div class="flex items-start gap-3">
                        {#if parentPost.user?.profilePictureUrl}
                          <img
                            src={parentPost.user.profilePictureUrl}
                            alt={parentPost.user.username}
                            class="w-8 h-8 rounded-full object-cover flex-shrink-0"
                          />
                        {:else}
                          <div class="w-8 h-8 rounded-full bg-gray-200 flex items-center justify-center flex-shrink-0">
                            <span class="text-gray-500 text-xs font-medium">
                              {parentPost.user?.username?.charAt(0).toUpperCase() || '?'}
                            </span>
                          </div>
                        {/if}

                        <div class="flex-1 min-w-0">
                          <div class="flex items-center gap-2 mb-1">
                            <span class="text-sm font-medium text-gray-900 truncate">
                              {parentPost.user?.username || 'Unknown'}
                            </span>
                            <span class="text-gray-400 text-xs">·</span>
                            <RelativeTime dateString={parentPost.createdAt} className="text-gray-500 text-xs" />
                            <EditedBadge createdAt={parentPost.createdAt} updatedAt={parentPost.updatedAt} />
                          </div>
                          <p class="text-gray-800 text-sm whitespace-pre-wrap break-words line-clamp-3">
                            {parentPostContent}
                          </p>
                          {#if parentPostLink}
                            <div class="mt-2 text-xs text-blue-600 break-all">
                              <a
                                href={parentPostLink.url}
                                target="_blank"
                                rel="noopener noreferrer"
                                class="underline"
                              >
                                {parentPostLink.url}
                              </a>
                            </div>
                          {/if}
                        </div>
                      </div>
                    </div>
                  {/if}

                  <div class="flex items-start gap-3">
                    {#if comment.user?.id}
                      <a
                        href={buildProfileHref(comment.user.id)}
                        class="flex-shrink-0"
                        on:click={(event) => handleProfileNavigation(event, comment.user?.id)}
                        aria-label={`View ${(comment.user?.username ?? 'user')}'s profile`}
                      >
                        {#if comment.user?.profilePictureUrl}
                          <img
                            src={comment.user.profilePictureUrl}
                            alt={comment.user.username}
                            class="w-9 h-9 rounded-full object-cover"
                          />
                        {:else}
                          <div class="w-9 h-9 rounded-full bg-gray-200 flex items-center justify-center">
                            <span class="text-gray-500 text-sm font-medium">
                              {comment.user?.username?.charAt(0).toUpperCase() || '?'}
                            </span>
                          </div>
                        {/if}
                      </a>
                    {:else}
                      {#if comment.user?.profilePictureUrl}
                        <img
                          src={comment.user.profilePictureUrl}
                          alt={comment.user.username}
                          class="w-9 h-9 rounded-full object-cover flex-shrink-0"
                        />
                      {:else}
                        <div class="w-9 h-9 rounded-full bg-gray-200 flex items-center justify-center flex-shrink-0">
                          <span class="text-gray-500 text-sm font-medium">
                            {comment.user?.username?.charAt(0).toUpperCase() || '?'}
                          </span>
                        </div>
                      {/if}
                    {/if}

                    <div class="flex-1 min-w-0">
                      <div class="flex items-center gap-2 mb-1">
                        {#if comment.user?.id}
                          <a
                            href={buildProfileHref(comment.user.id)}
                            class="font-medium text-gray-900 truncate hover:underline"
                            on:click={(event) => handleProfileNavigation(event, comment.user?.id)}
                          >
                            {comment.user?.username || 'Unknown'}
                          </a>
                        {:else}
                          <span class="font-medium text-gray-900 truncate">
                            {comment.user?.username || 'Unknown'}
                          </span>
                        {/if}
                        <span class="text-gray-400 text-sm">commented</span>
                        <RelativeTime dateString={comment.createdAt} className="text-gray-500 text-sm" />
                        <EditedBadge createdAt={comment.createdAt} updatedAt={comment.updatedAt} />
                      </div>

                      <LinkifiedText
                        text={comment.content}
                        highlightQuery={normalizedQuery}
                        className="text-gray-800 whitespace-pre-wrap break-words"
                        linkClassName="text-blue-600 hover:text-blue-800 underline"
                      />

                      {#if comment.links && comment.links.length > 0}
                        <div class="mt-2 text-sm text-blue-600 break-all">
                          <a
                            href={comment.links[0].url}
                            target="_blank"
                            rel="noopener noreferrer"
                            class="underline"
                          >
                            {comment.links[0].url}
                          </a>
                        </div>
                      {/if}

                      <div class="mt-3">
                        <ReactionBar
                          reactionCounts={comment.reactionCounts ?? {}}
                          userReactions={new Set(comment.viewerReactions ?? [])}
                          onToggle={(emoji) => toggleCommentReaction(comment, emoji)}
                          commentId={comment.id}
                        />
                      </div>
                    </div>
                  </div>
                </article>
              {/if}
            {/each}
          </div>
        </div>
      {/each}
    {:else}
      {#each $searchResults as result, index (resultKey(result, index))}
        {@const sectionName = resolveSectionName(result, $activeSection?.id ?? null, $activeSection?.name ?? null)}
        {@const sectionIcon = resolveSectionIcon(result, $activeSection?.id ?? null, $activeSection?.icon ?? null)}
      {#if result.type === 'post' && result.post}
        {#if sectionName}
          <div class="inline-flex items-center gap-2 text-gray-500 min-w-0">
            <span class={sectionPillClass}>
              {#if sectionIcon}
                <span class={sectionPillIconClass} aria-hidden="true">{sectionIcon}</span>
              {/if}
              <span class={sectionPillTextClass}>{sectionName}</span>
            </span>
          </div>
        {/if}
        <PostCard post={result.post} />
      {:else if result.type === 'comment' && result.comment}
        {@const comment = result.comment}
        {@const parentPost = result.post}
        <article class="bg-white rounded-lg shadow-sm border border-gray-200 p-4 space-y-4">
          {#if parentPost}
            <div class="rounded-lg border border-gray-200 bg-gray-50 p-3">
              <div class="flex items-center justify-between text-xs text-gray-500 mb-2">
                <div class="flex items-center gap-2">
                  <span>Parent post</span>
                  {#if sectionName && showParentSectionPill}
                    <span class={sectionPillClass}>
                      {#if sectionIcon}
                        <span class={sectionPillIconClass} aria-hidden="true">{sectionIcon}</span>
                      {/if}
                      <span class={sectionPillTextClass}>{sectionName}</span>
                    </span>
                  {/if}
                </div>
                <button
                  type="button"
                  class="text-blue-600 hover:text-blue-800 underline"
                  on:click={() => openPostThread(parentPost)}
                >
                  View full thread
                </button>
              </div>
              <div class="flex items-start gap-3">
                {#if parentPost.user?.profilePictureUrl}
                  <img
                    src={parentPost.user.profilePictureUrl}
                    alt={parentPost.user.username}
                    class="w-8 h-8 rounded-full object-cover flex-shrink-0"
                  />
                {:else}
                  <div class="w-8 h-8 rounded-full bg-gray-200 flex items-center justify-center flex-shrink-0">
                    <span class="text-gray-500 text-xs font-medium">
                      {parentPost.user?.username?.charAt(0).toUpperCase() || '?'}
                    </span>
                  </div>
                {/if}

                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2 mb-1">
                    <span class="text-sm font-medium text-gray-900 truncate">
                      {parentPost.user?.username || 'Unknown'}
                    </span>
                    <span class="text-gray-400 text-xs">·</span>
                    <RelativeTime dateString={parentPost.createdAt} className="text-gray-500 text-xs" />
                    <EditedBadge createdAt={parentPost.createdAt} updatedAt={parentPost.updatedAt} />
                  </div>
                  <p class="text-gray-800 text-sm whitespace-pre-wrap break-words line-clamp-3">
                    {parentPost.content}
                  </p>
                  {#if parentPost.links && parentPost.links.length > 0}
                    <div class="mt-2 text-xs text-blue-600 break-all">
                      <a
                        href={parentPost.links[0].url}
                        target="_blank"
                        rel="noopener noreferrer"
                        class="underline"
                      >
                        {parentPost.links[0].url}
                      </a>
                    </div>
                  {/if}
                </div>
              </div>
            </div>
          {/if}

          {#if !parentPost && sectionName}
            <div class="inline-flex items-center gap-2 text-gray-500 min-w-0">
              <span class={sectionPillClass}>
                {#if sectionIcon}
                  <span class={sectionPillIconClass} aria-hidden="true">{sectionIcon}</span>
                {/if}
                <span class={sectionPillTextClass}>{sectionName}</span>
              </span>
            </div>
          {/if}

          <div class="flex items-start gap-3">
            {#if comment.user?.id}
              <a
                href={buildProfileHref(comment.user.id)}
                class="flex-shrink-0"
                on:click={(event) => handleProfileNavigation(event, comment.user?.id)}
                aria-label={`View ${(comment.user?.username ?? 'user')}'s profile`}
              >
                {#if comment.user?.profilePictureUrl}
                  <img
                    src={comment.user.profilePictureUrl}
                    alt={comment.user.username}
                    class="w-9 h-9 rounded-full object-cover"
                  />
                {:else}
                  <div class="w-9 h-9 rounded-full bg-gray-200 flex items-center justify-center">
                    <span class="text-gray-500 text-sm font-medium">
                      {comment.user?.username?.charAt(0).toUpperCase() || '?'}
                    </span>
                  </div>
                {/if}
              </a>
            {:else}
              {#if comment.user?.profilePictureUrl}
                <img
                  src={comment.user.profilePictureUrl}
                  alt={comment.user.username}
                  class="w-9 h-9 rounded-full object-cover flex-shrink-0"
                />
              {:else}
                <div class="w-9 h-9 rounded-full bg-gray-200 flex items-center justify-center flex-shrink-0">
                  <span class="text-gray-500 text-sm font-medium">
                    {comment.user?.username?.charAt(0).toUpperCase() || '?'}
                  </span>
                </div>
              {/if}
            {/if}

            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 mb-1">
                {#if comment.user?.id}
                  <a
                    href={buildProfileHref(comment.user.id)}
                    class="font-medium text-gray-900 truncate hover:underline"
                    on:click={(event) => handleProfileNavigation(event, comment.user?.id)}
                  >
                    {comment.user?.username || 'Unknown'}
                  </a>
                {:else}
                  <span class="font-medium text-gray-900 truncate">
                    {comment.user?.username || 'Unknown'}
                  </span>
                {/if}
                <span class="text-gray-400 text-sm">commented</span>
                <RelativeTime dateString={comment.createdAt} className="text-gray-500 text-sm" />
                <EditedBadge createdAt={comment.createdAt} updatedAt={comment.updatedAt} />
              </div>

              <LinkifiedText
                text={comment.content}
                highlightQuery={normalizedQuery}
                className="text-gray-800 whitespace-pre-wrap break-words"
                linkClassName="text-blue-600 hover:text-blue-800 underline"
              />

              {#if comment.links && comment.links.length > 0}
                <div class="mt-2 text-sm text-blue-600 break-all">
                  <a
                    href={comment.links[0].url}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="underline"
                  >
                    {comment.links[0].url}
                  </a>
                </div>
              {/if}

              <div class="mt-3">
                <ReactionBar
                  reactionCounts={comment.reactionCounts ?? {}}
                  userReactions={new Set(comment.viewerReactions ?? [])}
                  onToggle={(emoji) => toggleCommentReaction(comment, emoji)}
                  commentId={comment.id}
                />
              </div>
            </div>
          </div>
        </article>
      {/if}
      {/each}
    {/if}
  {/if}
</section>
