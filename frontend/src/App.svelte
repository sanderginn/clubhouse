<script lang="ts">
  import './styles/globals.css';
  import { Layout, PostForm } from './components';
  import { activeSection, isAuthenticated, posts } from './stores';
</script>

<Layout>
  <div class="space-y-6">
    {#if $activeSection}
      <div class="flex items-center gap-3">
        <span class="text-3xl">{$activeSection.icon}</span>
        <h1 class="text-2xl font-bold text-gray-900">{$activeSection.name}</h1>
      </div>

      {#if $isAuthenticated}
        <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
          <PostForm />
        </div>
      {/if}

      <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        {#if $posts.length === 0}
          <p class="text-gray-600">
            No posts yet in {$activeSection.name}. Be the first to share something!
          </p>
        {:else}
          <div class="space-y-4">
            {#each $posts as post (post.id)}
              <div class="border-b border-gray-100 pb-4 last:border-0 last:pb-0">
                <p class="text-gray-900">{post.content}</p>
                <p class="text-xs text-gray-500 mt-2">
                  {new Date(post.createdAt).toLocaleString()}
                </p>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {:else}
      <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <h1 class="text-2xl font-bold text-gray-900 mb-4">Welcome to Clubhouse</h1>
        <p class="text-gray-600">Select a section from the sidebar to get started.</p>
      </div>
    {/if}
  </div>
</Layout>
