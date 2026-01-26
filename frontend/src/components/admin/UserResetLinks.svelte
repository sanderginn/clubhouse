<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { api } from '../../services/api';

  interface AdminUser {
    id: string;
    username: string;
    email: string;
    is_admin: boolean;
    approved_at: string;
    created_at: string;
  }

  interface ResetLinkResponse {
    token: string;
    user_id: string;
    expires_at: string;
  }

  interface ResetLinkState {
    url: string;
    expiresAt: string;
    copied: boolean;
  }

  let users: AdminUser[] = [];
  let isLoading = true;
  let errorMessage = '';
  let actionUserId: string | null = null;
  let resetLinks: Record<string, ResetLinkState> = {};
  let usersAbortController: AbortController | null = null;
  let usersTimeoutId: ReturnType<typeof setTimeout> | null = null;

  const formatDate = (value: string) => new Date(value).toLocaleString();

  const clearUsersTimeout = () => {
    if (usersTimeoutId) {
      clearTimeout(usersTimeoutId);
      usersTimeoutId = null;
    }
  };

  const loadUsers = async () => {
    usersAbortController?.abort();
    clearUsersTimeout();

    const controller = typeof AbortController === 'undefined' ? null : new AbortController();
    usersAbortController = controller;

    isLoading = true;
    errorMessage = '';

    if (controller) {
      usersTimeoutId = setTimeout(() => controller.abort(), 10000);
    }

    try {
      users = await api.get<AdminUser[]>(
        '/admin/users/approved',
        controller ? { signal: controller.signal } : undefined
      );
    } catch (error) {
      if (usersAbortController !== controller) {
        return;
      }
      if (error instanceof Error && error.name === 'AbortError') {
        errorMessage = 'Request timed out. Please try again.';
      } else {
        errorMessage = error instanceof Error ? error.message : 'Failed to load users.';
      }
    } finally {
      if (usersAbortController === controller) {
        usersAbortController = null;
        clearUsersTimeout();
        isLoading = false;
      }
    }
  };

  const buildResetLink = (token: string) => {
    const path = '/reset-password';
    if (typeof window === 'undefined') {
      return `${path}?token=${encodeURIComponent(token)}`;
    }

    const url = new URL(path, window.location.origin);
    url.searchParams.set('token', token);
    return url.toString();
  };

  const generateResetLink = async (userId: string) => {
    actionUserId = userId;
    errorMessage = '';
    try {
      const response = await api.post<ResetLinkResponse>('/admin/password-reset/generate', {
        user_id: userId,
      });
      resetLinks = {
        ...resetLinks,
        [userId]: {
          url: buildResetLink(response.token),
          expiresAt: response.expires_at,
          copied: false,
        },
      };
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : 'Failed to generate reset link.';
    } finally {
      actionUserId = null;
    }
  };

  const copyResetLink = async (userId: string) => {
    const link = resetLinks[userId]?.url;
    if (!link) return;

    let copied = false;
    if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
      try {
        await navigator.clipboard.writeText(link);
        copied = true;
      } catch {
        copied = false;
      }
    }

    if (!copied && typeof document !== 'undefined' && typeof document.execCommand === 'function') {
      const textarea = document.createElement('textarea');
      textarea.value = link;
      textarea.setAttribute('readonly', '');
      textarea.style.position = 'absolute';
      textarea.style.left = '-9999px';
      document.body.appendChild(textarea);
      textarea.select();
      copied = document.execCommand('copy');
      document.body.removeChild(textarea);
    }

    resetLinks = {
      ...resetLinks,
      [userId]: {
        ...resetLinks[userId],
        copied,
      },
    };
  };

  const clearResetLink = (userId: string) => {
    const { [userId]: _removed, ...remaining } = resetLinks;
    resetLinks = remaining;
  };

  onMount(() => {
    loadUsers();
  });

  onDestroy(() => {
    usersAbortController?.abort();
    clearUsersTimeout();
  });
</script>

<section class="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
  <div class="flex flex-wrap items-start justify-between gap-4">
    <div>
      <p class="text-xs uppercase tracking-[0.3em] text-slate-400 font-mono">Roster</p>
      <h2 class="text-2xl font-serif font-semibold text-slate-900">Members</h2>
      <p class="mt-2 text-sm text-slate-600">
        Generate single-use password reset links for approved members. Links expire after one hour.
      </p>
    </div>
    <button
      class="rounded-full border border-slate-200 bg-white px-4 py-2 text-xs font-semibold text-slate-600 transition hover:border-slate-300 hover:bg-slate-50"
      on:click={loadUsers}
    >
      Refresh
    </button>
  </div>

  {#if isLoading}
    <div class="mt-6 rounded-xl border border-dashed border-slate-200 bg-slate-50 p-6 text-sm text-slate-500">
      Loading members...
    </div>
  {:else if errorMessage}
    <div class="mt-6 rounded-xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">
      {errorMessage}
    </div>
  {:else if users.length === 0}
    <div class="mt-6 rounded-xl border border-dashed border-slate-200 bg-slate-50 p-6 text-sm text-slate-500">
      No approved members yet.
    </div>
  {:else}
    <div class="mt-6 space-y-4">
      {#each users as user (user.id)}
        <div class="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div>
              <div class="flex flex-wrap items-center gap-2">
                <p class="text-lg font-semibold text-slate-900">{user.username}</p>
                {#if user.is_admin}
                  <span class="rounded-full bg-indigo-50 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.2em] text-indigo-600">
                    Admin
                  </span>
                {/if}
              </div>
              <p class="text-sm text-slate-500">{user.email || 'No email on file'}</p>
              <p class="mt-2 text-xs font-mono uppercase tracking-widest text-slate-400">
                Approved {formatDate(user.approved_at)}
              </p>
            </div>
            <div class="flex items-center gap-2">
              <button
                class="rounded-full border border-amber-200 bg-amber-50 px-4 py-2 text-xs font-semibold text-amber-700 transition hover:bg-amber-100 disabled:opacity-60"
                on:click={() => generateResetLink(user.id)}
                disabled={actionUserId === user.id}
              >
                Generate reset link
              </button>
            </div>
          </div>

          {#if resetLinks[user.id]}
            <div class="mt-4 rounded-xl border border-amber-200 bg-amber-50 p-4">
              <p class="text-xs font-mono uppercase tracking-widest text-amber-600">One-time link</p>
              <div class="mt-2 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <p class="break-all text-sm text-amber-900">{resetLinks[user.id].url}</p>
                <div class="flex items-center gap-2">
                  <button
                    class="rounded-full bg-amber-600 px-3 py-1 text-xs font-semibold text-white transition hover:bg-amber-700"
                    on:click={() => copyResetLink(user.id)}
                  >
                    {resetLinks[user.id].copied ? 'Copied' : 'Copy link'}
                  </button>
                  <button
                    class="rounded-full border border-amber-200 bg-white px-3 py-1 text-xs font-semibold text-amber-700 transition hover:bg-amber-100"
                    on:click={() => clearResetLink(user.id)}
                  >
                    Hide
                  </button>
                </div>
              </div>
              <p class="mt-3 text-xs text-amber-700">
                Single-use link. Expires {formatDate(resetLinks[user.id].expiresAt)}.
              </p>
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</section>
