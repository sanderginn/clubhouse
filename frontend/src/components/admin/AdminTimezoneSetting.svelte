<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api } from '../../services/api';
  import { configStore } from '../../stores';
  import { formatInTimezone } from '../../lib/time';

  interface ConfigResponse {
    config?: {
      displayTimezone?: string;
      display_timezone?: string;
    };
  }

  const baseTimezones = [
    { value: 'UTC', label: 'UTC' },
    { value: 'America/New_York', label: 'America/New_York (ET)' },
    { value: 'America/Chicago', label: 'America/Chicago (CT)' },
    { value: 'America/Denver', label: 'America/Denver (MT)' },
    { value: 'America/Los_Angeles', label: 'America/Los_Angeles (PT)' },
    { value: 'Europe/London', label: 'Europe/London' },
    { value: 'Europe/Berlin', label: 'Europe/Berlin' },
    { value: 'Europe/Paris', label: 'Europe/Paris' },
    { value: 'Asia/Dubai', label: 'Asia/Dubai' },
    { value: 'Asia/Singapore', label: 'Asia/Singapore' },
    { value: 'Asia/Tokyo', label: 'Asia/Tokyo' },
    { value: 'Australia/Sydney', label: 'Australia/Sydney' },
  ];

  let isLoading = true;
  let isSaving = false;
  let errorMessage = '';
  let successMessage = '';
  let selectedTimezone = '';
  let configAbortController: AbortController | null = null;
  let configTimeoutId: ReturnType<typeof setTimeout> | null = null;

  const normalizeTimezone = (response: ConfigResponse | null): string => {
    const config = response?.config;
    if (!config) return '';
    if (typeof config.displayTimezone === 'string' && config.displayTimezone.trim() !== '') {
      return config.displayTimezone.trim();
    }
    if (typeof config.display_timezone === 'string' && config.display_timezone.trim() !== '') {
      return config.display_timezone.trim();
    }
    return '';
  };

  const clearTimeouts = () => {
    if (configTimeoutId) {
      clearTimeout(configTimeoutId);
      configTimeoutId = null;
    }
  };

  const loadConfig = async () => {
    configAbortController?.abort();
    clearTimeouts();

    const controller = typeof AbortController === 'undefined' ? null : new AbortController();
    configAbortController = controller;

    if (controller) {
      configTimeoutId = setTimeout(() => controller.abort(), 10000);
    }

    isLoading = true;
    errorMessage = '';
    successMessage = '';

    try {
      const response = await api.get<ConfigResponse>(
        '/admin/config',
        controller ? { signal: controller.signal } : undefined
      );
      const timezone = normalizeTimezone(response);
      selectedTimezone = timezone || baseTimezones[0].value;
      configStore.setDisplayTimezone(selectedTimezone || null);
    } catch (error) {
      if (error instanceof Error && error.name === 'AbortError') {
        errorMessage = 'Request timed out. Please try again.';
      } else {
        errorMessage = error instanceof Error ? error.message : 'Failed to load timezone settings.';
      }
    } finally {
      if (configAbortController === controller) {
        configAbortController = null;
        clearTimeouts();
        isLoading = false;
      }
    }
  };

  const updateTimezone = async () => {
    if (!selectedTimezone) return;
    isSaving = true;
    errorMessage = '';
    successMessage = '';

    try {
      const response = await api.patch<ConfigResponse>('/admin/config', {
        display_timezone: selectedTimezone,
      });
      const timezone = normalizeTimezone(response) || selectedTimezone;
      selectedTimezone = timezone;
      configStore.setDisplayTimezone(timezone || null);
      successMessage = 'Display timezone updated.';
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : 'Failed to update timezone.';
    } finally {
      isSaving = false;
    }
  };

  if (typeof window !== 'undefined') {
    void loadConfig();
  }

  onDestroy(() => {
    configAbortController?.abort();
    clearTimeouts();
  });

  $: timezoneOptions =
    selectedTimezone && !baseTimezones.some((timezone) => timezone.value === selectedTimezone)
      ? [{ value: selectedTimezone, label: `${selectedTimezone} (custom)` }, ...baseTimezones]
      : baseTimezones;

  $: previewLabel = selectedTimezone
    ? formatInTimezone(new Date(), { dateStyle: 'full', timeStyle: 'long' }, selectedTimezone)
    : 'Select a timezone to preview the current time.';
</script>

<section class="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
  <div class="flex flex-wrap items-start justify-between gap-4">
    <div>
      <p class="text-xs uppercase tracking-[0.3em] text-slate-400 font-mono">Display</p>
      <h2 class="text-2xl font-serif font-semibold text-slate-900">Timezone</h2>
      <p class="mt-2 text-sm text-slate-600">
        Choose the timezone used for all timestamps across the community.
      </p>
    </div>
    <button
      type="button"
      class="rounded-full border border-slate-200 bg-white px-4 py-2 text-xs font-semibold text-slate-600 transition hover:border-slate-300 hover:bg-slate-50 disabled:opacity-60"
      on:click={loadConfig}
      disabled={isLoading || isSaving}
    >
      Refresh
    </button>
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

  <div class="mt-6 grid gap-4">
    <label class="block text-sm font-semibold text-slate-700" for="timezone-select">
      Display timezone
    </label>
    <select
      id="timezone-select"
      class="w-full rounded-xl border border-slate-200 bg-white px-4 py-3 text-sm text-slate-700 shadow-sm focus:border-amber-400 focus:outline-none"
      bind:value={selectedTimezone}
      disabled={isLoading || isSaving}
    >
      {#each timezoneOptions as timezone}
        <option value={timezone.value}>{timezone.label}</option>
      {/each}
    </select>
    <div class="rounded-xl border border-slate-100 bg-slate-50 p-4 text-sm text-slate-600">
      <p class="text-xs uppercase tracking-[0.2em] text-slate-400 font-mono">Preview</p>
      <p class="mt-2 font-semibold text-slate-700">{previewLabel}</p>
    </div>
    <div class="flex items-center gap-3">
      <button
        type="button"
        class="rounded-full bg-slate-900 px-5 py-2 text-xs font-semibold text-white shadow transition hover:bg-slate-800 disabled:opacity-60"
        on:click={updateTimezone}
        disabled={isLoading || isSaving}
      >
        Save timezone
      </button>
      <p class="text-xs text-slate-500">Changes apply immediately after saving.</p>
    </div>
  </div>
</section>
