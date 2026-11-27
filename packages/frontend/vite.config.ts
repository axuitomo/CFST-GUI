import { readFileSync } from "node:fs";
import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";
import tailwindcss from "@tailwindcss/vite";
import vue from "@vitejs/plugin-vue";

// Vue 3.6 Vapor：从 3.6.0-rc.13 起，裸 `vue` 的默认入口（runtime.esm-bundler）同时
// 再导出 `@vue/runtime-dom` 与 `@vue/runtime-vapor`，Vapor SFC 的编译产物可以直接从
// 裸 `vue` 导入 defineVaporComponent / createVaporApp 等 API，不必再别名到预构建的
// esm-browser 包。走默认入口还有两个好处：打包器能按实际引用摇掉未用到的运行时（预构建
// 包是整块交付），dev / build / Vitest 也统一到同一个入口，冷启动少解析约 35 KB。
// 升级 vue 时必须确认该入口仍导出 Vapor API，否则 Vapor SFC 会在运行时拿到 undefined
// （scripts/checks/vapor-mode.mjs 守这条不变量）。

export default defineConfig(() => ({
  // features.vapor：强制所有 <script setup> SFC 以 Vapor 模式编译
  plugins: [
    vue({ features: { vapor: true } }),
    tailwindcss(),
    {
      name: "distribution-legal-files",
      generateBundle() {
        for (const fileName of ["LICENSE", "THIRD-PARTY-NOTICES.md"]) {
          this.emitFile({ type: "asset", fileName, source: readFileSync(new URL(`../../${fileName}`, import.meta.url), "utf8") });
        }
      },
    },
  ],
  clearScreen: false,
  build: {
    rolldownOptions: {
      input: {
        app: fileURLToPath(new URL("./index.html", import.meta.url)),
        animationPreview: fileURLToPath(new URL("./animation-preview.html", import.meta.url)),
      },
    },
    chunkSizeWarningLimit: 4096,
  },
  resolve: {
    tsconfigPaths: true,
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
