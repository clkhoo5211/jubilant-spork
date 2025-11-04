import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Determine base path for GitHub Pages
const base = process.env.GITHUB_PAGES === 'true' || process.env.CI === 'true' 
  ? '/jubilant-spork/' 
  : '/'

export default defineConfig({
  plugins: [react()],
  base: base,
  server: {
    host: '0.0.0.0',
    port: 4000,
    proxy: {
      '/api': {
        target: process.env.VITE_API_BASE_URL || 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
  },
})
