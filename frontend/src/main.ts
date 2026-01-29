import App from './App.svelte';
import { initErrorTracker, captureError } from './lib/observability/errorTracker';
import { setFatalError } from './lib/observability/errorState';
import { initPerformanceMonitoring } from './lib/observability/performance';
import { initTracing } from './lib/observability/tracing';

initErrorTracker();
initPerformanceMonitoring();
initTracing();

if (typeof window !== 'undefined') {
  window.onerror = (message, source, lineno, colno, error) => {
    const errorMessage = typeof message === 'string' ? message : 'Unhandled error';
    captureError(error ?? errorMessage, {
      level: 'error',
      tags: { source: 'window.onerror' },
      extra: {
        message: errorMessage,
        source,
        lineno,
        colno,
      },
    });
    setFatalError({
      message: errorMessage,
      error,
      source: 'window',
      timestamp: new Date(),
    });
    return false;
  };

  window.addEventListener('unhandledrejection', (event) => {
    const reason = event.reason ?? 'Unhandled promise rejection';
    const reasonMessage = reason instanceof Error ? reason.message : String(reason);
    captureError(reason, {
      level: 'error',
      tags: { source: 'unhandledrejection' },
      extra: {
        message: reasonMessage,
      },
    });
    setFatalError({
      message: reasonMessage,
      error: reason,
      source: 'unhandledrejection',
      timestamp: new Date(),
    });
  });
}

const app = new App({
  target: document.getElementById('app')!,
});

export default app;
