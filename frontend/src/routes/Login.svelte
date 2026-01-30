<script lang="ts">
  import { api } from '../services/api';
  import { authStore, type User, uiStore } from '../stores';
  import { buildSettingsHref, replacePath } from '../services/routeNavigation';

  let username = '';
  let password = '';
  let totpCode = '';
  let error = '';
  let errorCode: string | null = null;
  let isLoading = false;
  let needsTotp = false;
  let totpAttempted = false;
  let mfaSetupRequired = false;

  interface LoginResponse {
    id: string;
    username: string;
    email?: string | null;
    is_admin: boolean;
    totp_enabled: boolean;
    message: string;
  }

  interface TotpChallengeResponse {
    mfa_required: true;
    challenge_id?: string;
    message: string;
  }

  const isTotpChallenge = (
    value: LoginResponse | TotpChallengeResponse
  ): value is TotpChallengeResponse => {
    return (value as TotpChallengeResponse).mfa_required === true;
  };

  function clearError() {
    if (error) {
      error = '';
    }
    if (errorCode) {
      errorCode = null;
    }
    if (mfaSetupRequired) {
      mfaSetupRequired = false;
    }
    if (totpAttempted) {
      totpAttempted = false;
    }
  }

  async function handleSubmit() {
    error = '';
    errorCode = null;
    mfaSetupRequired = false;
    const trimmedUsername = username.trim();
    const trimmedTotp = totpCode.replace(/\s+/g, '');

    if (!trimmedUsername || !password) {
      error = 'Username and password are required';
      return;
    }

    if (needsTotp && !trimmedTotp) {
      totpAttempted = true;
      error = 'Authentication code is required';
      return;
    }

    isLoading = true;

    try {
      const response = await api.post<LoginResponse | TotpChallengeResponse>('/auth/login', {
        username: trimmedUsername,
        password,
        totp_code: needsTotp ? trimmedTotp : undefined,
      });

      if (isTotpChallenge(response)) {
        needsTotp = true;
        totpAttempted = false;
        error = '';
        authStore.setMfaChallenge({
          username: trimmedUsername,
          challengeId: response.challenge_id,
        });
      } else {
        const user: User = {
          id: response.id,
          username: response.username,
          email: response.email ?? '',
          isAdmin: response.is_admin,
          totpEnabled: response.totp_enabled,
        };
        authStore.setUser(user);
        totpAttempted = false;
      }
    } catch (e) {
      const errorWithCode = e as Error & { code?: string; mfaRequired?: boolean };
      errorCode = errorWithCode.code ?? (errorWithCode.mfaRequired ? 'MFA_SETUP_REQUIRED' : null);
      if (errorWithCode.code === 'TOTP_REQUIRED') {
        needsTotp = true;
        totpAttempted = false;
        authStore.setMfaChallenge({ username: trimmedUsername });
        error = '';
      } else if (errorWithCode.code === 'MFA_SETUP_REQUIRED' || errorWithCode.mfaRequired) {
        needsTotp = false;
        totpAttempted = false;
        const hasSession = await authStore.checkSession();
        if (hasSession) {
          uiStore.setActiveView('settings');
          replacePath(buildSettingsHref());
          return;
        }
        mfaSetupRequired = true;
        error = errorWithCode.message || 'Multi-factor authentication is required to continue.';
      } else if (errorWithCode.code === 'USER_NOT_APPROVED') {
        needsTotp = false;
        totpAttempted = false;
        error = errorWithCode.message || 'Your account is awaiting admin approval.';
      } else {
        if (needsTotp) {
          totpAttempted = true;
        }
        error = e instanceof Error ? e.message : 'Login failed';
      }
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
        <div
          class={`rounded-md p-4 ${
            errorCode === 'USER_NOT_APPROVED'
              ? 'bg-blue-50 text-blue-700'
              : errorCode === 'MFA_SETUP_REQUIRED'
                ? 'bg-amber-50 text-amber-700'
                : 'bg-red-50 text-red-700'
          }`}
        >
          <p class="text-sm">{error}</p>
          {#if mfaSetupRequired}
            <p class="mt-2 text-xs text-amber-700/80">
              Finish enrollment in your account settings to regain access.
            </p>
          {/if}
        </div>
      {/if}

      {#if needsTotp}
        <div class="rounded-md shadow-sm -space-y-px">
          <div>
            <label for="totp" class="sr-only">Authentication code</label>
            <input
              id="totp"
              name="totp"
              type="text"
              inputmode="numeric"
              autocomplete="one-time-code"
              required
              bind:value={totpCode}
              on:input={clearError}
              class={`appearance-none rounded-md relative block w-full px-3 py-2 border placeholder-gray-500 text-gray-900 focus:outline-none focus:z-10 sm:text-sm ${
                needsTotp && totpAttempted && error
                  ? 'border-red-300 focus:border-red-500 focus:ring-red-500'
                  : 'border-gray-300 focus:border-indigo-500 focus:ring-indigo-500'
              }`}
              placeholder="6-digit authentication code"
            />
          </div>
        </div>
        <p class="text-sm text-gray-500">
          Enter the 6-digit code from your authenticator app to finish signing in.
        </p>
      {:else}
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
              on:input={clearError}
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
              on:input={clearError}
              class="appearance-none rounded-none relative block w-full px-3 py-2 border border-gray-300 placeholder-gray-500 text-gray-900 rounded-b-md focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 focus:z-10 sm:text-sm"
              placeholder="Password"
            />
          </div>
        </div>
      {/if}

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
            {needsTotp ? 'Verifying...' : 'Signing in...'}
          {:else}
            {needsTotp ? 'Verify code' : 'Sign in'}
          {/if}
        </button>
      </div>
    </form>
  </div>
</div>
