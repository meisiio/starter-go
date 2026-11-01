import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://localhost:8086',
      // health probes live outside /api/v1 (GEN-10)
      '/health': 'http://localhost:8086',
      '/ready': 'http://localhost:8086',
    },
  },
})
