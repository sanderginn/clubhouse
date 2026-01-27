<script lang="ts">
  import { api } from '../../services/api';

  interface TotpEnrollResponse {
    qr_code?: string;
    qr_code_data_url?: string;
    qr_code_svg?: string;
    secret?: string;
    manual_entry_key?: string;
    otpauth_url?: string;
  }

  interface TotpVerifyResponse {
    message: string;
  }

  let isLoading = false;
  let isVerifying = false;
  let errorMessage = '';
  let successMessage = '';
  let isConfigMissing = false;
  let code = '';
  let qrCode = '';
  let manualKey = '';
  let otpauthUrl = '';

  const normalizeQr = (response: TotpEnrollResponse) => {
    const raw = response.qr_code ?? response.qr_code_data_url ?? response.qr_code_svg ?? '';
    if (!raw) return '';
    if (raw.startsWith('data:')) return raw;
    if (raw.trim().startsWith('<svg')) {
      return `data:image/svg+xml;utf8,${encodeURIComponent(raw)}`;
    }
    const base64Like = /^[A-Za-z0-9+/=]+$/.test(raw);
    return base64Like ? `data:image/png;base64,${raw}` : raw;
  };

  const startEnrollment = async () => {
    isLoading = true;
    errorMessage = '';
    successMessage = '';
    isConfigMissing = false;
    try {
      const response = await api.post<TotpEnrollResponse>('/admin/totp/enroll');
      qrCode = normalizeQr(response);
      manualKey = response.manual_entry_key ?? response.secret ?? '';
      otpauthUrl = response.otpauth_url ?? '';
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
    errorMessage = '';
    successMessage = '';
    isConfigMissing = false;
    try {
      const response = await api.post<TotpVerifyResponse>('/admin/totp/verify', { code: trimmed });
      successMessage = response.message || 'Multi-factor authentication enabled.';
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

  const resetEnrollment = () => {
    qrCode = '';
    manualKey = '';
    otpauthUrl = '';
    code = '';
    errorMessage = '';
    successMessage = '';
    isConfigMissing = false;
  };
</script>

<section class="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
  <div class="flex flex-wrap items-start justify-between gap-4">
    <div>
      <p class="text-xs uppercase tracking-[0.3em] text-slate-400 font-mono">Security</p>
      <h2 class="text-2xl font-serif font-semibold text-slate-900">Admin MFA</h2>
      <p class="mt-2 text-sm text-slate-600">
        Add an authenticator app to protect admin logins. You'll scan a QR code, then verify a
        6-digit code to enable MFA.
      </p>
    </div>
    <button
      class="rounded-full border border-slate-200 bg-white px-4 py-2 text-xs font-semibold text-slate-600 transition hover:border-slate-300 hover:bg-slate-50 disabled:opacity-60"
      on:click={startEnrollment}
      disabled={isLoading}
    >
      {isLoading ? 'Generating...' : 'Start enrollment'}
    </button>
  </div>

  {#if errorMessage}
    <div class="mt-6 rounded-xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">
      {errorMessage}
    </div>
  {/if}

  {#if isConfigMissing}
    <div class="mt-4 rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
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
    <div class="mt-6 rounded-xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-700">
      {successMessage}
    </div>
  {/if}

  {#if qrCode || manualKey || otpauthUrl}
    <div class="mt-6 grid gap-6 rounded-2xl border border-slate-100 bg-slate-50/70 p-6 md:grid-cols-[180px,1fr]">
      <div class="flex items-center justify-center rounded-xl border border-slate-200 bg-white p-4">
        {#if qrCode}
          <img src={qrCode} alt="TOTP QR code" class="h-40 w-40 object-contain" />
        {:else}
          <p class="text-xs text-slate-500 text-center">
            QR code unavailable. Use the manual key to add this account.
          </p>
        {/if}
      </div>
      <div class="space-y-4">
        <div>
          <h3 class="text-sm font-semibold text-slate-900">Step 1: Scan the QR code</h3>
          <p class="mt-1 text-sm text-slate-600">
            Use Google Authenticator, 1Password, or any TOTP app to scan. If you can't scan, use
            the manual key below.
          </p>
        </div>
        {#if manualKey}
          <div class="rounded-xl border border-slate-200 bg-white p-4">
            <p class="text-xs font-mono uppercase tracking-widest text-slate-400">Manual entry key</p>
            <p class="mt-2 break-all text-sm font-semibold text-slate-900">{manualKey}</p>
          </div>
        {/if}
        {#if otpauthUrl}
          <div class="rounded-xl border border-slate-200 bg-white p-4">
            <p class="text-xs font-mono uppercase tracking-widest text-slate-400">Authenticator URL</p>
            <p class="mt-2 break-all text-xs text-slate-600">{otpauthUrl}</p>
          </div>
        {/if}
        <div class="rounded-xl border border-slate-200 bg-white p-4">
          <h3 class="text-sm font-semibold text-slate-900">Step 2: Verify the code</h3>
          <p class="mt-1 text-sm text-slate-600">
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
              class="w-40 rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-900 focus:border-amber-400 focus:outline-none focus:ring-2 focus:ring-amber-200"
            />
            <button
              class="rounded-full bg-amber-600 px-4 py-2 text-xs font-semibold text-white transition hover:bg-amber-700 disabled:opacity-60"
              on:click={verifyEnrollment}
              disabled={isVerifying}
            >
              {isVerifying ? 'Verifying...' : 'Verify code'}
            </button>
            <button
              class="rounded-full border border-slate-200 bg-white px-4 py-2 text-xs font-semibold text-slate-600 transition hover:border-slate-300 hover:bg-slate-50"
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
</section>
