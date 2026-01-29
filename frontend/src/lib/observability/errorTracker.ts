import * as Sentry from '@sentry/browser';

type ErrorLevel = 'info' | 'warning' | 'error' | 'fatal';

interface CaptureContext {
  tags?: Record<string, string>;
  extra?: Record<string, unknown>;
  level?: ErrorLevel;
}

const dsn = import.meta.env.VITE_SENTRY_DSN as string | undefined;
const isBrowser = typeof window !== 'undefined';
let initialized = false;

export function initErrorTracker(): void {
  if (!isBrowser || initialized || !dsn) {
    return;
  }

  Sentry.init({
    dsn,
    environment: import.meta.env.MODE,
    release: import.meta.env.VITE_APP_VERSION as string | undefined,
    integrations: [],
  });

  initialized = true;
}

export function captureError(error: unknown, context?: CaptureContext): void {
  if (!dsn) {
    return;
  }

  if (!initialized) {
    initErrorTracker();
  }

  Sentry.withScope((scope) => {
    if (context?.tags) {
      scope.setTags(context.tags);
    }
    if (context?.extra) {
      scope.setExtras(context.extra);
    }
    if (context?.level) {
      scope.setLevel(context.level);
    }

    if (error instanceof Error) {
      Sentry.captureException(error);
      return;
    }

    Sentry.captureMessage(typeof error === 'string' ? error : 'Unknown error');
  });
}

export function captureMessage(message: string, context?: CaptureContext): void {
  if (!dsn) {
    return;
  }

  if (!initialized) {
    initErrorTracker();
  }

  Sentry.withScope((scope) => {
    if (context?.tags) {
      scope.setTags(context.tags);
    }
    if (context?.extra) {
      scope.setExtras(context.extra);
    }
    if (context?.level) {
      scope.setLevel(context.level);
    }

    Sentry.captureMessage(message);
  });
}

export function setErrorUser(user: { id: string; username?: string; email?: string } | null): void {
  if (!dsn) {
    return;
  }

  if (!initialized) {
    initErrorTracker();
  }

  if (!user) {
    Sentry.setUser(null);
    return;
  }

  Sentry.setUser({
    id: user.id,
    username: user.username,
    email: user.email,
  });
}
