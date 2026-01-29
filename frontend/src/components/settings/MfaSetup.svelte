<script lang="ts">
  import QRCode from 'qrcode';
  import { api } from '../../services/api';
  import { authStore, currentUser } from '../../stores/authStore';

  interface TotpEnrollResponse {
    secret?: string;
    otpauth_url?: string;
    message?: string;
  }

  interface TotpVerifyResponse {
    message?: string;
    backup_codes?: string[];
  }

  interface TotpDisableResponse {
    message?: string;
  }

  let isLoading = false;
  let isVerifying = false;
  let isDisabling = false;
  let errorMessage = '';
  let successMessage = '';
  let isConfigMissing = false;
  let code = '';
  let disableCode = '';
  let qrCode = '';
  let manualKey = '';
  let otpauthUrl = '';
  let backupCodes: string[] = [];
  let backupConfirmed = false;
  let backupDismissed = false;

  const buildQrCode = async (value: string) => {
    if (!value) return '';
    try {
      return await QRCode.toDataURL(value, { margin: 1, width: 200 });
    } catch {
      return '';
    }
  };

  const resetMessages = () => {
    errorMessage = '';
    successMessage = '';
    isConfigMissing = false;
  };

  const resetEnrollmentData = () => {
    qrCode = '';
    manualKey = '';
    otpauthUrl = '';
    code = '';
    backupCodes = [];
    backupConfirmed = false;
    backupDismissed = false;
  };

  const resetEnrollment = () => {
    resetEnrollmentData();
    resetMessages();
  };

  const startEnrollment = async () => {
    isLoading = true;
    resetMessages();
    try {
      const response = await api.post<TotpEnrollResponse>('/users/me/mfa/enable');
      manualKey = response.secret ?? '';
      otpauthUrl = response.otpauth_url ?? '';
      qrCode = await buildQrCode(otpauthUrl);
    } catch (error) {
      const errorWithCode = error as Error & { code?: string };
      if (errorWithCode.code === 'TOTP_CONFIG_MISSING') {
        isConfigMissing = true;
        errorMessage = 'TOTP is not configured on the server yet.';
      } else {
        errorMessage = error instanceof Error ? error.message : 'Failed to start enrollment.';
      }
    } finally {
      isLoading = false;
    }
  };

  const verifyEnrollment = async () => {
    const trimmed = code.replace(/\s+/g, '');
    if (!trimmed) {
      errorMessage = 'Authentication code is required.';
      return;
    }
    isVerifying = true;
    resetMessages();
    try {
      const response = await api.post<TotpVerifyResponse>('/users/me/mfa/verify', { code: trimmed });
      successMessage = response.message || 'Multi-factor authentication enabled.';
      backupCodes = response.backup_codes ?? [];
      backupConfirmed = false;
      backupDismissed = false;
      authStore.updateUser({ totpEnabled: true });
      code = '';
    } catch (error) {
      const errorWithCode = error as Error & { code?: string };
      if (errorWithCode.code === 'TOTP_CONFIG_MISSING') {
        isConfigMissing = true;
        errorMessage = 'TOTP is not configured on the server yet.';
      } else {
        errorMessage = error instanceof Error ? error.message : 'Verification failed.';
      }
    } finally {
      isVerifying = false;
    }
  };

  const disableMfa = async () => {
    const trimmed = disableCode.replace(/\s+/g, '');
    if (!trimmed) {
      errorMessage = 'Authentication code is required.';
      return;
    }
    isDisabling = true;
    resetMessages();
    try {
      const response = await api.post<TotpDisableResponse>('/users/me/mfa/disable', { code: trimmed });
      successMessage = response.message || 'Multi-factor authentication disabled.';
      authStore.updateUser({ totpEnabled: false });
      disableCode = '';
      resetEnrollmentData();
    } catch (error) {
      const errorWithCode = error as Error & { code?: string };
      if (errorWithCode.code === 'TOTP_CONFIG_MISSING') {
        isConfigMissing = true;
        errorMessage = 'TOTP is not configured on the server yet.';
      } else {
        errorMessage = error instanceof Error ? error.message : 'Disable failed.';
      }
    } finally {
      isDisabling = false;
    }
  };
