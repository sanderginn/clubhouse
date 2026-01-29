<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api } from '../services/api';
  import { mapApiPost, type ApiPost } from '../stores/postMapper';
  import { mapApiComment, type ApiComment } from '../stores/commentMapper';
  import type { Post } from '../stores/postStore';
  import type { Comment } from '../stores/commentStore';
  import PostCard from './PostCard.svelte';
  import ReactionBar from './reactions/ReactionBar.svelte';
  import LinkifiedText from './LinkifiedText.svelte';
  import { buildProfileHref, handleProfileNavigation, returnToFeed } from '../services/profileNavigation';

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

  let activeTab: 'posts' | 'comments' = 'posts';
  let currentController: AbortController | null = null;
  let postsController: AbortController | null = null;
  let commentsController: AbortController | null = null;
  let postsRequestId = 0;
  let commentsRequestId = 0;
  let pendingCommentReactions = new Set<string>();

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
    resetProfile();
    void loadProfile(userId);
    void loadPosts(userId, true);
  }

  $: if (userId && activeTab === 'comments' && !commentsLoaded && !commentsLoading) {
    void loadComments(userId, true);
  }

  function resetProfile() {
    currentController?.abort();
    postsController?.abort();
    commentsController?.abort();
    postsRequestId += 1;
    commentsRequestId += 1;
    pendingCommentReactions = new Set();
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
    activeTab = 'posts';
  }

  async function loadProfile(id: string) {
    currentController?.abort();
    const controller = new AbortController();
    currentController = controller;
    isLoadingProfile = true;
    profileError = null;

    try {
      const response = await api.get<ApiProfileResponse>(`/users/${id}`, { signal: controller.signal });

      if (currentController !== controller || id !== userId) return;

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
      if (currentController !== controller || id !== userId) return;
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

      if (requestId !== postsRequestId || id !== userId) {
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
      if (requestId !== postsRequestId || id !== userId) {
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

      if (requestId !== commentsRequestId || id !== userId) {
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
      if (requestId !== commentsRequestId || id !== userId) {
        return;
      }
      commentsError = error instanceof Error ? error.message : 'Failed to load comments.';
    } finally {
      if (requestId === commentsRequestId) {
        commentsLoading = false;
      }
    }
  }

  function formatDate(dateString?: string | null): string {
    if (!dateString) return 'Unknown date';
    const date = new Date(dateString);
    return date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
  }

  function formatRelative(dateString: string): string {
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    if (diffMins < 1) return 'just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;

    return date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: date.getFullYear() !== now.getFullYear() ? 'numeric' : undefined,
    });
  }

  async function toggleCommentReaction(comment: Comment, emoji: string) {
    const key = `${comment.id}-${emoji}`;
    if (pendingCommentReactions.has(key)) return;
    pendingCommentReactions.add(key);

    const userReactions = new Set(comment.viewerReactions ?? []);
    const hasReacted = userReactions.has(emoji);

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
      comment.reactionCounts = {
        ...(comment.reactionCounts ?? {}),
        [emoji]: (comment.reactionCounts?.[emoji] ?? 0) + 1,
      };
    }
    comments = [...comments];

    try {
      if (hasReacted) {
        await api.removeCommentReaction(comment.id, emoji);
      } else {
        await api.addCommentReaction(comment.id, emoji);
      }
    } catch (error) {
      if (hasReacted) {
        comment.viewerReactions = [...(comment.viewerReactions ?? []), emoji];
        comment.reactionCounts = {
          ...(comment.reactionCounts ?? {}),
          [emoji]: (comment.reactionCounts?.[emoji] ?? 0) + 1,
        };
      } else {
        comment.viewerReactions = (comment.viewerReactions ?? []).filter((e) => e !== emoji);
        if (comment.reactionCounts && comment.reactionCounts[emoji]) {
          comment.reactionCounts[emoji]--;
          if (comment.reactionCounts[emoji] <= 0) {
            delete comment.reactionCounts[emoji];
          }
        }
      }
      comments = [...comments];
    } finally {
      pendingCommentReactions.delete(key);
    }
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
          resetProfile();
          void loadProfile(userId);
          void loadPosts(userId, true);
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
                on:click={() => userId && loadPosts(userId, true)}
                class="mt-2 text-xs text-red-700 underline hover:no-underline"
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
                on:click={() => userId && loadPosts(userId, false)}
                class="text-sm text-gray-600 hover:text-gray-900"
                disabled={postsLoading}
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
                on:click={() => userId && loadComments(userId, true)}
                class="mt-2 text-xs text-red-700 underline hover:no-underline"
              >
                Try again
              </button>
            </div>
          {:else if comments.length === 0}
            <p class="text-gray-500 text-sm">No comments yet.</p>
          {:else}
            {#each comments as comment (comment.id)}
              <article class="bg-white rounded-lg border border-gray-200 p-4">
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
                      <time class="text-gray-500 text-sm" datetime={comment.createdAt}>
                        {formatRelative(comment.createdAt)}
                      </time>
                    </div>

                    <LinkifiedText
                      text={comment.content}
                      className="text-gray-800 whitespace-pre-wrap break-words text-sm"
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
            {/each}
          {/if}

          {#if commentsHasMore && comments.length > 0}
            <div class="flex justify-center">
              <button
                type="button"
                on:click={() => userId && loadComments(userId, false)}
                class="text-sm text-gray-600 hover:text-gray-900"
                disabled={commentsLoading}
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
