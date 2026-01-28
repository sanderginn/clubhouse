<script lang="ts">
  import type { Post } from '../stores/postStore';
  import { postStore } from '../stores/postStore';
  import { api } from '../services/api';
  import CommentThread from './comments/CommentThread.svelte';
  import ReactionBar from './reactions/ReactionBar.svelte';
  import { buildProfileHref, handleProfileNavigation } from '../services/profileNavigation';
  import LinkifiedText from './LinkifiedText.svelte';

  export let post: Post;

  $: userReactions = new Set(post.viewerReactions ?? []);

  async function toggleReaction(emoji: string) {
    const hasReacted = userReactions.has(emoji);
    // Optimistic update
    postStore.toggleReaction(post.id, emoji);

    try {
      if (hasReacted) {
        await api.removePostReaction(post.id, emoji);
      } else {
        await api.addPostReaction(post.id, emoji);
      }
    } catch (e) {
      console.error('Failed to toggle reaction:', e);
      // Revert on error
      postStore.toggleReaction(post.id, emoji);
    }
  }

  function formatDate(dateString: string): string {
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

  function getProviderIcon(provider: string | undefined): string {
    switch (provider) {
      case 'spotify':
        return '🎵';
      case 'youtube':
        return '▶️';
      case 'soundcloud':
        return '☁️';
      case 'imdb':
      case 'rottentomatoes':
        return '🎬';
      case 'goodreads':
        return '📚';
      case 'eventbrite':
      case 'ra':
        return '📅';
      default:
        return '🔗';
    }
  }

  $: link = post.links?.[0];
  $: metadata = link?.metadata;
</script>

<article class="bg-white rounded-lg shadow-sm border border-gray-200 p-4 hover:shadow-md transition-shadow">
  <div class="flex items-start gap-3">
    {#if post.user?.id}
      <a
        href={buildProfileHref(post.user.id)}
        on:click={(event) => handleProfileNavigation(event, post.user?.id)}
        class="flex-shrink-0"
        aria-label={`View ${(post.user?.username ?? 'user')}'s profile`}
      >
        {#if post.user?.profilePictureUrl}
          <img
            src={post.user.profilePictureUrl}
            alt={post.user.username}
            class="w-10 h-10 rounded-full object-cover"
          />
        {:else}
          <div class="w-10 h-10 rounded-full bg-gray-200 flex items-center justify-center">
            <span class="text-gray-500 text-sm font-medium">
              {post.user?.username?.charAt(0).toUpperCase() || '?'}
            </span>
          </div>
        {/if}
      </a>
    {:else}
      {#if post.user?.profilePictureUrl}
        <img
          src={post.user.profilePictureUrl}
          alt={post.user.username}
          class="w-10 h-10 rounded-full object-cover flex-shrink-0"
        />
      {:else}
        <div class="w-10 h-10 rounded-full bg-gray-200 flex items-center justify-center flex-shrink-0">
          <span class="text-gray-500 text-sm font-medium">
            {post.user?.username?.charAt(0).toUpperCase() || '?'}
          </span>
        </div>
      {/if}
    {/if}

    <div class="flex-1 min-w-0">
      <div class="flex items-center gap-2 mb-1">
        {#if post.user?.id}
          <a
            href={buildProfileHref(post.user.id)}
            class="font-medium text-gray-900 truncate hover:underline"
            on:click={(event) => handleProfileNavigation(event, post.user?.id)}
          >
            {post.user?.username || 'Unknown'}
          </a>
        {:else}
          <span class="font-medium text-gray-900 truncate">
            {post.user?.username || 'Unknown'}
          </span>
        {/if}
        <span class="text-gray-400 text-sm">·</span>
        <time class="text-gray-500 text-sm" datetime={post.createdAt}>
          {formatDate(post.createdAt)}
        </time>
      </div>

      <LinkifiedText text={post.content} className="text-gray-800 whitespace-pre-wrap break-words mb-3" />

      {#if link && metadata}
        <a
          href={link.url}
          target="_blank"
          rel="noopener noreferrer"
          class="block rounded-lg border border-gray-200 overflow-hidden hover:border-gray-300 transition-colors"
        >
          <div class="flex">
            {#if metadata.image}
              <div class="w-24 h-24 flex-shrink-0">
                <img
                  src={metadata.image}
                  alt={metadata.title || 'Link preview'}
                  class="w-full h-full object-cover"
                />
              </div>
            {/if}
            <div class="flex-1 p-3 min-w-0">
              <div class="flex items-center gap-1 mb-1">
                <span>{getProviderIcon(metadata.provider)}</span>
                {#if metadata.provider}
                  <span class="text-xs text-gray-500 capitalize">{metadata.provider}</span>
                {/if}
              </div>
              {#if metadata.title}
                <h4 class="font-medium text-gray-900 text-sm truncate">
                  {metadata.title}
                </h4>
              {/if}
              {#if metadata.description}
                <p class="text-gray-600 text-xs line-clamp-2 mt-0.5">
                  {metadata.description}
                </p>
              {/if}
              {#if metadata.author}
                <p class="text-gray-500 text-xs mt-1">
                  by {metadata.author}
                </p>
              {/if}
            </div>
          </div>
        </a>
      {:else if link}
        <a
          href={link.url}
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 text-sm break-all"
        >
          <span>🔗</span>
          <span class="underline">{link.url}</span>
        </a>
      {/if}

      <div class="flex items-center gap-4 mt-3 text-gray-500 text-sm">
        <div class="flex items-center gap-1">
          <span>💬</span>
          <span>{post.commentCount || 0}</span>
        </div>
      </div>

      <div class="mt-3">
        <ReactionBar
          reactionCounts={post.reactionCounts ?? {}}
          userReactions={userReactions}
          onToggle={toggleReaction}
        />
      </div>

      <div class="mt-4 border-t border-gray-200 pt-4">
        <CommentThread postId={post.id} commentCount={post.commentCount ?? 0} />
      </div>
    </div>
  </div>
</article>
