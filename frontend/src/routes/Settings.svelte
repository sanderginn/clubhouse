<script lang="ts">
  import { onDestroy } from 'svelte';
  import { returnToFeed } from '../services/profileNavigation';
  import { buildFeedHref } from '../services/routeNavigation';
  import { api } from '../services/api';
  import { authStore } from '../stores/authStore';
  import { currentUser } from '../stores/authStore';
  import MfaSetup from '../components/settings/MfaSetup.svelte';
  import { postStore } from '../stores/postStore';
  import { commentStore } from '../stores/commentStore';
  import { searchStore } from '../stores/searchStore';

  const allowedImageTypes = new Set([
    'image/jpeg',
    'image/png',
    'image/gif',
    'image/webp',
    'image/bmp',
    'image/avif',
    'image/tiff',
  ]);
  const maxImageBytes = 10 * 1024 * 1024;

  let fileInput: HTMLInputElement | null = null;
  let selectedFile: File | null = null;
  let previewUrl: string | null = null;
  let removeRequested = false;
  let uploadProgress = 0;
  let isSaving = false;
  let error: string | null = null;
  let successMessage: string | null = null;

  function handleHomeNavigation(event: MouseEvent) {
    if (
      event.defaultPrevented ||
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    ) {
      return;
    }
    event.preventDefault();
    returnToFeed();
  }

  function handleBack() {
    if (typeof window === 'undefined') return;
    if (window.history.length > 1) {
      window.history.back();
    } else {
      returnToFeed();
    }
  }

  function formatFileSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }

  function clearPreview() {
    if (previewUrl) {
      URL.revokeObjectURL(previewUrl);
      previewUrl = null;
    }
  }

  function resetSelection() {
    clearPreview();
    selectedFile = null;
    removeRequested = false;
    uploadProgress = 0;
    if (fileInput) {
      fileInput.value = '';
    }
  }

  function handleFileChange(event: Event) {
    const target = event.currentTarget as HTMLInputElement;
    const file = target.files?.[0];
    if (!file) return;

    error = null;
    successMessage = null;

    if (!allowedImageTypes.has(file.type)) {
      error = 'Supported formats: JPG, PNG, GIF, WEBP, BMP, AVIF, TIFF.';
      target.value = '';
      return;
    }

    if (file.size > maxImageBytes) {
      error = 'Image must be under 10 MB.';
      target.value = '';
      return;
    }

    clearPreview();
    selectedFile = file;
    previewUrl = URL.createObjectURL(file);
    removeRequested = false;
    uploadProgress = 0;
  }

  function handleRemove() {
    error = null;
    successMessage = null;
    clearPreview();
    selectedFile = null;
    removeRequested = true;
    uploadProgress = 0;
    if (fileInput) {
      fileInput.value = '';
    }
  }

  function handleClearSelection() {
    error = null;
    successMessage = null;
    resetSelection();
  }

  async function handleSave() {
    if (!$currentUser || isSaving) return;

    const hasChanges = removeRequested || !!selectedFile;
    if (!hasChanges) return;

    isSaving = true;
    error = null;
    successMessage = null;

    try {
      let profilePictureUrl = $currentUser.profilePictureUrl ?? '';

      if (selectedFile) {
        const response = await api.uploadImage(selectedFile, (progress) => {
          uploadProgress = progress;
        });
        profilePictureUrl = response.url;
      } else if (removeRequested) {
        profilePictureUrl = '';
      }

      const response = await api.patch<{
        id: string;
        username: string;
        email: string;
        profile_picture_url?: string | null;
        bio?: string | null;
        is_admin: boolean;
      }>('/users/me', { profile_picture_url: profilePictureUrl });

      const nextProfileUrl = response.profile_picture_url ?? '';
      const normalizedProfileUrl = nextProfileUrl || undefined;

      authStore.updateUser({
        id: response.id,
        username: response.username,
        email: response.email,
        profilePictureUrl: normalizedProfileUrl,
        bio: response.bio ?? undefined,
        isAdmin: response.is_admin,
      });

      postStore.updateUserProfilePicture(response.id, normalizedProfileUrl);
      commentStore.updateUserProfilePicture(response.id, normalizedProfileUrl);
      searchStore.updateUserProfilePicture(response.id, normalizedProfileUrl);

      resetSelection();
      successMessage = 'Profile picture updated.';
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to update profile picture.';
    } finally {
      isSaving = false;
    }
  }

  onDestroy(() => {
    clearPreview();
  });

  $: currentProfileUrl = $currentUser?.profilePictureUrl ?? '';
  $: displayUrl = removeRequested ? null : previewUrl ?? (currentProfileUrl || null);
  $: canRemove = !!displayUrl || !!currentProfileUrl;
  $: hasChanges = removeRequested || !!selectedFile;
  $: displayName = $currentUser?.username ?? 'User';
  $: initials = displayName ? displayName.trim().charAt(0).toUpperCase() : '?';
  $: selectedFileLabel = selectedFile
    ? `${selectedFile.name} • ${formatFileSize(selectedFile.size)}`
    : 'No file selected';
