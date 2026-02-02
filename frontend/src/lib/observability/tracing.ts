import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { resourceFromAttributes } from '@opentelemetry/resources';
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';

let initialized = false;

const DEFAULT_DEV_ENDPOINT = '/otlp/v1/traces';

const buildPropagationUrls = (): RegExp[] => {
  if (typeof window === 'undefined') {
    return [/^\/api\/v1\//];
  }

  const apiBase = new URL('/api/v1/', window.location.origin).toString();
  const escaped = apiBase.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return [new RegExp(`^${escaped}`), /^\/api\/v1\//];
};

export function initTracing(): void {
  if (initialized || typeof window === 'undefined') {
    return;
  }

  const exporterUrl =
    (import.meta.env.VITE_OTEL_EXPORTER_OTLP_ENDPOINT as string | undefined) ??
    (import.meta.env.DEV ? DEFAULT_DEV_ENDPOINT : undefined);

  if (!exporterUrl) {
    initialized = true;
    return;
  }

  const exporter = new OTLPTraceExporter({ url: exporterUrl });
  const provider = new WebTracerProvider({
    resource: resourceFromAttributes({
      'service.name':
        (import.meta.env.VITE_OTEL_SERVICE_NAME as string | undefined) ?? 'clubhouse-frontend',
      'service.version':
        (import.meta.env.VITE_APP_VERSION as string | undefined) ?? 'unknown',
      'deployment.environment': import.meta.env.MODE,
    }),
    spanProcessors: [new BatchSpanProcessor(exporter)],
  });

  provider.register();

  registerInstrumentations({
    instrumentations: [
      new FetchInstrumentation({
        propagateTraceHeaderCorsUrls: buildPropagationUrls(),
        clearTimingResources: true,
      }),
    ],
  });

  initialized = true;
}
