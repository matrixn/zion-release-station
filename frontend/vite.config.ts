import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  plugins: [vue()],
  base: '/releasestation/',
  server: {
    port: 5173,
    proxy: {
      '/releasestation/api': 'http://127.0.0.1:24871',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    sourcemap: false,
  },
});
