<script lang="ts">
  import { returnToFeed } from '../services/profileNavigation';
  import { buildFeedHref } from '../services/routeNavigation';

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
      class="inline-flex items-center gap-2 px-3 py-2 rounded-lg border border-gray-200 text-sm font-medium text-gray-700 hover:bg-gray-100"
      on:click={handleBack}
      type="button"
    >
      <span aria-hidden="true">←</span>
      Back
    </button>
  </div>

  <div class="grid gap-4">
    <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-5">
      <h2 class="text-lg font-semibold text-gray-900">Account</h2>
      <p class="text-sm text-gray-600 mt-1">
        Update your display name, email address, and profile picture. Coming in a follow-up.
      </p>
    </div>

    <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-5">
      <h2 class="text-lg font-semibold text-gray-900">Security</h2>
      <p class="text-sm text-gray-600 mt-1">
        Password reset, MFA setup, and active sessions. MFA will land in a future update.
      </p>
    </div>

    <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-5">
      <h2 class="text-lg font-semibold text-gray-900">Notifications</h2>
      <p class="text-sm text-gray-600 mt-1">
        Control mention alerts, push notifications, and digest frequency.
      </p>
    </div>
  </div>
</section>
