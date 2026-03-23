import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return

          if (id.includes('react-dom') || id.includes('/react/')) {
            return 'react-vendor'
          }
          if (id.includes('recharts') || id.includes('d3-')) {
            return 'chart-vendor'
          }
          if (id.includes('lightweight-charts')) {
            return 'trading-chart-vendor'
          }
          if (id.includes('katex')) {
            return 'katex-vendor'
          }
          if (id.includes('framer-motion')) {
            return 'motion-vendor'
          }
          if (id.includes('lucide-react')) {
            return 'icons-vendor'
          }
          if (id.includes('swr') || id.includes('axios')) {
            return 'data-vendor'
          }
          if (id.includes('@radix-ui') || id.includes('sonner')) {
            return 'ui-vendor'
          }

          return 'vendor'
        },
      },
    },
  },
  server: {
    host: '0.0.0.0',
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
