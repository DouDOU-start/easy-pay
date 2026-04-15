import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    host: true, // bind on 0.0.0.0 so Windows host can reach WSL2 services
    port: 5173,
    // WSL2 + /mnt/e (NTFS) doesn't deliver reliable inotify events,
    // so tell chokidar to poll for changes.
    watch: {
      usePolling: true,
      interval: 300,
    },
    proxy: {
      '/admin': 'http://localhost:8080',
    },
  },
})
