import { configDefaults, defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config.ts";

export default defineConfig((env) =>
  mergeConfig(viteConfig(env), {
    test: { exclude: [...configDefaults.exclude, "**/*.browser.test.ts"] },
  }),
);