</script>

<section class="space-y-6">
  <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
    <div class="space-y-2">
      <nav aria-label="Breadcrumb">
        <ol class="flex items-center gap-2 text-sm text-gray-500">
          <li>
            <a
              href={buildFeedHref(null)}
              class="hover:text-gray-700"
              on:click={handleHomeNavigation}
            >
              Home
            </a>
          </li>
          <li class="text-gray-400">/</li>
          <li class="text-gray-700 font-medium">Settings</li>
        </ol>
      </nav>
      <h1 class="text-2xl font-bold text-gray-900">Settings</h1>
      <p class="text-gray-600">
        Manage your account preferences and community settings. More options are coming soon.
      </p>
    </div>
    <button
      class="inline-flex items-center gap-2 rounded-full border border-gray-200 bg-white px-3 py-1.5 text-xs font-semibold text-gray-600 hover:text-gray-900 hover:border-gray-300"
      on:click={handleBack}
      type="button"
    >
      <span aria-hidden="true">←</span>
      <span>Back to feed</span>
    </button>
  </div>

  <div class="grid gap-4">
    <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-5">
      <div class="flex flex-col gap-2">
        <h2 class="text-lg font-semibold text-gray-900">Account</h2>
        <p class="text-sm text-gray-600">
          Update your profile picture and account details. Profile info is visible to your community.
        </p>
      </div>

      <div class="mt-5 flex flex-col gap-5 sm:flex-row sm:items-start sm:justify-between">
        <div class="flex items-start gap-4">
          <div class="relative h-20 w-20 overflow-hidden rounded-full border border-gray-200 bg-gray-100">
            {#if displayUrl}
              <img src={displayUrl} alt="Profile preview" class="h-full w-full object-cover" />
            {:else}
              <div class="flex h-full w-full items-center justify-center text-xl font-semibold text-gray-500">
                {initials}
              </div>
            {/if}
            {#if isSaving && uploadProgress > 0}
              <div class="absolute inset-x-0 bottom-0 h-2 bg-gray-200">
                <div
                  class="h-full bg-primary transition-all"
                  style={`width: ${uploadProgress}%`}
                ></div>
              </div>
            {/if}
          </div>

          <div class="space-y-3">
            <div>
              <p class="text-sm font-medium text-gray-900">Profile picture</p>
              <p class="text-xs text-gray-500">
                Square images look best. Max size 10 MB.
              </p>
            </div>

            <div class="flex flex-wrap items-center gap-2">
              <input
                class="sr-only"
                type="file"
                accept="image/*"
                bind:this={fileInput}
                on:change={handleFileChange}
              />
              <button
                type="button"
                class="inline-flex items-center rounded-lg border border-gray-200 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 disabled:opacity-50"
                on:click={() => fileInput?.click()}
                disabled={!$currentUser || isSaving}
              >
                Upload photo
              </button>
              <button
                type="button"
                class="inline-flex items-center rounded-lg border border-gray-200 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 disabled:opacity-50"
                on:click={handleRemove}
                disabled={!$currentUser || isSaving || !canRemove}
              >
                Remove
              </button>
              {#if selectedFile}
                <button
                  type="button"
                  class="inline-flex items-center rounded-lg border border-gray-200 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 disabled:opacity-50"
                  on:click={handleClearSelection}
                  disabled={isSaving}
                >
                  Clear selection
                </button>
              {/if}
            </div>

            <p class="text-xs text-gray-500">{selectedFileLabel}</p>

            {#if error}
              <p class="text-sm text-red-600">{error}</p>
            {/if}
            {#if successMessage}
              <p class="text-sm text-emerald-600">{successMessage}</p>
            {/if}
          </div>
        </div>

        <div class="flex items-center gap-3">
          <button
            type="button"
            class="inline-flex items-center rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-white shadow-sm transition disabled:opacity-50"
            on:click={handleSave}
            disabled={!$currentUser || isSaving || !hasChanges}
          >
            {isSaving ? 'Saving...' : 'Save changes'}
          </button>
        </div>
      </div>
    </div>

    <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-5 space-y-4">
      <div>
        <h2 class="text-lg font-semibold text-gray-900">Security</h2>
        <p class="text-sm text-gray-600 mt-1">
          Keep your account safe with multi-factor authentication.
        </p>
      </div>
      <MfaSetup />
    </div>

    <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-5">
      <h2 class="text-lg font-semibold text-gray-900">Notifications</h2>
      <p class="text-sm text-gray-600 mt-1">
        Control mention alerts, push notifications, and digest frequency.
      </p>
    </div>
  </div>
</section>
