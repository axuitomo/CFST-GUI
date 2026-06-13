import { defineConfig } from "vite";
import tailwindcss from "@tailwindcss/vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  clearScreen: false,
  build: {
    chunkSizeWarningLimit: 4096,
  },
  resolve: {
    tsconfigPaths: true,
  },
  server: {
    port: 34117,
    strictPort: true,
  },
});
