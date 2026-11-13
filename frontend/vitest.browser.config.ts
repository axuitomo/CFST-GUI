import { fileURLToPath, URL } from "node:url";
import { defineConfig, mergeConfig } from "vitest/config";
import { playwright } from "@vitest/browser-playwright";
import viteConfig from "./vite.config.ts";

export default defineConfig((env) =>
  mergeConfig(viteConfig(env), {
    resolve: {
      alias: [{ find: /^vue$/, replacement: fileURLToPath(new URL("./node_modules/vue/dist/vue.runtime-with-vapor.esm-browser.js", import.meta.url)) }],
    },
    test: {
      include: ["src/**/*.browser.test.ts"],
      browser: {
        enabled: true,
        provider: playwright(),
        headless: true,
        instances: [{ browser: "chromium" }],
        viewport: { width: 390, height: 844 },
      },
    },
  }),
);
