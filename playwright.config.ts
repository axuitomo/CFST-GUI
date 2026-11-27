import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  fullyParallel: true,
  updateSnapshots: "none",
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI ? "line" : "html",
  expect: {
    // 视觉回归只断言与字体无关的几何关系(见 tests/ui-regression.spec.ts 的 @layout);
    // 整页像素基线已移除:两端 Segoe UI 版本不同,字形本身就有差异,基线无法在本机生成。
    toMatchAriaSnapshot: {
      children: "deep-equal",
      pathTemplate: "{testDir}/__aria__/{arg}{ext}",
    },
  },
  use: {
    baseURL: "http://127.0.0.1:34117",
    trace: "on-first-retry",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
    {
      name: "firefox",
      use: { ...devices["Desktop Firefox"] },
    },
    {
      name: "webkit",
      use: { ...devices["Desktop Safari"] },
    },
  ],
  webServer: {
    command: "node packages/frontend/node_modules/vite/bin/vite.js packages/frontend --host 127.0.0.1",
    url: "http://127.0.0.1:34117",
    reuseExistingServer: !process.env.CI,
  },
});
