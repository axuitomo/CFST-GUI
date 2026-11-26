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
      // 小字号文本的边缘像素在两端机器上仍有约 1% 的差异(字体版本/系统 hinting 决定,
      // 工具栏改版这类真实改动实测 ≥ 2.5%)。留 1.5% 的容差,基线才能在本机生成后直接比对;
      // 不能同时给 maxDiffPixels(它会与比例取较小值,等于把容差压回 0)。
      maxDiffPixelRatio: 0.015,
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