</script>

<section class="space-y-4">
  <div class="flex flex-wrap items-start justify-between gap-4">
    <div>
      <h3 class="text-sm font-semibold text-gray-900">Multi-factor authentication</h3>
      <p class="mt-1 text-sm text-gray-600">
        Add an authenticator app to secure your account. You'll scan a QR code and verify a 6-digit
        code.
      </p>
    </div>
    <div class="flex items-center gap-2">
      <span
        class={`inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-semibold ${
          $currentUser?.totpEnabled
            ? 'bg-emerald-100 text-emerald-700'
            : 'bg-gray-100 text-gray-600'
        }`}
      >
        <span
          class={`h-2 w-2 rounded-full ${
            $currentUser?.totpEnabled ? 'bg-emerald-500' : 'bg-gray-400'
          }`}
        ></span>
        {$currentUser?.totpEnabled ? 'Enabled' : 'Disabled'}
      </span>
      <button
        class="rounded-full border border-gray-200 bg-white px-4 py-2 text-xs font-semibold text-gray-600 transition hover:border-gray-300 hover:bg-gray-50 disabled:opacity-60"
        on:click={startEnrollment}
        disabled={isLoading || $currentUser?.totpEnabled}
        type="button"
      >
        {isLoading ? 'Generating...' : 'Start enrollment'}
      </button>
    </div>
  </div>

  {#if errorMessage}
    <div class="rounded-xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">
      {errorMessage}
    </div>
  {/if}

  {#if isConfigMissing}
    <div class="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
      <p class="font-semibold">Set up the TOTP encryption key to continue.</p>
      <p class="mt-2 text-amber-900/80">
        In your backend environment, set a base64-encoded 32-byte key:
      </p>
      <div class="mt-2 rounded-lg border border-amber-200 bg-white p-3 font-mono text-xs text-amber-900">
        CLUBHOUSE_TOTP_ENCRYPTION_KEY=&lt;base64-32-byte-key&gt;
      </div>
      <p class="mt-2 text-amber-900/80">
        Example generator: <code class="font-mono text-xs">openssl rand -base64 32</code>. Restart
        the backend and try again.
      </p>
    </div>
  {/if}

  {#if successMessage}
    <div class="rounded-xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-700">
      {successMessage}
    </div>
  {/if}

  {#if backupCodes.length > 0 && !backupDismissed}
    <div class="rounded-2xl border border-amber-200 bg-amber-50/70 p-5">
      <h4 class="text-sm font-semibold text-amber-900">Backup codes (save these now)</h4>
      <p class="mt-1 text-sm text-amber-900/80">
        These codes are shown once and let you sign in if you lose access to your authenticator.
        Store them securely before continuing.
      </p>
      <div class="mt-3 grid gap-2 sm:grid-cols-2">
        {#each backupCodes as backup}
          <div class="rounded-lg border border-amber-200 bg-white px-3 py-2 font-mono text-xs text-amber-900">
            {backup}
          </div>
        {/each}
      </div>
      <div class="mt-4 flex flex-wrap items-center gap-3">
        <label class="inline-flex items-center gap-2 text-sm text-amber-900">
          <input type="checkbox" bind:checked={backupConfirmed} class="h-4 w-4 rounded border-amber-300" />
          I saved these backup codes
        </label>
        <button
          class="rounded-full bg-amber-600 px-4 py-2 text-xs font-semibold text-white transition hover:bg-amber-700 disabled:opacity-60"
          type="button"
          on:click={() => (backupDismissed = true)}
          disabled={!backupConfirmed}
        >
          Continue
        </button>
      </div>
    </div>
  {/if}

  {#if qrCode || manualKey || otpauthUrl}
    <div class="grid gap-6 rounded-2xl border border-gray-100 bg-gray-50/70 p-6 md:grid-cols-[180px,1fr]">
      <div class="flex items-center justify-center rounded-xl border border-gray-200 bg-white p-4">
        {#if qrCode}
          <img src={qrCode} alt="MFA QR code" class="h-40 w-40 object-contain" />
        {:else}
          <p class="text-xs text-gray-500 text-center">
            QR code unavailable. Use the manual key to add this account.
          </p>
        {/if}
      </div>
      <div class="space-y-4">
        <div>
          <h4 class="text-sm font-semibold text-gray-900">Step 1: Scan the QR code</h4>
          <p class="mt-1 text-sm text-gray-600">
            Use Google Authenticator, 1Password, or any TOTP app to scan. If you can't scan, use the
            manual key below.
          </p>
        </div>
        {#if manualKey}
          <div class="rounded-xl border border-gray-200 bg-white p-4">
            <p class="text-xs font-mono uppercase tracking-widest text-gray-400">Manual entry key</p>
            <p class="mt-2 break-all text-sm font-semibold text-gray-900">{manualKey}</p>
          </div>
        {/if}
        {#if otpauthUrl}
          <div class="rounded-xl border border-gray-200 bg-white p-4">
            <p class="text-xs font-mono uppercase tracking-widest text-gray-400">Authenticator URL</p>
            <p class="mt-2 break-all text-xs text-gray-600">{otpauthUrl}</p>
          </div>
        {/if}
        <div class="rounded-xl border border-gray-200 bg-white p-4">
          <h4 class="text-sm font-semibold text-gray-900">Step 2: Verify the code</h4>
          <p class="mt-1 text-sm text-gray-600">
            Enter the 6-digit code from your authenticator app to finish enabling MFA.
          </p>
          <div class="mt-3 flex flex-wrap items-center gap-3">
            <input
              id="totp-code"
              type="text"
              inputmode="numeric"
              autocomplete="one-time-code"
              placeholder="123 456"
              bind:value={code}
              class="w-40 rounded-lg border border-gray-300 px-3 py-2 text-sm text-gray-900 focus:border-amber-400 focus:outline-none focus:ring-2 focus:ring-amber-200"
            />
            <button
              class="rounded-full bg-amber-600 px-4 py-2 text-xs font-semibold text-white transition hover:bg-amber-700 disabled:opacity-60"
              on:click={verifyEnrollment}
              disabled={isVerifying}
              type="button"
            >
              {isVerifying ? 'Verifying...' : 'Verify code'}
            </button>
            <button
              class="rounded-full border border-gray-200 bg-white px-4 py-2 text-xs font-semibold text-gray-600 transition hover:border-gray-300 hover:bg-gray-50"
              on:click={resetEnrollment}
              type="button"
            >
              Start over
            </button>
          </div>
        </div>
      </div>
    </div>
  {/if}


  {#if $currentUser?.totpEnabled}
    <div class="rounded-2xl border border-gray-200 bg-white p-5">
      <h4 class="text-sm font-semibold text-gray-900">Disable MFA</h4>
      <p class="mt-1 text-sm text-gray-600">
        Enter a 6-digit code from your authenticator app to disable MFA.
      </p>
      <div class="mt-3 flex flex-wrap items-center gap-3">
        <input
          id="totp-disable"
          type="text"
          inputmode="numeric"
          autocomplete="one-time-code"
          placeholder="123 456"
          bind:value={disableCode}
          class="w-40 rounded-lg border border-gray-300 px-3 py-2 text-sm text-gray-900 focus:border-rose-300 focus:outline-none focus:ring-2 focus:ring-rose-200"
        />
        <button
          class="rounded-full bg-rose-600 px-4 py-2 text-xs font-semibold text-white transition hover:bg-rose-700 disabled:opacity-60"
          on:click={disableMfa}
          disabled={isDisabling}
          type="button"
        >
          {isDisabling ? 'Disabling...' : 'Disable MFA'}
        </button>
      </div>
    </div>
  {/if}
</section>
