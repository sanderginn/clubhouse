import { captureError, captureMessage } from './errorTracker';

const shouldLogToConsole = import.meta.env.DEV;

type LogContext = Record<string, unknown> | undefined;

function formatContext(context: LogContext): Record<string, unknown> | undefined {
  if (!context || Object.keys(context).length === 0) {
    return undefined;
  }
  return context;
}

export function logInfo(message: string, context?: LogContext): void {
  const extra = formatContext(context);
  if (shouldLogToConsole) {
    if (extra) {
      console.info(message, extra);
    } else {
      console.info(message);
    }
  }
}

export function logWarn(message: string, context?: LogContext): void {
  const extra = formatContext(context);
  if (shouldLogToConsole) {
    if (extra) {
      console.warn(message, extra);
    } else {
      console.warn(message);
    }
  }

  captureMessage(message, {
    level: 'warning',
    extra,
  });
}

export function logError(message: string, context?: LogContext, error?: unknown): void {
  const extra = formatContext(context);
  if (shouldLogToConsole) {
    if (extra) {
      console.error(message, extra, error);
    } else {
      console.error(message, error);
    }
  }

  captureError(error ?? new Error(message), {
    level: 'error',
    extra,
  });
}
