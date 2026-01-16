<script lang="ts">
  import { onMount } from 'svelte';
  import './styles/globals.css';
  import { Layout, PostForm } from './components';
  import { Login, Register } from './routes';
  import { authStore, isAuthenticated, activeSection, posts } from './stores';

  let authPage: 'login' | 'register' = 'login';

  onMount(() => {
    authStore.checkSession();
  });

  function handleNavigate(page: 'login' | 'register') {
    authPage = page;
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
  {#if authPage === 'login'}
    <Login onNavigate={handleNavigate} />
  {:else}
    <Register onNavigate={handleNavigate} />
  {/if}
{:else}
  <Layout>
    <div class="space-y-6">
      {#if $activeSection}
        <div class="flex items-center gap-3">
          <span class="text-3xl">{$activeSection.icon}</span>
          <h1 class="text-2xl font-bold text-gray-900">{$activeSection.name}</h1>
        </div>

        <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
          <PostForm />
        </div>

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
{/if}
