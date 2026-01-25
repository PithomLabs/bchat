import { defineConfig } from 'vite';
import { resolve } from 'path';

export default defineConfig({
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    lib: {
      // Build as self-executing IIFE
      entry: resolve(__dirname, 'src/embed.ts'),
      name: 'AgentChatWidget',
      formats: ['iife'],
      fileName: () => 'embed.min.js',
    },
    rollupOptions: {
      output: {
        // Ensure all code is bundled into single file
        inlineDynamicImports: true,
      },
    },
    // Minify for production (esbuild is bundled with Vite)
    minify: 'esbuild',
    // Target modern browsers
    target: 'es2020',
    // No sourcemaps in production
    sourcemap: false,
  },
  // For dev server testing
  server: {
    port: 3002,
    open: '/src/test.html',
    proxy: {
      // Proxy API calls to backend during development
      '/api': {
        target: 'http://localhost:8081',
        changeOrigin: true,
      },
    },
  },
});
