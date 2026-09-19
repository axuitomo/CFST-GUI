import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";
import tailwindcss from "@tailwindcss/vite";
import vue from "@vitejs/plugin-vue";

// Vue 3.6 Vapor：默认 `vue` 入口仅含 VDOM 运行时，不导出 Vapor API
// （defineVaporComponent / template / createVaporApp 等），而 Vapor SFC 的
// 编译产物仍从裸 `vue` 导入这些 API。因此需把 `vue` 精确别名到
// runtime-with-vapor 构建：dev 用带告警的开发版，build 用精简生产版。
// Vitest 在 Node 下运行且不挂载 SFC，保持默认 bundler 入口。
const isVitest = !!process.env.VITEST;
const resolveVueEntry = (command: "serve" | "build") => fileURLToPath(new URL(command === "build" ? "./node_modules/vue/dist/vue.runtime-with-vapor.esm-browser.prod.js" : "./node_modules/vue/dist/vue.runtime-with-vapor.esm-browser.js", import.meta.url));

export default defineConfig(({ command }) => ({
  // features.vapor：强制所有 <script setup> SFC 以 Vapor 模式编译
  plugins: [vue({ features: { vapor: true } }), tailwindcss()],
  clearScreen: false,
  build: {
    chunkSizeWarningLimit: 4096,
  },
  resolve: {
    tsconfigPaths: true,
    alias: isVitest
      ? []
      : [
          {
            // 仅替换裸 `vue`，避免误伤 vue/compiler-sfc、vue/server-renderer 等子路径
            find: /^vue$/,
            replacement: resolveVueEntry(command),
          },
        ],
  },
  server: {
    // wails3 dev 会导出 WAILS_VITE_PORT 并把 FRONTEND_DEVSERVER_URL 指向同一个
    // 端口；桌面开发模式下的原生窗口正是通过该环境变量代理到 Vite 的。直接运行
    // pnpm dev 时该变量为空，回退到 34117，Playwright 也依赖这个默认端口。
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 34117,
    strictPort: true,
  },
}));
