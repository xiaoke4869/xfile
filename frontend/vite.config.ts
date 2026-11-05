import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath } from 'node:url'
export default defineConfig({ plugins: [vue()], base: './', server:{port:5173,strictPort:true}, build:{rollupOptions:{input:{main:fileURLToPath(new URL('./index.html',import.meta.url)),icons:fileURLToPath(new URL('./file-icons.html',import.meta.url))}}} })
