import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: 'dist',
    rollupOptions: {
      output: {
        manualChunks: {
          'three': ['three', '@react-three/fiber', '@react-three/drei'],
          'recharts': ['recharts'],
          'react': ['react', 'react-dom', 'react-router'],
        },
      },
    },
  },
  resolve: {
    dedupe: ['react', 'react-dom', 'three'],
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        ws: true,
      },
    },
  },
})
