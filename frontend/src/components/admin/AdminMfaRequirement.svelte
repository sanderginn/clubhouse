<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api } from '../../services/api';

  interface ConfigResponse {
    config?: {
      linkMetadataEnabled?: boolean;
      mfaRequired?: boolean;
      mfa_required?: boolean;
    };
  }

  interface ApprovedUser {
    id: string;
    username: string;
    email: string;
    is_admin: boolean;
    approved_at: string;
    created_at: string;
    totp_enabled?: boolean;
  }

  let isLoading = true;
  let isSaving = false;
  let errorMessage = '';
  let successMessage = '';
  let mfaRequired = false;
  let totalUsers = 0;
  let usersWithoutMfa = 0;
  let configAbortController: AbortController | null = null;
  let usersAbortController: AbortController | null = null;
  let configTimeoutId: ReturnType<typeof setTimeout> | null = null;
  let usersTimeoutId: ReturnType<typeof setTimeout> | null = null;

  const normalizeMfaRequired = (response: ConfigResponse | null): boolean => {
    const config = response?.config;
    if (!config) return false;
    if (typeof config.mfaRequired === 'boolean') return config.mfaRequired;
    if (typeof config.mfa_required === 'boolean') return config.mfa_required;
    return false;
  };

  const clearTimeouts = () => {
    if (configTimeoutId) {
      clearTimeout(configTimeoutId);
      configTimeoutId = null;
    }
    if (usersTimeoutId) {
      clearTimeout(usersTimeoutId);
      usersTimeoutId = null;
    }
  };

  const loadConfig = async () => {
    configAbortController?.abort();
    if (configTimeoutId) {
      clearTimeout(configTimeoutId);
      configTimeoutId = null;
    }

    const controller = typeof AbortController === 'undefined' ? null : new AbortController();
    configAbortController = controller;

    if (controller) {
      configTimeoutId = setTimeout(() => controller.abort(), 10000);
    }

    const response = await api.get<ConfigResponse>(
      '/admin/config',
      controller ? { signal: controller.signal } : undefined
    );
    if (configAbortController === controller) {
      configAbortController = null;
      if (configTimeoutId) {
        clearTimeout(configTimeoutId);
        configTimeoutId = null;
      }
    }

    mfaRequired = normalizeMfaRequired(response);
  };

  const loadUsers = async () => {
    usersAbortController?.abort();
    if (usersTimeoutId) {
      clearTimeout(usersTimeoutId);
      usersTimeoutId = null;
    }

    const controller = typeof AbortController === 'undefined' ? null : new AbortController();
    usersAbortController = controller;

    if (controller) {
      usersTimeoutId = setTimeout(() => controller.abort(), 10000);
    }

    const response = await api.get<ApprovedUser[] | null>(
      '/admin/users/approved',
      controller ? { signal: controller.signal } : undefined
    );

    if (usersAbortController === controller) {
      usersAbortController = null;
      if (usersTimeoutId) {
        clearTimeout(usersTimeoutId);
        usersTimeoutId = null;
      }
    }

    const approvedUsers = Array.isArray(response) ? response : [];
    totalUsers = approvedUsers.length;
    usersWithoutMfa = approvedUsers.filter((user) => !user.totp_enabled).length;
  };

  const refresh = async () => {
    isLoading = true;
    errorMessage = '';
    successMessage = '';
    try {
      await Promise.all([loadConfig(), loadUsers()]);
    } catch (error) {
      if (error instanceof Error && error.name === 'AbortError') {
        errorMessage = 'Request timed out. Please try again.';
      } else {
        errorMessage = error instanceof Error ? error.message : 'Failed to load MFA settings.';
      }
    } finally {
      isLoading = false;
    }
  };

  const confirmEnable = () => {
    if (typeof window === 'undefined') return true;
    return window.confirm(
      'Requiring MFA will block members without MFA until they enroll. Continue?'
    );
  };

  const updateRequirement = async (nextValue: boolean) => {
    if (nextValue && !confirmEnable()) {
      return;
    }

    isSaving = true;
    errorMessage = '';
    successMessage = '';

    try {
      const response = await api.patch<ConfigResponse>('/admin/config', {
        mfa_required: nextValue,
      });
      mfaRequired = normalizeMfaRequired(response);
      successMessage = mfaRequired
        ? 'MFA requirement enabled for all members.'
        : 'MFA requirement disabled.';
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : 'Failed to update MFA requirement.';
    } finally {
      isSaving = false;
    }
  };

  if (typeof window !== 'undefined') {
    void refresh();
  }

  onDestroy(() => {
    configAbortController?.abort();
    usersAbortController?.abort();
    clearTimeouts();
  });
