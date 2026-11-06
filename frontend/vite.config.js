import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(),
  ],
  server: {
    proxy: {
      '/api': {
        target: process.env.VITE_API_PROXY || 'http://127.0.0.1:8080',
        ws: true,
      },
      '/healthz': process.env.VITE_API_PROXY || 'http://127.0.0.1:8080',
      '/readyz': process.env.VITE_API_PROXY || 'http://127.0.0.1:8080',
    },
  },
})
