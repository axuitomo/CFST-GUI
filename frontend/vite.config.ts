import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  plugins: [vue()],
  clearScreen: false,
  build: {
    chunkSizeWarningLimit: 4096,
  },
  server: {
    port: 34115,
    strictPort: true,
  },
});
