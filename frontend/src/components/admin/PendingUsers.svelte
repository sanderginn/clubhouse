<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { api } from '../../services/api';

  interface PendingUser {
    id: string;
    username: string;
    email: string;
    created_at: string;
  }

  let pendingUsers: PendingUser[] = [];
  let isLoading = true;
  let errorMessage = '';
  let actionUserId: string | null = null;
  let pendingUsersAbortController: AbortController | null = null;
  let pendingUsersTimeoutId: ReturnType<typeof setTimeout> | null = null;

  const formatDate = (value: string) => new Date(value).toLocaleString();

  const clearPendingUsersTimeout = () => {
    if (pendingUsersTimeoutId) {
      clearTimeout(pendingUsersTimeoutId);
      pendingUsersTimeoutId = null;
    }
  };

  const loadPendingUsers = async () => {
    pendingUsersAbortController?.abort();
    clearPendingUsersTimeout();

    const controller =
      typeof AbortController === 'undefined' ? null : new AbortController();
    pendingUsersAbortController = controller;

    isLoading = true;
    errorMessage = '';

    if (controller) {
      pendingUsersTimeoutId = setTimeout(() => controller.abort(), 10000);
    }

    try {
      const response = await api.get<PendingUser[] | null>(
        '/admin/users',
        controller ? { signal: controller.signal } : undefined
      );
      pendingUsers = Array.isArray(response) ? response : [];
    } catch (error) {
      if (pendingUsersAbortController !== controller) {
        return;
      }
      if (error instanceof Error && error.name === 'AbortError') {
        errorMessage = 'Request timed out. Please try again.';
      } else {
        errorMessage = error instanceof Error ? error.message : 'Failed to load pending users.';
      }
    } finally {
      if (pendingUsersAbortController === controller) {
        pendingUsersAbortController = null;
        clearPendingUsersTimeout();
        isLoading = false;
      }
    }
  };

  const approveUser = async (userId: string) => {
    actionUserId = userId;
    errorMessage = '';
    try {
      await api.patch(`/admin/users/${userId}/approve`);
      pendingUsers = pendingUsers.filter((user) => user.id !== userId);
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : 'Failed to approve user.';
    } finally {
      actionUserId = null;
    }
  };

  const rejectUser = async (userId: string) => {
    actionUserId = userId;
    errorMessage = '';
    try {
      await api.delete(`/admin/users/${userId}`);
      pendingUsers = pendingUsers.filter((user) => user.id !== userId);
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : 'Failed to reject user.';
    } finally {
      actionUserId = null;
    }
  };

  onMount(() => {
    loadPendingUsers();
  });

  onDestroy(() => {
    pendingUsersAbortController?.abort();
    clearPendingUsersTimeout();
  });
</script>

<section class="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
  <div class="flex flex-wrap items-start justify-between gap-4">
    <div>
      <p class="text-xs uppercase tracking-[0.3em] text-slate-400 font-mono">Queue</p>
      <h2 class="text-2xl font-serif font-semibold text-slate-900">Pending approvals</h2>
      <p class="mt-2 text-sm text-slate-600">
        Review new members and keep the roster intentional.
      </p>
    </div>
    <button
      class="rounded-full border border-slate-200 bg-white px-4 py-2 text-xs font-semibold text-slate-600 transition hover:border-slate-300 hover:bg-slate-50"
      on:click={loadPendingUsers}
    >
      Refresh
    </button>
  </div>

  {#if isLoading}
    <div class="mt-6 rounded-xl border border-dashed border-slate-200 bg-slate-50 p-6 text-sm text-slate-500">
      Loading pending users...
    </div>
  {:else if errorMessage}
    <div class="mt-6 rounded-xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">
      {errorMessage}
    </div>
  {:else if pendingUsers.length === 0}
    <div class="mt-6 rounded-xl border border-dashed border-emerald-200 bg-emerald-50 p-6 text-sm text-emerald-700">
      All caught up. No pending approvals right now.
    </div>
  {:else}
    <div class="mt-6 space-y-4">
      {#each pendingUsers as user (user.id)}
        <div class="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
          <div class="flex flex-wrap items-center justify-between gap-4">
            <div>
              <p class="text-lg font-semibold text-slate-900">{user.username}</p>
              <p class="text-sm text-slate-500">{user.email}</p>
              <p class="mt-2 text-xs font-mono uppercase tracking-widest text-slate-400">
                Joined {formatDate(user.created_at)}
              </p>
            </div>
            <div class="flex items-center gap-2">
              <button
                class="rounded-full bg-emerald-500 px-4 py-2 text-xs font-semibold text-white transition hover:bg-emerald-600 disabled:opacity-60"
                on:click={() => approveUser(user.id)}
                disabled={actionUserId === user.id}
              >
                Approve
              </button>
              <button
                class="rounded-full border border-rose-200 bg-rose-50 px-4 py-2 text-xs font-semibold text-rose-700 transition hover:bg-rose-100 disabled:opacity-60"
                on:click={() => rejectUser(user.id)}
                disabled={actionUserId === user.id}
              >
                Reject
              </button>
            </div>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</section>
