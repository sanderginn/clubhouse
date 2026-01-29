<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../services/api';
  import { replacePath } from '../../services/routeNavigation';

  interface AuditLog {
    id: string;
    admin_user_id?: string | null;
    admin_username?: string | null;
    action: string;
    related_post_id?: string | null;
    related_comment_id?: string | null;
    related_user_id?: string | null;
    related_username?: string | null;
    target_user_id?: string | null;
    target_username?: string | null;
    metadata?: Record<string, unknown> | null;
    created_at: string;
  }

  interface AuditLogsResponse {
    logs: AuditLog[];
    has_more: boolean;
    next_cursor?: string | null;
  }

  interface AuditLogActionsResponse {
    actions: string[];
  }

  interface AdminUser {
    id: string;
    username: string;
    email: string;
    is_admin: boolean;
    approved_at: string;
    created_at: string;
  }

  let logs: AuditLog[] = [];
  let cursor: string | null = null;
  let hasMore = true;
  let isLoading = false;
  let errorMessage = '';
  let isFilterLoading = false;
  let filterErrorMessage = '';

  let availableActions: string[] = [];
  let users: AdminUser[] = [];

  let selectedActions: string[] = [];
  let startDate = '';
  let endDate = '';
  let adminUserId = '';
  let targetUserId = '';
  let filtersInitialized = false;

  const actionLabels: Record<string, string> = {
    approve_user: 'Approved user',
    reject_user: 'Rejected user',
    hard_delete_post: 'Deleted post',
    hard_delete_comment: 'Deleted comment',
    restore_post: 'Restored post',
    restore_comment: 'Restored comment',
    delete_post: 'Deleted post',
    delete_comment: 'Deleted comment',
    toggle_link_metadata: 'Toggled link metadata',
    register_user: 'Registered user',
    update_profile: 'Updated profile',
    suspend_user: 'Suspended user',
    unsuspend_user: 'Unsuspended user',
    enroll_mfa: 'Enrolled MFA',
    enable_mfa: 'Enabled MFA',
    generate_password_reset_token: 'Generated password reset token',
  };

  const userActions = new Set([
    'approve_user',
    'reject_user',
    'suspend_user',
    'unsuspend_user',
    'register_user',
    'update_profile',
    'enroll_mfa',
    'enable_mfa',
    'generate_password_reset_token',
  ]);

  const formatAction = (action: string) => actionLabels[action] ?? action.replace(/_/g, ' ');
  const formatDate = (value: string) =>
    new Intl.DateTimeFormat('en-US', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));

  const getMetadataValue = (metadata: Record<string, unknown> | null | undefined, key: string) => {
    if (!metadata) return null;
    const value = metadata[key];
    return value === undefined || value === null ? null : value;
  };

  const isUserAction = (action: string) => userActions.has(action);

  const resolveUserLabel = (log: AuditLog) => {
    if (log.target_username) return log.target_username;
    if (log.related_username) return log.related_username;
    const metadataUsername = getMetadataValue(log.metadata, 'username');
    if (typeof metadataUsername === 'string' && metadataUsername.trim() !== '') {
      return metadataUsername;
    }
    const metadataEmail = getMetadataValue(log.metadata, 'email');
    if (typeof metadataEmail === 'string' && metadataEmail.trim() !== '') {
      return metadataEmail;
    }
    return null;
  };

  const resolveUserLabelByID = (log: AuditLog, userID: string) => {
    if (log.admin_user_id && log.admin_user_id === userID) {
      return log.admin_username ?? null;
    }
    if (log.target_user_id && log.target_user_id === userID && log.target_username) {
      return log.target_username;
    }
    if (log.related_user_id && log.related_user_id === userID && log.related_username) {
      return log.related_username;
    }
    return null;
  };

  const formatLogTitle = (log: AuditLog) => {
    const base = formatAction(log.action);
    if (!isUserAction(log.action)) {
      return base;
    }
    const userLabel = resolveUserLabel(log);
    return userLabel ? `${base} ${userLabel}` : base;
  };

  const formatRelated = (log: AuditLog) => {
    const parts: string[] = [];
    const userLabel = resolveUserLabel(log);
    if (userLabel) {
      parts.push(`User · ${userLabel}`);
    } else if (log.related_user_id) {
      parts.push(`User · ${log.related_user_id.slice(0, 8)}…`);
    } else if (log.target_user_id) {
      parts.push(`User · ${log.target_user_id.slice(0, 8)}…`);
    }

    if (log.related_post_id) parts.push(`Post · ${log.related_post_id.slice(0, 8)}…`);
    if (log.related_comment_id) parts.push(`Comment · ${log.related_comment_id.slice(0, 8)}…`);

    return parts.length > 0 ? parts.join(' · ') : 'System';
  };

  const formatMetadataSummary = (log: AuditLog) => {
    const metadata = log.metadata ?? {};
    const details: string[] = [];

    if (log.action === 'toggle_link_metadata') {
      const setting = getMetadataValue(metadata, 'setting');
      const oldValue = getMetadataValue(metadata, 'old_value');
      const newValue = getMetadataValue(metadata, 'new_value');
      if (setting !== null && oldValue !== null && newValue !== null) {
        details.push(`${setting}: ${oldValue} → ${newValue}`);
      }
    }

    const excerpt = getMetadataValue(metadata, 'content_excerpt');
    if (typeof excerpt === 'string' && excerpt.trim() !== '') {
      details.push(`Excerpt: ${excerpt}`);
    }

    const changedFields = getMetadataValue(metadata, 'changed_fields');
    if (Array.isArray(changedFields) && changedFields.length > 0) {
      details.push(`Updated fields: ${changedFields.join(', ')}`);
    }

    const deletedBy = getMetadataValue(metadata, 'deleted_by_user_id');
    if (typeof deletedBy === 'string' && deletedBy.trim() !== '') {
      const userLabel = resolveUserLabelByID(log, deletedBy);
      details.push(`Deleted by ${userLabel ?? `${deletedBy.slice(0, 8)}…`}`);
    }

    const restoredBy = getMetadataValue(metadata, 'restored_by_user_id');
    if (typeof restoredBy === 'string' && restoredBy.trim() !== '') {
      const userLabel = resolveUserLabelByID(log, restoredBy);
      details.push(`Restored by ${userLabel ?? `${restoredBy.slice(0, 8)}…`}`);
    }

    return details;
  };

  const buildActionOptions = () => {
    const options = new Set<string>([...availableActions, ...Object.keys(actionLabels), ...selectedActions]);
    return Array.from(options).sort((a, b) => a.localeCompare(b));
  };

  const buildFilterParams = () => {
    const params = new URLSearchParams();
    params.set('tab', 'audit');
    selectedActions.forEach((action) => params.append('action', action));
    if (startDate) params.set('start', startDate);
    if (endDate) params.set('end', endDate);
    if (adminUserId) params.set('admin_user_id', adminUserId);
    if (targetUserId) params.set('target_user_id', targetUserId);
    return params;
  };

  const syncUrlWithFilters = () => {
    if (typeof window === 'undefined') return;
    const params = buildFilterParams();
    const query = params.toString();
    const path = `${window.location.pathname}${query ? `?${query}` : ''}`;
    replacePath(path);
  };

  const parseDateParam = (value: string | null) => {
    if (!value) return '';
    if (/^\d{4}-\d{2}-\d{2}$/.test(value)) return value;
    return '';
  };

  const loadFilters = async () => {
    if (isFilterLoading) return;
    isFilterLoading = true;
    filterErrorMessage = '';
    try {
      const [actionsResponse, usersResponse] = await Promise.all([
        api.get<AuditLogActionsResponse>('/admin/audit-logs/actions'),
        api.get<AdminUser[] | null>('/admin/users/approved'),
      ]);
      availableActions = Array.isArray(actionsResponse?.actions) ? actionsResponse.actions : [];
      users = Array.isArray(usersResponse) ? usersResponse : [];
    } catch (error) {
      filterErrorMessage = error instanceof Error ? error.message : 'Failed to load audit filters.';
    } finally {
      isFilterLoading = false;
    }
  };

  const applyFilters = async ({ updateUrl = true }: { updateUrl?: boolean } = {}) => {
    if (!filtersInitialized) return;
    if (updateUrl) {
      syncUrlWithFilters();
    }
    await loadLogs({ reset: true });
  };

  const clearFilters = async () => {
    selectedActions = [];
    startDate = '';
    endDate = '';
    adminUserId = '';
    targetUserId = '';
    await applyFilters();
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
      const params = buildFilterParams();
      if (cursor) {
        params.set('cursor', cursor);
      }
      const query = params.toString();
      const response = await api.get<AuditLogsResponse>(`/admin/audit-logs${query ? `?${query}` : ''}`);
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
    if (typeof window !== 'undefined') {
      const params = new URLSearchParams(window.location.search);
      selectedActions = params.getAll('action');
      startDate = parseDateParam(params.get('start'));
      endDate = parseDateParam(params.get('end'));
      adminUserId = params.get('admin_user_id') ?? '';
      targetUserId = params.get('target_user_id') ?? '';
    }
    filtersInitialized = true;
    syncUrlWithFilters();
    loadFilters();
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

  <div class="mt-6 rounded-2xl border border-slate-200 bg-slate-50 p-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <p class="text-xs font-semibold uppercase tracking-[0.3em] text-slate-400">Filters</p>
        <p class="text-sm text-slate-600">Slice the log by action, date range, or user.</p>
      </div>
      <button
        class="rounded-full border border-slate-200 bg-white px-4 py-2 text-xs font-semibold text-slate-600 transition hover:border-slate-300 hover:bg-slate-50"
        on:click={clearFilters}
        disabled={isLoading}
      >
        Clear filters
      </button>
    </div>

    {#if filterErrorMessage}
      <div class="mt-3 rounded-xl border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700">
        {filterErrorMessage}
      </div>
    {/if}

    <div class="mt-4 grid gap-4 lg:grid-cols-2 xl:grid-cols-4">
      <div class="space-y-2">
        <p class="text-xs font-semibold uppercase tracking-[0.2em] text-slate-500">Action type</p>
        <div class="max-h-44 space-y-2 overflow-auto rounded-xl border border-slate-200 bg-white p-3 text-xs text-slate-600">
          {#if isFilterLoading}
            <p class="text-slate-400">Loading action types...</p>
          {:else if buildActionOptions().length === 0}
            <p class="text-slate-400">No action types yet.</p>
          {:else}
            {#each buildActionOptions() as action}
              <label class="flex items-center gap-2">
                <input
                  class="h-4 w-4 rounded border-slate-300 text-amber-500 focus:ring-amber-400"
                  type="checkbox"
                  value={action}
                  bind:group={selectedActions}
                  on:change={() => applyFilters()}
                />
                <span>{formatAction(action)}</span>
              </label>
            {/each}
          {/if}
        </div>
      </div>

      <div class="space-y-2">
        <p class="text-xs font-semibold uppercase tracking-[0.2em] text-slate-500">Date range</p>
        <div class="space-y-3">
          <label class="block text-xs text-slate-500">
            Start date
            <input
              class="mt-2 w-full rounded-xl border border-slate-200 px-3 py-2 text-sm text-slate-700"
              type="date"
              bind:value={startDate}
              on:change={() => applyFilters()}
            />
          </label>
          <label class="block text-xs text-slate-500">
            End date
            <input
              class="mt-2 w-full rounded-xl border border-slate-200 px-3 py-2 text-sm text-slate-700"
              type="date"
              bind:value={endDate}
              on:change={() => applyFilters()}
            />
          </label>
        </div>
      </div>

      <div class="space-y-2">
        <p class="text-xs font-semibold uppercase tracking-[0.2em] text-slate-500">Admin user</p>
        <select
          class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm text-slate-700"
          bind:value={adminUserId}
          on:change={() => applyFilters()}
        >
          <option value="">Any admin</option>
          {#each users.filter((user) => user.is_admin) as user}
            <option value={user.id}>{user.username} · {user.email}</option>
          {/each}
        </select>
      </div>

      <div class="space-y-2">
        <p class="text-xs font-semibold uppercase tracking-[0.2em] text-slate-500">Target user</p>
        <select
          class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm text-slate-700"
          bind:value={targetUserId}
          on:change={() => applyFilters()}
        >
          <option value="">Any user</option>
          {#each users as user}
            <option value={user.id}>{user.username} · {user.email}</option>
          {/each}
        </select>
      </div>
    </div>
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
        {@const summary = formatMetadataSummary(log)}
        <div class="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
          <div class="flex flex-wrap items-center justify-between gap-4">
            <div class="space-y-2">
              <p class="text-sm font-semibold text-slate-900">{formatLogTitle(log)}</p>
              <p class="text-xs text-slate-500">
                {log.admin_username || 'System'} · {formatRelated(log)}
              </p>
              {#if summary.length > 0}
                <div class="flex flex-wrap gap-2 text-xs text-slate-500">
                  {#each summary as detail}
                    <span class="rounded-full bg-slate-100 px-3 py-1">{detail}</span>
                  {/each}
                </div>
              {/if}
            </div>
            <div class="text-xs font-mono uppercase tracking-widest text-slate-400">
              {formatDate(log.created_at)}
            </div>
          </div>
          <details class="mt-4 rounded-xl border border-slate-100 bg-slate-50 p-3 text-xs text-slate-600">
            <summary class="cursor-pointer select-none text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-400">
              Metadata
            </summary>
            <pre class="mt-3 whitespace-pre-wrap break-words font-mono text-[11px] text-slate-500">
{JSON.stringify(log.metadata ?? {}, null, 2)}
            </pre>
          </details>
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
