import { onCLS, onFCP, onINP, onLCP, onTTFB, type Metric } from 'web-vitals';
import { logWarn } from './logger';

const METRICS_ENDPOINT = '/api/v1/metrics/vitals';
const MAX_QUEUE_SIZE = 200;
const MAX_METRICS_PER_REQUEST = 50;
const FLUSH_INTERVAL_MS = 5000;
const DEFAULT_SAMPLE_RATE = 0.2;

type MetricType =
  | 'web_vital'
  | 'api_timing'
  | 'websocket_connect'
  | 'asset_load'
  | 'component_render';

interface FrontendMetric {
  type: MetricType;
  name?: string;
  value?: number;
  unit?: string;
  rating?: string;
  delta?: number;
  id?: string;
  navigationType?: string;
  endpoint?: string;
  method?: string;
  status?: number;
  durationMs?: number;
  resourceType?: string;
  component?: string;
  outcome?: string;
}

const metricQueue: FrontendMetric[] = [];
let flushTimer: ReturnType<typeof setTimeout> | null = null;
let initialized = false;

function isBrowser(): boolean {
  return typeof window !== 'undefined';
}

function getSampleRate(): number {
  const envValue = Number(import.meta.env.VITE_PERF_SAMPLE_RATE ?? DEFAULT_SAMPLE_RATE);
  if (Number.isNaN(envValue) || envValue <= 0) {
    return 0;
  }
  if (envValue >= 1) {
    return 1;
  }
  return envValue;
}

function shouldSample(): boolean {
  return Math.random() < getSampleRate();
}

function scheduleFlush(): void {
  if (flushTimer) {
    return;
  }
  flushTimer = setTimeout(() => {
    flushTimer = null;
    void flushMetrics();
  }, FLUSH_INTERVAL_MS);
}

function enqueueMetric(metric: FrontendMetric): void {
  if (!isBrowser()) {
    return;
  }
  if (metricQueue.length >= MAX_QUEUE_SIZE) {
    metricQueue.shift();
  }
  metricQueue.push(metric);
  if (metricQueue.length >= MAX_METRICS_PER_REQUEST) {
    void flushMetrics();
    return;
  }
  scheduleFlush();
}

function sendPayload(payload: { metrics: FrontendMetric[] }): void {
  if (!isBrowser()) {
    return;
  }

  const body = JSON.stringify(payload);
  if (navigator.sendBeacon) {
    const blob = new Blob([body], { type: 'application/json' });
    const ok = navigator.sendBeacon(METRICS_ENDPOINT, blob);
    if (ok) {
      return;
    }
  }

  void fetch(METRICS_ENDPOINT, {
    method: 'POST',
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
    },
    keepalive: true,
    body,
  }).catch(() => {
    // Swallow errors to avoid disrupting user flows.
  });
}

async function flushMetrics(): Promise<void> {
  if (!isBrowser()) {
    return;
  }
  if (metricQueue.length === 0) {
    return;
  }

  const batch = metricQueue.splice(0, MAX_METRICS_PER_REQUEST);
  sendPayload({ metrics: batch });

  if (metricQueue.length > 0) {
    scheduleFlush();
  }
}

function normalizeEndpoint(endpoint: string): string {
  const withoutQuery = endpoint.split('?')[0] ?? endpoint;
  return withoutQuery.replace(
    /[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}/g,
    ':id'
  );
}

function sanitizeResourceName(resourceUrl: string): string {
  try {
    const url = new URL(resourceUrl, window.location.href);
    return url.pathname || resourceUrl;
  } catch {
    return resourceUrl.split('?')[0] ?? resourceUrl;
  }
}

function initResourceTiming(): void {
  if (!isBrowser() || !('PerformanceObserver' in window)) {
    return;
  }

  const observedTypes = new Set(['img', 'script', 'link', 'css', 'font']);
  const observer = new PerformanceObserver((list) => {
    for (const entry of list.getEntries()) {
      if (entry.entryType !== 'resource') {
        continue;
      }
      const resource = entry as PerformanceResourceTiming;
      if (!observedTypes.has(resource.initiatorType)) {
        continue;
      }
      if (!Number.isFinite(resource.duration) || resource.duration < 0) {
        continue;
      }
      enqueueMetric({
        type: 'asset_load',
        name: sanitizeResourceName(resource.name),
        resourceType: resource.initiatorType,
        durationMs: resource.duration,
      });
    }
  });

  try {
    observer.observe({ type: 'resource', buffered: true });
  } catch {
    try {
      observer.observe({ entryTypes: ['resource'] });
    } catch {
      logWarn('PerformanceObserver resource timing unavailable');
    }
  }
}

function reportWebVital(metric: Metric): void {
  const unit = metric.name === 'CLS' ? 'score' : 'ms';
  enqueueMetric({
    type: 'web_vital',
    name: metric.name,
    value: metric.value,
    delta: metric.delta,
    rating: metric.rating,
    id: metric.id,
    navigationType: metric.navigationType,
    unit,
  });
}

export function initPerformanceMonitoring(): void {
  if (!isBrowser() || initialized) {
    return;
  }
  initialized = true;

  onCLS(reportWebVital);
  onINP(reportWebVital);
  onLCP(reportWebVital);
  onTTFB(reportWebVital);
  onFCP(reportWebVital);

  initResourceTiming();

  if (typeof window !== 'undefined') {
    window.addEventListener('beforeunload', () => {
      void flushMetrics();
    });
  }
}

export function recordApiTiming(
  endpoint: string,
  method: string,
  status: number,
  durationMs: number
): void {
  enqueueMetric({
    type: 'api_timing',
    endpoint: normalizeEndpoint(endpoint),
    method,
    status,
    durationMs,
  });
}

export function recordWebsocketConnect(outcome: string, durationMs: number): void {
  enqueueMetric({
    type: 'websocket_connect',
    outcome,
    durationMs,
  });
}

export function recordComponentRender(component: string, durationMs: number): void {
  if (!shouldSample()) {
    return;
  }
  enqueueMetric({
    type: 'component_render',
    component,
    durationMs,
  });
}
