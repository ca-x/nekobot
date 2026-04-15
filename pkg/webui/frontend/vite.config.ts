import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

const apiBase = process.env.VITE_API_BASE ?? 'http://127.0.0.1:18080';
const wsBase = apiBase.replace(/^http/i, 'ws');

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) {
            return;
          }

          if (id.includes('/@shader-gradient/') || id.includes('/camera-controls/')) {
            return 'shader-gradient-vendor';
          }

          if (id.includes('/three/')) {
            return 'three-vendor';
          }

          if (id.includes('/@xterm/') || id.includes('/xterm/')) {
            return 'xterm-vendor';
          }
        },
      },
    },
  },
  server: {
    host: '127.0.0.1',
    port: 5174,
    strictPort: true,
    proxy: {
      '/api': {
        target: apiBase,
        changeOrigin: true,
        ws: true,
      },
      '/ws': {
        target: wsBase,
        ws: true,
      },
    },
  },
});
