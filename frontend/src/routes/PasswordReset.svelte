<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api } from '../services/api';

  export let token: string | null = null;
  export let onNavigate: (page: 'login' | 'register') => void;

  let password = '';
  let confirmPassword = '';
  let error = '';
  let success = '';
  let isLoading = false;
  let redirectTimer: ReturnType<typeof setTimeout> | null = null;

  let resolvedToken = '';
  if (token !== null) {
    resolvedToken = token;
  } else if (typeof window !== 'undefined') {
    resolvedToken = new URLSearchParams(window.location.search).get('token') ?? '';
  }

  async function handleSubmit() {
    error = '';
    success = '';

    const currentToken = token ?? resolvedToken;

    if (!currentToken) {
      error = 'Reset token is required';
      return;
    }

    if (!password) {
      error = 'Password is required';
      return;
    }

    if (password.length < 12) {
      error = 'Password must be at least 12 characters';
      return;
    }

    if (!confirmPassword) {
      error = 'Please confirm your password';
      return;
    }

    if (password !== confirmPassword) {
      error = 'Passwords do not match';
      return;
    }

    isLoading = true;

    try {
      const response = await api.post<{ message: string }>('/auth/password-reset/redeem', {
        token: currentToken,
        new_password: password,
      });
      success = response.message || 'Password reset successful';
      password = '';
      confirmPassword = '';
      redirectTimer = setTimeout(() => onNavigate('login'), 1500);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Password reset failed';
    } finally {
      isLoading = false;
    }
  }

  onDestroy(() => {
    if (redirectTimer) {
      clearTimeout(redirectTimer);
    }
  });
</script>

<div class="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
  <div class="max-w-md w-full space-y-8">
    <div>
      <h2 class="mt-6 text-center text-3xl font-extrabold text-gray-900">Reset your password</h2>
      <p class="mt-2 text-center text-sm text-gray-600">
        Remembered your password?
        <button
          type="button"
          on:click={() => onNavigate('login')}
          class="font-medium text-indigo-600 hover:text-indigo-500"
        >
          Sign in
        </button>
      </p>
    </div>

    <form class="mt-8 space-y-6" novalidate on:submit|preventDefault={handleSubmit}>
      {#if error}
        <div class="rounded-md bg-red-50 p-4">
          <p class="text-sm text-red-700">{error}</p>
        </div>
      {/if}

      {#if success}
        <div class="rounded-md bg-green-50 p-4">
          <p class="text-sm text-green-700">{success} Redirecting to sign in…</p>
        </div>
      {/if}

      <div class="rounded-md shadow-sm -space-y-px">
        <div>
          <label for="password" class="sr-only">New password</label>
          <input
            id="password"
            name="password"
            type="password"
            required
            bind:value={password}
            aria-describedby="password-help"
            class="appearance-none rounded-none relative block w-full px-3 py-2 border border-gray-300 placeholder-gray-500 text-gray-900 rounded-t-md focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 focus:z-10 sm:text-sm"
            placeholder="New password"
          />
          <p id="password-help" class="px-3 py-2 text-xs text-gray-500">
            Must be at least 12 characters.
          </p>
        </div>
        <div>
          <label for="confirmPassword" class="sr-only">Confirm new password</label>
          <input
            id="confirmPassword"
            name="confirmPassword"
            type="password"
            required
            bind:value={confirmPassword}
            class="appearance-none rounded-none relative block w-full px-3 py-2 border border-gray-300 placeholder-gray-500 text-gray-900 rounded-b-md focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 focus:z-10 sm:text-sm"
            placeholder="Confirm new password"
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
            Resetting password...
          {:else}
            Reset password
          {/if}
        </button>
      </div>
    </form>
  </div>
</div>
