import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import Icons from 'unplugin-icons/vite'
import IconsResolver from 'unplugin-icons/resolver'
import { compression } from 'vite-plugin-compression2'
import { visualizer } from 'rollup-plugin-visualizer'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    // Auto import Vue APIs and composables
    AutoImport({
      imports: [
        'vue',
        'vue-router',
        'pinia',
        {
          '@vueuse/core': [
            'useStorage',
            'useLocalStorage',
            'useDark',
            'useToggle',
            'useEventListener',
            'useClipboard',
            'useDebounce',
            'useThrottle',
          ],
        },
      ],
      dts: 'src/auto-imports.d.ts',
      dirs: ['src/composables', 'src/stores'],
      vueTemplate: true,
    }),
    // Auto import Vue components
    Components({
      dirs: ['src/components'],
      extensions: ['vue'],
      deep: true,
      dts: 'src/components.d.ts',
      resolvers: [
        // Auto import icons as components
        IconsResolver({
          prefix: 'Icon',
          enabledCollections: ['lucide', 'mdi', 'heroicons'],
        }),
      ],
    }),
    // Icons on-demand loading
    Icons({
      compiler: 'vue3',
      autoInstall: true,
    }),
    // Gzip compression for production
    compression({
      algorithms: ['gzip'],
      exclude: [/\.(br)$/, /\.(gz)$/],
      threshold: 1024,
    }),
    // Brotli compression for better compression ratio
    compression({
      algorithms: ['brotliCompress'],
      exclude: [/\.(br)$/, /\.(gz)$/],
      threshold: 1024,
    }),
    // Bundle analyzer (generates stats.html)
    visualizer({
      filename: 'dist/stats.html',
      open: false,
      gzipSize: true,
      brotliSize: true,
    }),
  ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:23456',
        changeOrigin: true,
      },
      '/health': {
        target: 'http://localhost:23456',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
    // Put all assets in root directory instead of assets/
    assetsDir: '',
    // Minify with esbuild for faster builds
    minify: 'esbuild',
    // Target modern browsers for smaller bundle
    target: 'es2020',
    // CSS code splitting
    cssCodeSplit: true,
    // Chunk size warning limit
    chunkSizeWarningLimit: 500,
    // Module preload polyfill
    modulePreload: {
      polyfill: false, // Modern browsers support modulepreload natively
    },
    rollupOptions: {
      output: {
        // Use content hash for cache busting
        entryFileNames: '[name]-[hash].js',
        chunkFileNames: '[name]-[hash].js',
        assetFileNames: '[name]-[hash].[ext]',
        // Experimental: native ES modules for better tree-shaking
        experimentalMinChunkSize: 5000, // 5KB minimum chunk size to reduce HTTP requests
        // Optimize chunk splitting for parallel downloads
        manualChunks: (id) => {
          // Core Vue ecosystem - loaded first
          if (id.includes('node_modules/vue') ||
              id.includes('node_modules/@vue') ||
              id.includes('node_modules/vue-router') ||
              id.includes('node_modules/pinia')) {
            return 'vue-core'
          }
          // Vue Flow - heavy library, separate chunk (lazy loaded)
          if (id.includes('node_modules/@vue-flow')) {
            return 'vue-flow'
          }
          // vue-i18n library only (not locale files)
          if (id.includes('node_modules/vue-i18n')) {
            return 'i18n-core'
          }
          // VueUse utilities
          if (id.includes('node_modules/@vueuse')) {
            return 'vueuse'
          }
          // Axios and HTTP utilities
          if (id.includes('node_modules/axios')) {
            return 'http'
          }
          // Mermaid and ALL its dependencies must be in the same chunk
          // to avoid "Cannot access 'x' before initialization" errors
          if (id.includes('node_modules/mermaid') ||
              id.includes('node_modules/d3') ||
              id.includes('node_modules/dagre') ||
              id.includes('node_modules/elkjs') ||
              id.includes('node_modules/cytoscape') ||
              id.includes('node_modules/dompurify') ||
              id.includes('node_modules/khroma') ||
              id.includes('node_modules/lodash') ||
              id.includes('node_modules/stylis')) {
            return 'mermaid'
          }
          // Other vendor libraries
          if (id.includes('node_modules')) {
            return 'vendor'
          }
        },
      },
    },
  },
  // Optimize dependencies
  optimizeDeps: {
    include: ['vue', 'vue-router', 'pinia', 'axios'],
    exclude: ['@vue-flow/core', 'mermaid'],
  },
  test: {
    globals: true,
    environment: 'happy-dom',
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
    },
  },
})
