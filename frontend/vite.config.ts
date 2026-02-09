import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import path from 'path';

type ProxyLogLevel = 'none' | 'silent' | 'error' | 'warn' | 'info' | 'debug';

const DEFAULT_PROXY_LOG_LEVEL: ProxyLogLevel = 'warn';
const proxyLogLevelRaw = process.env.VITE_PROXY_LOG_LEVEL?.toLowerCase() ?? DEFAULT_PROXY_LOG_LEVEL;
const proxyLogLevel: ProxyLogLevel =
  proxyLogLevelRaw === 'none' ||
  proxyLogLevelRaw === 'silent' ||
  proxyLogLevelRaw === 'error' ||
  proxyLogLevelRaw === 'warn' ||
  proxyLogLevelRaw === 'info' ||
  proxyLogLevelRaw === 'debug'
    ? proxyLogLevelRaw
    : DEFAULT_PROXY_LOG_LEVEL;

const proxyLogOrder: Record<ProxyLogLevel, number> = {
  none: -1,
  silent: -1,
  error: 0,
  warn: 1,
  info: 2,
  debug: 3,
};

const shouldProxyLog = (level: ProxyLogLevel) =>
  proxyLogOrder[proxyLogLevel] >= 0 && proxyLogOrder[level] <= proxyLogOrder[proxyLogLevel];

const logProxy = (level: ProxyLogLevel, ...args: unknown[]) => {
  if (!shouldProxyLog(level)) {
    return;
  }

  switch (level) {
    case 'error':
      console.error(...args);
      break;
    case 'warn':
      console.warn(...args);
      break;
    case 'info':
      console.info(...args);
      break;
    case 'debug':
      console.debug(...args);
      break;
    default:
      console.log(...args);
      break;
  }
};

export default defineConfig({
  plugins: [svelte()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    exclude: ['**/node_modules/**', '**/e2e/**'],
  },
  resolve: {
    alias: {
      $lib: path.resolve('./src/lib'),
    },
  },
  server: {
    allowedHosts: ["localhost", ".ngrok-free.app"],
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.VITE_API_PROXY_TARGET ?? 'http://localhost:8080',
        changeOrigin: true,
        ws: true,
        secure: false,
        configure: (proxy, options) => {
          // HTTP request/response logging
          proxy.on('error', (err, req, res) => {
            logProxy('error', '[Vite Proxy] Error:', err.message);
          });
          proxy.on('proxyReq', (proxyReq, req, res) => {
            logProxy('info', '[Vite Proxy] Request:', req.method, req.url);
          });
          proxy.on('proxyRes', (proxyRes, req, res) => {
            logProxy('info', '[Vite Proxy] Response:', proxyRes.statusCode, req.url);
          });

          // WebSocket-specific logging
          proxy.on('upgrade', (req, socket, head) => {
            logProxy('info', '[Vite Proxy] WebSocket upgrade request:', req.url);
          });
          proxy.on('proxyReqWs', (proxyReq, req, socket, options, head) => {
            logProxy('debug', '[Vite Proxy] WebSocket proxying to backend:', req.url);
          });
          proxy.on('open', (proxySocket) => {
            logProxy('info', '[Vite Proxy] WebSocket connection opened to backend');
            proxySocket.on('data', (chunk) => {
              logProxy('debug', '[Vite Proxy] WebSocket data from backend');
            });
          });
          proxy.on('close', (res, socket, head) => {
            logProxy('info', '[Vite Proxy] WebSocket connection closed');
          });
        },
      },
      '/otlp': {
        target: process.env.VITE_OTEL_PROXY_TARGET ?? 'http://localhost:4318',
        changeOrigin: true,
        secure: false,
        rewrite: (path) => path.replace(/^\/otlp/, ''),
      },
    },
  },
});
