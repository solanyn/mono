import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/copilotkit': 'http://localhost:8000',
    },
  },
  build: {
    rollupOptions: {
      external: [
        /^vscode-/,
        /^langium/,
      ],
    },
  },
})
