import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // During `npm run dev`, the frontend runs on its own port (5173).
    // Any request to /api/* gets forwarded to the Go server on :8080,
    // so the React app can just call fetch("/api/snippets") without
    // worrying about which port it's really talking to.
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
