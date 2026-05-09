import path from 'node:path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// In dev, the Vite server proxies /api and /ws to the Go backend on :8443.
// `make dev-api` runs the backend in CR_DEV mode (plain HTTP, non-Secure
// cookies) so cookies survive the http://localhost:5173 → http://localhost:8443
// hop without needing a real cert.
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://localhost:8443',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8443',
        changeOrigin: true,
        ws: true,
      },
    },
  },
  build: {
    outDir: './dist',
    emptyOutDir: true,
    target: 'es2022',
    sourcemap: false,
  },
})
