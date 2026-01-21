import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import path from 'path';

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
            console.log('[Vite Proxy] Error:', err.message);
          });
          proxy.on('proxyReq', (proxyReq, req, res) => {
            console.log('[Vite Proxy] Request:', req.method, req.url);
          });
          proxy.on('proxyRes', (proxyRes, req, res) => {
            console.log('[Vite Proxy] Response:', proxyRes.statusCode, req.url);
          });

          // WebSocket-specific logging
          proxy.on('upgrade', (req, socket, head) => {
            console.log('[Vite Proxy] WebSocket upgrade request:', req.url);
          });
          proxy.on('proxyReqWs', (proxyReq, req, socket, options, head) => {
            console.log('[Vite Proxy] WebSocket proxying to backend:', req.url);
          });
          proxy.on('open', (proxySocket) => {
            console.log('[Vite Proxy] WebSocket connection opened to backend');
            proxySocket.on('data', (chunk) => {
              console.log('[Vite Proxy] WebSocket data from backend');
            });
          });
          proxy.on('close', (res, socket, head) => {
            console.log('[Vite Proxy] WebSocket connection closed');
          });
        },
      },
    },
  },
});
