import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react-swc'

export default defineConfig({
  base: '/',
  plugins: [react()],
  build: {
    target: 'esnext', // Set for modern browsers compatibility
    outDir: 'dist', // Specify the output directory
    sourcemap: false, // Disable source map generation
  },
})