</script>

<section class="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
  <div class="flex flex-wrap items-start justify-between gap-4">
    <div>
      <p class="text-xs uppercase tracking-[0.3em] text-slate-400 font-mono">Policy</p>
      <h2 class="text-2xl font-serif font-semibold text-slate-900">MFA requirement</h2>
      <p class="mt-2 text-sm text-slate-600">
        Require every member to enroll in multi-factor authentication before they can sign in.
      </p>
    </div>
    <div class="flex items-center gap-3">
      <button
        type="button"
        class={`relative inline-flex h-6 w-11 items-center rounded-full transition disabled:opacity-60 ${
          mfaRequired ? 'bg-emerald-500' : 'bg-slate-300'
        }`}
        role="switch"
        aria-checked={mfaRequired}
        aria-label="Require MFA for all users"
        on:click={() => updateRequirement(!mfaRequired)}
        disabled={isLoading || isSaving}
      >
        <span
          class={`inline-block h-4 w-4 rounded-full bg-white shadow transition ${
            mfaRequired ? 'translate-x-6' : 'translate-x-1'
          }`}
        ></span>
      </button>
      <button
        type="button"
        class="rounded-full border border-slate-200 bg-white px-4 py-2 text-xs font-semibold text-slate-600 transition hover:border-slate-300 hover:bg-slate-50 disabled:opacity-60"
        on:click={refresh}
        disabled={isLoading || isSaving}
      >
        Refresh
      </button>
    </div>
  </div>

  {#if errorMessage}
    <div class="mt-6 rounded-xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">
      {errorMessage}
    </div>
  {/if}

  {#if successMessage}
    <div class="mt-6 rounded-xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-700">
      {successMessage}
    </div>
  {/if}

  <div class="mt-6 grid gap-4 md:grid-cols-[1.2fr,1fr]">
    <div class="rounded-2xl border border-slate-100 bg-slate-50/70 p-4">
      <p class="text-xs font-mono uppercase tracking-widest text-slate-500">Status</p>
      <div class="mt-3 flex flex-wrap items-center gap-3">
        <span
          class={`inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-semibold ${
            mfaRequired ? 'bg-emerald-100 text-emerald-700' : 'bg-slate-200 text-slate-600'
          }`}
        >
          <span class={`h-2 w-2 rounded-full ${mfaRequired ? 'bg-emerald-500' : 'bg-slate-400'}`}></span>
          {mfaRequired ? 'MFA required' : 'MFA optional'}
        </span>
        {#if isLoading}
          <span class="text-xs text-slate-400">Loading roster…</span>
        {:else}
          <span class="inline-flex items-center gap-2 rounded-full bg-amber-100 px-3 py-1 text-xs font-semibold text-amber-800">
            {usersWithoutMfa} without MFA · {totalUsers} total
          </span>
        {/if}
      </div>
      <p class="mt-3 text-sm text-slate-600">
        Members without MFA will be redirected to enrollment during sign-in when this is enabled.
      </p>
    </div>

    <div class="rounded-2xl border border-slate-100 bg-white p-4">
      <p class="text-xs font-mono uppercase tracking-widest text-slate-400">Before enabling</p>
      <p class="mt-3 text-sm text-slate-600">
        Turning on this policy blocks members without MFA until they enroll. Let your community
        know ahead of time and verify that backup codes are stored.
      </p>
    </div>
  </div>
</section>
