<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api } from '../services/api';
  import { mapApiPost, type ApiPost } from '../stores/postMapper';
  import { mapApiComment, type ApiComment } from '../stores/commentMapper';
  import type { Post } from '../stores/postStore';
  import type { Comment } from '../stores/commentStore';
  import PostCard from './PostCard.svelte';
  import { returnToFeed } from '../services/profileNavigation';
  import { buildThreadHref } from '../services/routeNavigation';
  import { displayTimezone } from '../stores';
  import { sections } from '../stores/sectionStore';
  import { getSectionSlugById } from '../services/sectionSlug';
  import { formatInTimezone } from '../lib/time';

  export let userId: string | null;

  interface ApiProfileUser {
    id: string;
    username: string;
    profilePictureUrl?: string | null;
    profile_picture_url?: string | null;
    createdAt?: string | null;
    created_at?: string | null;
  }

  interface ApiProfileStats {
    postCount?: number;
    commentCount?: number;
    post_count?: number;
    comment_count?: number;
  }

  type ApiProfileResponse =
    | { user: ApiProfileUser; stats?: ApiProfileStats }
    | (ApiProfileUser & { stats?: ApiProfileStats });

  interface ProfileUser {
    id: string;
    username: string;
    profilePictureUrl?: string | null;
    createdAt?: string | null;
  }

  let profile: ProfileUser | null = null;
  let stats: { postCount: number; commentCount: number } | null = null;
  let isLoadingProfile = false;
  let profileError: string | null = null;
  let resolvedUserId: string | null = null;
  let resolveRequestId = 0;

  let posts: Post[] = [];
  let postsCursor: string | null = null;
  let postsHasMore = true;
  let postsLoading = false;
  let postsError: string | null = null;

  let comments: Comment[] = [];
  let commentsCursor: string | null = null;
  let commentsHasMore = true;
  let commentsLoading = false;
  let commentsError: string | null = null;
  let commentsLoaded = false;
  let postContext: Record<string, Post | null> = {};
  let postContextErrors: Record<string, string | null> = {};
  let postContextLoading = new Set<string>();
  let commentThreads: { postId: string; commentIds: string[] }[] = [];

  let activeTab: 'posts' | 'comments' = 'posts';
  let currentController: AbortController | null = null;
  let postsController: AbortController | null = null;
  let commentsController: AbortController | null = null;
  let postsRequestId = 0;
  let commentsRequestId = 0;
  let contextRequestId = 0;

  function normalizeProfileResponse(
    response: ApiProfileResponse
  ): { user: ApiProfileUser; stats: ApiProfileStats } {
    const hasUserField = typeof response === 'object' && response !== null && 'user' in response;
    const apiUser = (hasUserField ? response.user : response) as ApiProfileUser | undefined;
    if (!apiUser?.id) {
      throw new Error('Profile data is missing.');
    }
    const apiStats = (hasUserField
      ? (response as { stats?: ApiProfileStats }).stats
      : (response as { stats?: ApiProfileStats }).stats) ?? {};
    return { user: apiUser, stats: apiStats };
  }

  $: if (userId) {
    void resolveProfileTarget(userId);
  }

  $: if (resolvedUserId && activeTab === 'comments' && !commentsLoaded && !commentsLoading) {
    void loadComments(resolvedUserId, true);
  }

  function resetProfile() {
    currentController?.abort();
    postsController?.abort();
    commentsController?.abort();
    postsRequestId += 1;
    commentsRequestId += 1;
    contextRequestId += 1;
    profile = null;
    stats = null;
    profileError = null;
    isLoadingProfile = false;
    posts = [];
    postsCursor = null;
    postsHasMore = true;
    postsLoading = false;
    postsError = null;
    comments = [];
    commentsCursor = null;
    commentsHasMore = true;
    commentsLoading = false;
    commentsError = null;
    commentsLoaded = false;
    postContext = {};
    postContextErrors = {};
    postContextLoading = new Set();
    commentThreads = [];
    activeTab = 'posts';
  }

  function isUuid(value: string): boolean {
    return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(
      value
    );
  }

  async function resolveProfileTarget(identifier: string) {
    const requestId = (resolveRequestId += 1);
    resetProfile();
    resolvedUserId = null;
    profileError = null;

    try {
      if (isUuid(identifier)) {
        resolvedUserId = identifier;
      } else {
        const response = await api.lookupUserByUsername(identifier);
        resolvedUserId = response.user.id;
      }
    } catch (error) {
      if (requestId !== resolveRequestId) return;
      profileError = error instanceof Error ? error.message : 'User not found.';
      return;
    }

    if (requestId !== resolveRequestId || !resolvedUserId) return;
    void loadProfile(resolvedUserId);
    void loadPosts(resolvedUserId, true);
  }

  async function loadProfile(id: string) {
    currentController?.abort();
    const controller = new AbortController();
    currentController = controller;
    isLoadingProfile = true;
    profileError = null;

    try {
      const response = await api.get<ApiProfileResponse>(`/users/${id}`, { signal: controller.signal });

      if (currentController !== controller || id !== resolvedUserId) return;

      const { user: apiUser, stats: apiStats } = normalizeProfileResponse(response);
      profile = {
        id: apiUser.id,
        username: apiUser.username,
        profilePictureUrl: apiUser.profilePictureUrl ?? apiUser.profile_picture_url ?? null,
        createdAt: apiUser.createdAt ?? apiUser.created_at ?? null,
      };

      stats = {
        postCount: apiStats.postCount ?? apiStats.post_count ?? 0,
        commentCount: apiStats.commentCount ?? apiStats.comment_count ?? 0,
      };
    } catch (error) {
      if (currentController !== controller || id !== resolvedUserId) return;
      profileError = error instanceof Error ? error.message : 'Failed to load profile.';
    } finally {
      if (currentController === controller) {
        isLoadingProfile = false;
      }
    }
  }

  async function loadPosts(id: string, reset: boolean) {
    if (postsLoading) return;
    if (!postsHasMore && !reset) return;

    postsController?.abort();
    const controller = new AbortController();
    postsController = controller;
    const requestId = (postsRequestId += 1);
    postsLoading = true;
    postsError = null;

    try {
      const params = new URLSearchParams({ limit: '20' });
      if (!reset && postsCursor) {
        params.set('cursor', postsCursor);
      }
      const response = await api.get<{
        posts: ApiPost[];
        meta?: { cursor?: string; hasMore?: boolean; has_more?: boolean };
        next_cursor?: string;
        has_more?: boolean;
      }>(`/users/${id}/posts?${params.toString()}`, { signal: controller.signal });

      if (requestId !== postsRequestId || id !== resolvedUserId) {
        return;
      }

      const nextPosts = (response.posts ?? []).map(mapApiPost);
      const meta = response.meta ?? {};
      const nextCursor = meta.cursor ?? response.next_cursor ?? null;
      const nextHasMore = meta.hasMore ?? meta.has_more ?? response.has_more ?? false;

      posts = reset ? nextPosts : [...posts, ...nextPosts];
      postsCursor = nextCursor;
      postsHasMore = nextHasMore;
    } catch (error) {
      if (requestId !== postsRequestId || id !== resolvedUserId) {
        return;
      }
      postsError = error instanceof Error ? error.message : 'Failed to load posts.';
    } finally {
      if (requestId === postsRequestId) {
        postsLoading = false;
      }
    }
  }

  async function loadComments(id: string, reset: boolean) {
    if (commentsLoading) return;
    if (!commentsHasMore && !reset) return;

    commentsController?.abort();
    const controller = new AbortController();
    commentsController = controller;
    const requestId = (commentsRequestId += 1);
    commentsLoading = true;
    commentsError = null;

    try {
      const params = new URLSearchParams({ limit: '20' });
      if (!reset && commentsCursor) {
        params.set('cursor', commentsCursor);
      }
      const response = await api.get<{
        comments: ApiComment[];
        meta?: { cursor?: string; hasMore?: boolean; has_more?: boolean };
        next_cursor?: string;
        has_more?: boolean;
      }>(`/users/${id}/comments?${params.toString()}`, { signal: controller.signal });

      if (requestId !== commentsRequestId || id !== resolvedUserId) {
        return;
      }

      const nextComments = (response.comments ?? []).map(mapApiComment);
      const meta = response.meta ?? {};
      const nextCursor = meta.cursor ?? response.next_cursor ?? null;
      const nextHasMore = meta.hasMore ?? meta.has_more ?? response.has_more ?? false;

      comments = reset ? nextComments : [...comments, ...nextComments];
      commentsCursor = nextCursor;
      commentsHasMore = nextHasMore;
      commentsLoaded = true;
    } catch (error) {
      if (requestId !== commentsRequestId || id !== resolvedUserId) {
        return;
      }
      commentsError = error instanceof Error ? error.message : 'Failed to load comments.';
    } finally {
      if (requestId === commentsRequestId) {
        commentsLoading = false;
      }
    }
  }

  $: if (activeTab === 'comments' && comments.length > 0) {
    for (const comment of comments) {
      void ensurePostContext(comment.postId);
    }
  }

  $: commentThreads = groupCommentThreads(comments);

  async function ensurePostContext(postId: string) {
    if (!postId || postContext[postId] !== undefined || postContextLoading.has(postId)) {
      return;
    }
    const requestId = contextRequestId;
    const activeUserId = userId;
    const isCurrentRequest = () => requestId === contextRequestId && activeUserId === userId;
    postContextLoading = new Set(postContextLoading).add(postId);
    postContextErrors = { ...postContextErrors, [postId]: null };
    try {
      const response = await api.get<{ post?: ApiPost | null }>(`/posts/${postId}`);
      if (!isCurrentRequest()) {
        return;
      }
      const post = response?.post ? mapApiPost(response.post) : null;
      postContext = { ...postContext, [postId]: post };
    } catch (error) {
      if (!isCurrentRequest()) {
        return;
      }
      postContext = { ...postContext, [postId]: null };
      postContextErrors = {
        ...postContextErrors,
        [postId]: error instanceof Error ? error.message : 'Failed to load post context.',
      };
    } finally {
      if (isCurrentRequest()) {
        const nextLoading = new Set(postContextLoading);
        nextLoading.delete(postId);
        postContextLoading = nextLoading;
      }
    }
  }

  function groupCommentThreads(items: Comment[]): { postId: string; commentIds: string[] }[] {
    const groups: { postId: string; commentIds: string[] }[] = [];
    const indexByPost = new Map<string, number>();

    for (const comment of items) {
      if (!comment.postId) continue;
      let index = indexByPost.get(comment.postId);
      if (index === undefined) {
        index = groups.length;
        indexByPost.set(comment.postId, index);
        groups.push({ postId: comment.postId, commentIds: [] });
      }

      const group = groups[index];
      if (!group.commentIds.includes(comment.id)) {
        group.commentIds = [...group.commentIds, comment.id];
      }
    }

    return groups;
  }

  function buildThreadLink(post: Post): string {
    const sectionSlug = getSectionSlugById($sections, post.sectionId) ?? post.sectionId;
    return buildThreadHref(sectionSlug, post.id);
  }

  function buildThreadLinkForPost(post?: Post | null): string {
    if (!post) return '#';
    return buildThreadLink(post);
  }

  function formatDate(dateString?: string | null): string {
    if (!dateString) return 'Unknown date';
    const date = new Date(dateString);
    return formatInTimezone(
      date,
      {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
      },
      $displayTimezone
    );
  }

  onDestroy(() => {
    currentController?.abort();
    postsController?.abort();
    commentsController?.abort();
  });
