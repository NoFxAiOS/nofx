import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [
    react(),
    // Add security headers plugin
    {
      name: 'security-headers',
      configureServer(server) {
        server.middlewares.use((req, res, next) => {
          // Prevent crawling and bot indexing
          res.setHeader('X-Robots-Tag', 'noindex, nofollow, noarchive, nosnippet')

          // Additional security headers
          res.setHeader('X-Content-Type-Options', 'nosniff')
          res.setHeader('X-Frame-Options', 'DENY')
          res.setHeader('X-XSS-Protection', '1; mode=block')
          res.setHeader('Referrer-Policy', 'strict-origin-when-cross-origin')

          next()
        })
      },
    },
  ],
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
