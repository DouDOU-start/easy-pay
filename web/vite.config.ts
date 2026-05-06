import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    host: true,
    port: 5173,
    watch: {
      usePolling: true,
      interval: 300,
    },
    proxy: {
      '/api': 'http://localhost:8080',
      '/setup': 'http://localhost:8080',
      '/callback': 'http://localhost:8080',
      '/health': 'http://localhost:8080',
    },
  },
})