</script>

<div class="space-y-6">
  <div class="flex items-center gap-3">
    <button
      type="button"
      on:click={returnToFeed}
      class="text-sm text-gray-500 hover:text-gray-800 flex items-center gap-2"
    >
      <span aria-hidden="true">&larr;</span>
      Back to feed
    </button>
  </div>

  {#if !userId}
    <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
      <p class="text-gray-600">Select a user to view their profile.</p>
    </div>
  {:else if isLoadingProfile}
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
        <span>Loading profile...</span>
      </div>
    </div>
  {:else if profileError}
    <div class="bg-red-50 border border-red-200 rounded-lg p-4 text-center">
      <p class="text-red-600">{profileError}</p>
      <button
        on:click={() => {
          if (!userId) return;
          void resolveProfileTarget(userId);
        }}
        class="mt-2 text-sm text-red-700 underline hover:no-underline"
      >
        Try again
      </button>
    </div>
  {:else if profile}
    <section class="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
      <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div class="flex items-center gap-4">
          {#if profile.profilePictureUrl}
            <img
              src={profile.profilePictureUrl}
              alt={profile.username}
              class="w-16 h-16 rounded-full object-cover"
            />
          {:else}
            <div class="w-16 h-16 rounded-full bg-gray-200 flex items-center justify-center">
              <span class="text-gray-600 text-2xl font-semibold">
                {profile.username.charAt(0).toUpperCase()}
              </span>
            </div>
          {/if}
          <div>
            <h1 class="text-2xl font-bold text-gray-900">{profile.username}</h1>
            <p class="text-sm text-gray-500">Member since {formatDate(profile.createdAt)}</p>
          </div>
        </div>
        {#if stats}
          <div class="flex gap-6 text-sm text-gray-600">
            <div class="text-center">
              <div class="text-lg font-semibold text-gray-900">{stats.postCount}</div>
              <div>Posts</div>
            </div>
            <div class="text-center">
              <div class="text-lg font-semibold text-gray-900">{stats.commentCount}</div>
              <div>Comments</div>
            </div>
          </div>
        {/if}
      </div>

    </section>

    <section class="bg-white rounded-lg shadow-sm border border-gray-200">
      <div class="flex border-b border-gray-200">
        <button
          type="button"
          class={`flex-1 px-4 py-3 text-sm font-medium ${
            activeTab === 'posts' ? 'text-primary border-b-2 border-primary' : 'text-gray-500'
          }`}
          on:click={() => (activeTab = 'posts')}
        >
          Posts
        </button>
        <button
          type="button"
          class={`flex-1 px-4 py-3 text-sm font-medium ${
            activeTab === 'comments' ? 'text-primary border-b-2 border-primary' : 'text-gray-500'
          }`}
          on:click={() => (activeTab = 'comments')}
        >
          Comments
        </button>
      </div>

      <div class="p-4 space-y-4">
        {#if activeTab === 'posts'}
          {#if postsLoading && posts.length === 0}
            <div class="flex items-center gap-2 text-gray-500">
              <svg class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
              <span>Loading posts...</span>
            </div>
          {:else if postsError}
            <div class="bg-red-50 border border-red-200 rounded-lg p-3 text-sm text-red-600">
              <p>{postsError}</p>
              <button
                on:click={() => resolvedUserId && loadPosts(resolvedUserId, true)}
                class="mt-2 text-xs text-red-700 underline hover:no-underline"
                disabled={!resolvedUserId}
              >
                Try again
              </button>
            </div>
          {:else if posts.length === 0}
            <p class="text-gray-500 text-sm">No posts yet.</p>
          {:else}
            {#each posts as post (post.id)}
              <PostCard {post} />
            {/each}
          {/if}

          {#if postsHasMore && posts.length > 0}
            <div class="flex justify-center">
              <button
                type="button"
                on:click={() => resolvedUserId && loadPosts(resolvedUserId, false)}
                class="text-sm text-gray-600 hover:text-gray-900"
                disabled={postsLoading || !resolvedUserId}
              >
                {postsLoading ? 'Loading...' : 'Load more posts'}
              </button>
            </div>
          {/if}
        {:else}
          {#if commentsLoading && comments.length === 0}
            <div class="flex items-center gap-2 text-gray-500">
              <svg class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
              <span>Loading comments...</span>
            </div>
          {:else if commentsError}
            <div class="bg-red-50 border border-red-200 rounded-lg p-3 text-sm text-red-600">
              <p>{commentsError}</p>
              <button
                on:click={() => resolvedUserId && loadComments(resolvedUserId, true)}
                class="mt-2 text-xs text-red-700 underline hover:no-underline"
                disabled={!resolvedUserId}
              >
                Try again
              </button>
            </div>
          {:else if comments.length === 0}
            <p class="text-gray-500 text-sm">No comments yet.</p>
          {:else}
            {#each commentThreads as thread (thread.postId)}
              {@const threadPost = postContext[thread.postId]}
              <div class="space-y-3">
                <div class="flex items-center justify-between text-xs text-gray-500">
                  <span class="font-semibold uppercase tracking-wide text-gray-400">Thread</span>
                  {#if threadPost}
                    <a
                      href={buildThreadLinkForPost(threadPost)}
                      class="text-xs text-blue-600 hover:text-blue-800"
                    >
                      Open full thread ->
                    </a>
                  {/if}
                </div>

                {#if postContextLoading.has(thread.postId)}
                  <div class="text-sm text-gray-500">Loading thread...</div>
                {:else if postContextErrors[thread.postId]}
                  <div class="bg-red-50 border border-red-200 rounded-lg p-3 text-sm text-red-600">
                    {postContextErrors[thread.postId]}
                  </div>
                {:else if threadPost}
                  <PostCard post={threadPost} highlightCommentIds={thread.commentIds} />
                {:else}
                  <div class="bg-amber-50 border border-amber-200 rounded-lg p-3 text-sm text-amber-800">
                    Thread unavailable.
                  </div>
                {/if}
              </div>
            {/each}
          {/if}

          {#if commentsHasMore && comments.length > 0}
            <div class="flex justify-center">
              <button
                type="button"
                on:click={() => resolvedUserId && loadComments(resolvedUserId, false)}
                class="text-sm text-gray-600 hover:text-gray-900"
                disabled={commentsLoading || !resolvedUserId}
              >
                {commentsLoading ? 'Loading...' : 'Load more comments'}
              </button>
            </div>
          {/if}
        {/if}
      </div>
    </section>
  {/if}
</div>

<style>
  .text-primary {
    color: var(--primary, #1f6feb);
  }
  .border-primary {
    border-color: var(--primary, #1f6feb);
  }
</style>
