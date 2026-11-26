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
    toHaveScreenshot: {
      animations: "disabled",
      caret: "hide",
      scale: "css",
      maxDiffPixels: 0,
      threshold: 0.1,
      pathTemplate: "{testDir}/__screenshots__/{projectName}-{platform}/{arg}{ext}",
    },
    toMatchAriaSnapshot: {
      children: "deep-equal",
      pathTemplate: "{testDir}/__aria__/{arg}{ext}",
    },
  },
  use: {
    baseURL: "http://127.0.0.1:34117",
    trace: "on-first-retry",
    // 视觉基线的前提是「同一份代码在哪里跑都栅格化成同一张图」:关掉 LCD 亚像素、
    // 字体 hinting 与子像素定位并固定 sRGB,否则同一张页面在本机与 CI 会差出成百上千像素,
    // 基线就只能由 CI 单方面生成。
    launchOptions: {
      args: ["--disable-lcd-text", "--disable-font-subpixel-positioning", "--font-render-hinting=none", "--force-color-profile=srgb"],
    },
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
    command: "node frontend/node_modules/vite/bin/vite.js frontend --host 127.0.0.1",
    url: "http://127.0.0.1:34117",
    reuseExistingServer: !process.env.CI,
  },
});
