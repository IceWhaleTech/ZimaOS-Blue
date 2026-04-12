import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import Icons from 'unplugin-icons/vite'
import IconsResolver from 'unplugin-icons/resolver'
import { compression } from 'vite-plugin-compression2'
import { visualizer } from 'rollup-plugin-visualizer'

const generatedDtsEnabled = !process.env.VITEST
const moduleUiBuild = process.env.VITE_MODULE_UI === '1'
const moduleUiName = process.env.VITE_MODULE_NAME || 'zimaos-blue'
const publicBase = process.env.VITE_PUBLIC_BASE || (moduleUiBuild ? `/modules/${moduleUiName}/` : '/')
const outDir = process.env.VITE_OUT_DIR || 'dist'

const routeRuntimeBundles = [
  {
    name: 'route-assistant-runtime',
    modules: [
      '/src/api/chat.ts',
      '/src/api/deepResearch.ts',
      '/src/api/proxy.ts',
      '/src/api/skill.ts',
      '/src/api/speech.ts',
      '/src/api/tasks.ts',
      '/src/api/voice.ts',
      '/src/components/speech/ModelDownloadPrompt.vue',
      '/src/components/typeless/CardComparison.vue',
      '/src/components/typeless/CardProgress.vue',
      '/src/components/ui/SemanticSearchField.vue',
      '/src/utils/audioConverter.ts',
      '/src/utils/deepResearchText.ts',
      '/src/utils/taskProjectionText.ts',
      '/src/utils/toolLocalization.ts',
      '/src/utils/toolWarnings.ts',
      '/src/utils/ttsPreferences.ts',
      '/src/utils/wakeword.ts',
    ],
  },
  {
    name: 'route-dashboard-runtime',
    modules: [
      '/src/components/ResourceChart.vue',
    ],
  },
  {
    name: 'route-provider-runtime',
    modules: [
      '/src/api/providerPool.ts',
      '/src/stores/providerPool.ts',
    ],
  },
] as const

function getRouteRuntimeBundle(id: string): string | undefined {
  for (const bundle of routeRuntimeBundles) {
    if (bundle.modules.some((moduleId) => id.includes(moduleId))) {
      return bundle.name
    }
  }
}

// https://vite.dev/config/
export default defineConfig({
  define: {
    __EMBED_DISABLE_MERMAID__: JSON.stringify(process.env.VITE_EMBED_DISABLE_MERMAID === '1'),
  },
  base: publicBase,
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
      dts: generatedDtsEnabled ? 'src/auto-imports.d.ts' : false,
      dirs: ['src/composables', 'src/stores'],
      vueTemplate: true,
    }),
    // Auto import Vue components
    Components({
      dirs: ['src/components'],
      extensions: ['vue'],
      deep: true,
      dts: generatedDtsEnabled ? 'src/components.d.ts' : false,
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
      filename: `${outDir}/stats.html`,
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
        target: 'http://localhost',
        changeOrigin: true,
      },
      '/health': {
        target: 'http://localhost',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir,
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
        // Keep manualChunks scoped to the modules we explicitly assign,
        // instead of letting Rollup pull broad transitive dependencies into them.
        onlyExplicitManualChunks: true,
        // Experimental: native ES modules for better tree-shaking
        experimentalMinChunkSize: 5000, // 5KB minimum chunk size to reduce HTTP requests
        // Optimize chunk splitting for parallel downloads
        manualChunks: (id) => {
          const normalizedId = id.replace(/\\/g, '/')

          // Shared helper injected by Rollup's CommonJS bridge
          if (normalizedId.includes('commonjsHelpers')) {
            return 'shared-cjs'
          }
          // Route-level local helpers are reused by the same route families.
          // Merge them into higher-level runtime bundles to cut secondary fetches.
          const routeRuntimeBundle = getRouteRuntimeBundle(normalizedId)
          if (routeRuntimeBundle) {
            return routeRuntimeBundle
          }
          // VueUse utilities and their transitive deps
          if (normalizedId.includes('node_modules/@vueuse') ||
              normalizedId.includes('node_modules/perfect-debounce') ||
              normalizedId.includes('node_modules/hookable') ||
              normalizedId.includes('node_modules/birpc')) {
            return 'vueuse'
          }
          // vue-i18n and its runtime/compiler dependencies
          if (normalizedId.includes('node_modules/vue-i18n') ||
              normalizedId.includes('node_modules/@intlify/')) {
            return 'i18n-core'
          }
          // Vue Flow - heavy library, separate chunk (lazy loaded)
          if (normalizedId.includes('node_modules/@vue-flow')) {
            return 'vue-flow'
          }
          // Core Vue ecosystem - loaded first
          if (normalizedId.includes('node_modules/vue') ||
              normalizedId.includes('node_modules/@vue/') ||
              normalizedId.includes('node_modules/vue-router') ||
              normalizedId.includes('node_modules/pinia')) {
            return 'vue-core'
          }
          // Axios and HTTP utilities
          if (normalizedId.includes('node_modules/axios')) {
            return 'http'
          }
          // Rich text / markdown rendering stack should stay off the startup path
          if (normalizedId.includes('node_modules/highlight.js') ||
              normalizedId.includes('node_modules/decode-named-character-reference') ||
              normalizedId.includes('node_modules/katex') ||
              normalizedId.includes('node_modules/micromark') ||
              normalizedId.includes('node_modules/mdast-') ||
              normalizedId.includes('node_modules/remark-') ||
              normalizedId.includes('node_modules/rehype-') ||
              normalizedId.includes('node_modules/unified') ||
              normalizedId.includes('node_modules/trough') ||
              normalizedId.includes('node_modules/vfile') ||
              normalizedId.includes('node_modules/vfile-') ||
              normalizedId.includes('node_modules/unist-') ||
              normalizedId.includes('node_modules/hast-') ||
              normalizedId.includes('node_modules/property-information') ||
              normalizedId.includes('node_modules/space-separated-tokens') ||
              normalizedId.includes('node_modules/comma-separated-tokens') ||
              normalizedId.includes('node_modules/zwitch') ||
              normalizedId.includes('node_modules/bail') ||
              normalizedId.includes('node_modules/@braintree/sanitize-url') ||
              normalizedId.includes('node_modules/ts-dedent')) {
            return 'richtext'
          }
          // Mermaid is lazy-loaded via dynamic import() in CardMermaid.vue
          // Do NOT assign it to a named chunk — let Vite naturally code-split it
          // so it's only fetched when a mermaid card is actually rendered
          if (normalizedId.includes('node_modules/mermaid') ||
              normalizedId.includes('node_modules/d3') ||
              normalizedId.includes('node_modules/dagre') ||
              normalizedId.includes('node_modules/elkjs') ||
              normalizedId.includes('node_modules/cytoscape') ||
              normalizedId.includes('node_modules/dompurify') ||
              normalizedId.includes('node_modules/khroma') ||
              normalizedId.includes('node_modules/lodash') ||
              normalizedId.includes('node_modules/stylis')) {
            return undefined // let Vite handle splitting naturally
          }
          // Other vendor libraries
          if (normalizedId.includes('node_modules')) {
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
