<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../services/api';

  interface AuditLog {
    id: string;
    admin_user_id: string;
    admin_username: string;
    action: string;
    related_post_id?: string | null;
    related_comment_id?: string | null;
    related_user_id?: string | null;
    created_at: string;
  }

  interface AuditLogsResponse {
    logs: AuditLog[];
    has_more: boolean;
    next_cursor?: string | null;
  }

  let logs: AuditLog[] = [];
  let cursor: string | null = null;
  let hasMore = true;
  let isLoading = false;
  let errorMessage = '';

  const actionLabels: Record<string, string> = {
    approve_user: 'Approved user',
    reject_user: 'Rejected user',
    hard_delete_post: 'Deleted post',
    hard_delete_comment: 'Deleted comment',
    restore_post: 'Restored post',
    restore_comment: 'Restored comment',
  };

  const formatAction = (action: string) => actionLabels[action] ?? action.replace(/_/g, ' ');
  const formatDate = (value: string) => new Date(value).toLocaleString();

  const formatRelated = (log: AuditLog) => {
    if (log.related_user_id) return `User · ${log.related_user_id.slice(0, 8)}…`;
    if (log.related_post_id) return `Post · ${log.related_post_id.slice(0, 8)}…`;
    if (log.related_comment_id) return `Comment · ${log.related_comment_id.slice(0, 8)}…`;
    return 'System';
  };

  const loadLogs = async ({ reset = false }: { reset?: boolean } = {}) => {
    if (isLoading) return;
    isLoading = true;
    errorMessage = '';

    if (reset) {
      cursor = null;
      hasMore = true;
      logs = [];
    }

    try {
      const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
      const response = await api.get<AuditLogsResponse>(`/admin/audit-logs${query}`);
      logs = reset ? response.logs : [...logs, ...response.logs];
      hasMore = response.has_more;
      cursor = response.next_cursor ?? null;
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : 'Failed to fetch audit logs.';
    } finally {
      isLoading = false;
    }
  };

  onMount(() => {
    loadLogs();
  });
</script>

<section class="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
  <div class="flex flex-wrap items-start justify-between gap-4">
    <div>
      <p class="text-xs uppercase tracking-[0.3em] text-slate-400 font-mono">Ledger</p>
      <h2 class="text-2xl font-serif font-semibold text-slate-900">Audit log</h2>
      <p class="mt-2 text-sm text-slate-600">
        Every admin action leaves a trace. Scroll for history and context.
      </p>
    </div>
    <button
      class="rounded-full border border-slate-200 bg-white px-4 py-2 text-xs font-semibold text-slate-600 transition hover:border-slate-300 hover:bg-slate-50"
      on:click={() => loadLogs({ reset: true })}
      disabled={isLoading}
    >
      Refresh
    </button>
  </div>

  {#if isLoading && logs.length === 0}
    <div class="mt-6 rounded-xl border border-dashed border-slate-200 bg-slate-50 p-6 text-sm text-slate-500">
      Loading audit logs...
    </div>
  {:else if errorMessage}
    <div class="mt-6 rounded-xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">
      {errorMessage}
    </div>
  {:else if logs.length === 0}
    <div class="mt-6 rounded-xl border border-dashed border-slate-200 bg-slate-50 p-6 text-sm text-slate-500">
      No audit logs yet.
    </div>
  {:else}
    <div class="mt-6 space-y-4">
      {#each logs as log (log.id)}
        <div class="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
          <div class="flex flex-wrap items-center justify-between gap-4">
            <div class="space-y-2">
              <p class="text-sm font-semibold text-slate-900">{formatAction(log.action)}</p>
              <p class="text-xs text-slate-500">
                {log.admin_username} · {formatRelated(log)}
              </p>
            </div>
            <div class="text-xs font-mono uppercase tracking-widest text-slate-400">
              {formatDate(log.created_at)}
            </div>
          </div>
        </div>
      {/each}
    </div>
  {/if}

  <div class="mt-6 flex items-center justify-center">
    <button
      class="rounded-full border border-slate-200 bg-white px-5 py-2 text-xs font-semibold text-slate-600 transition hover:border-slate-300 hover:bg-slate-50 disabled:opacity-60"
      on:click={() => loadLogs()}
      disabled={!hasMore || isLoading}
    >
      {#if hasMore}
        {#if isLoading}
          Loading...
        {:else}
          Load more
        {/if}
      {:else}
        End of log
      {/if}
    </button>
  </div>
</section>
