<script lang="ts">
  import { fade, fly } from 'svelte/transition';
  import PendingUsers from '../components/admin/PendingUsers.svelte';
  import AuditLogs from '../components/admin/AuditLogs.svelte';
  import UserResetLinks from '../components/admin/UserResetLinks.svelte';

  type AdminTab = 'pending' | 'users' | 'audit';

  const tabs: { id: AdminTab; label: string; description: string }[] = [
    {
      id: 'pending',
      label: 'Pending Users',
      description: 'Approve or reject new member requests.',
    },
    {
      id: 'users',
      label: 'Members',
      description: 'Manage approved users and generate reset links.',
    },
    {
      id: 'audit',
      label: 'Audit Logs',
      description: 'Track admin actions and system changes.',
    },
  ];

  let activeTab: AdminTab = 'pending';
</script>

<div class="admin-panel space-y-8">
  <section
    class="admin-hero relative overflow-hidden rounded-3xl border border-amber-100 bg-white px-6 py-8 shadow-sm"
    transition:fade
  >
    <div class="absolute inset-0 admin-grid" aria-hidden="true"></div>
    <div class="relative z-10 space-y-3">
      <p class="text-xs uppercase tracking-[0.35em] text-amber-700 font-mono">Moderation Console</p>
      <h1 class="text-3xl sm:text-4xl font-serif font-semibold text-slate-900">
        Keep Clubhouse calm, clear, and welcoming.
      </h1>
      <p class="max-w-2xl text-slate-600">
        Review membership requests, monitor admin actions, and keep the community aligned. All
        moderation actions are tracked here.
      </p>
      <div class="flex flex-wrap gap-3 pt-2">
        <span class="inline-flex items-center gap-2 rounded-full bg-amber-100 px-3 py-1 text-xs font-semibold text-amber-900">
          🧭 Guided reviews
        </span>
        <span class="inline-flex items-center gap-2 rounded-full bg-emerald-100 px-3 py-1 text-xs font-semibold text-emerald-900">
          ✅ Instant approvals
        </span>
        <span class="inline-flex items-center gap-2 rounded-full bg-slate-100 px-3 py-1 text-xs font-semibold text-slate-700">
          📜 Full audit trail
        </span>
      </div>
    </div>
  </section>

  <section class="grid gap-4 md:grid-cols-[1fr,2fr]">
    <div class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm" transition:fly={{ y: 12, duration: 220 }}>
      <h2 class="text-lg font-serif font-semibold text-slate-900">Admin Toolkit</h2>
      <p class="mt-2 text-sm text-slate-600">
        Choose a workflow. Approvals keep your roster curated; audit logs keep everything transparent.
      </p>
      <div class="mt-4 space-y-3">
        {#each tabs as tab}
          <button
            class={`w-full rounded-xl border px-4 py-3 text-left transition shadow-sm ${
              activeTab === tab.id
                ? 'border-amber-400 bg-amber-50 text-amber-900'
                : 'border-slate-200 bg-white text-slate-700 hover:border-slate-300 hover:bg-slate-50'
            }`}
            on:click={() => (activeTab = tab.id)}
          >
            <div class="flex items-start justify-between gap-2">
              <div>
                <p class="text-sm font-semibold">{tab.label}</p>
                <p class="mt-1 text-xs text-slate-500">{tab.description}</p>
              </div>
              <span class="text-xs font-mono uppercase tracking-widest text-slate-400">{tab.id}</span>
            </div>
          </button>
        {/each}
      </div>
    </div>

    <div class="min-h-[420px]">
      {#if activeTab === 'pending'}
        <div transition:fade>
          <PendingUsers />
        </div>
      {:else if activeTab === 'users'}
        <div transition:fade>
          <UserResetLinks />
        </div>
      {:else}
        <div transition:fade>
          <AuditLogs />
        </div>
      {/if}
    </div>
  </section>
</div>

<style>
  .admin-hero {
    background: radial-gradient(circle at top left, rgba(251, 191, 36, 0.15), transparent 55%),
      radial-gradient(circle at bottom right, rgba(16, 185, 129, 0.18), transparent 50%),
      linear-gradient(120deg, #fff8f1 0%, #ffffff 60%, #f5f8ff 100%);
  }

  .admin-grid {
    background-image: linear-gradient(rgba(148, 163, 184, 0.15) 1px, transparent 1px),
      linear-gradient(90deg, rgba(148, 163, 184, 0.15) 1px, transparent 1px);
    background-size: 32px 32px;
    mask-image: radial-gradient(circle at top left, rgba(0, 0, 0, 0.8), transparent 70%);
  }
</style>
