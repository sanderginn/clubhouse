<script lang="ts">
  import { api } from '../services/api';
  import { authStore, type User } from '../stores';

  let username = '';
  let password = '';
  let error = '';
  let isLoading = false;

  interface LoginResponse {
    id: string;
    username: string;
    email?: string | null;
    is_admin: boolean;
    message: string;
  }

  async function handleSubmit() {
    error = '';
    const trimmedUsername = username.trim();

    if (!trimmedUsername || !password) {
      error = 'Username and password are required';
      return;
    }

    isLoading = true;

    try {
      const response = await api.post<LoginResponse>('/auth/login', {
        username: trimmedUsername,
        password,
      });

      const user: User = {
        id: response.id,
        username: response.username,
        email: response.email ?? '',
        isAdmin: response.is_admin,
      };

      authStore.setUser(user);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Login failed';
    } finally {
      isLoading = false;
    }
  }

  export let onNavigate: (page: 'login' | 'register') => void;
</script>

<div class="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
  <div class="max-w-md w-full space-y-8">
    <div>
      <h2 class="mt-6 text-center text-3xl font-extrabold text-gray-900">Sign in to Clubhouse</h2>
      <p class="mt-2 text-center text-sm text-gray-600">
        Or
        <button
          type="button"
          on:click={() => onNavigate('register')}
          class="font-medium text-indigo-600 hover:text-indigo-500"
        >
          create a new account
        </button>
      </p>
    </div>

    <form class="mt-8 space-y-6" novalidate on:submit|preventDefault={handleSubmit}>
      {#if error}
        <div class="rounded-md bg-red-50 p-4">
          <p class="text-sm text-red-700">{error}</p>
        </div>
      {/if}

      <div class="rounded-md shadow-sm -space-y-px">
        <div>
          <label for="username" class="sr-only">Username</label>
          <input
            id="username"
            name="username"
            type="text"
            autocomplete="username"
            required
            bind:value={username}
            class="appearance-none rounded-none relative block w-full px-3 py-2 border border-gray-300 placeholder-gray-500 text-gray-900 rounded-t-md focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 focus:z-10 sm:text-sm"
            placeholder="Username"
          />
        </div>
        <div>
          <label for="password" class="sr-only">Password</label>
          <input
            id="password"
            name="password"
            type="password"
            autocomplete="current-password"
            required
            bind:value={password}
            class="appearance-none rounded-none relative block w-full px-3 py-2 border border-gray-300 placeholder-gray-500 text-gray-900 rounded-b-md focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 focus:z-10 sm:text-sm"
            placeholder="Password"
          />
        </div>
      </div>

      <div>
        <button
          type="submit"
          disabled={isLoading}
          class="group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {#if isLoading}
            <svg
              class="animate-spin -ml-1 mr-3 h-5 w-5 text-white"
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
            Signing in...
          {:else}
            Sign in
          {/if}
        </button>
      </div>
    </form>
  </div>
</div>
