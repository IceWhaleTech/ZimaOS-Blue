import { defineConfig } from 'vite'

export default defineConfig({
  build: {
    // 优化构建输出
    minify: 'terser',
    terserOptions: {
      compress: {
        drop_console: true,
        drop_debugger: true,
      },
      format: {
        comments: false,
      },
    },
    // 优化代码分割
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['@tauri-apps/api'],
        },
      },
    },
    // 优化构建性能
    chunkSizeWarningLimit: 1000,
    reportCompressedSize: false,
  },
  server: {
    middlewareMode: true,
  },
  // 优化依赖预构建
  optimizeDeps: {
    include: ['@tauri-apps/api'],
  },
})
