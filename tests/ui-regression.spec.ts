import { expect, test, type Page } from "@playwright/test";
import { normalizeSourceConfig } from "../frontend/src/lib/bridge/config";
import { sourceBackend } from "./helpers/source-backend";

const sourceURL = "https://example.com/regression-ips.txt";
const sections = [
  { id: "dashboard", desktop: "任务看板", mobile: "看板", title: "任务看板" },
  { id: "results", desktop: "当前结果", mobile: "结果", title: "当前测速结果" },
  { id: "sources", desktop: "输入源", mobile: "来源", title: "输入源管理" },
  { id: "settings", desktop: "系统配置", mobile: "配置", title: "系统配置" },
  { id: "dns", desktop: "DNS 读取", mobile: "DNS", title: "DNS 记录读取" },
] as const;

test.afterEach(async ({ page }) => {
  expect(await page.pageErrors(), "The page must not report uncaught runtime errors").toEqual([]);
});

async function navigate(page: Page, id: (typeof sections)[number]["id"], mobile: boolean) {
  const section = sections.find((item) => item.id === id)!;
  await page
    .getByRole("navigation")
    .getByRole("button", { name: mobile ? section.mobile : section.desktop, exact: true })
    .click();
  await expect(page.locator("header")).toContainText(section.title);
  await expect(page.locator(".theme-shell")).toHaveCount(1);
}

for (const mobile of [false, true]) {
  const layout = mobile ? "phone" : "desktop";
  test.describe(layout, () => {
    test.use({
      viewport: mobile ? { width: 390, height: 844 } : { width: 1280, height: 900 },
      colorScheme: "light",
      reducedMotion: "reduce",
      locale: "zh-CN",
      timezoneId: "UTC",
    });
    test.beforeEach(async ({ page }) => {
      await page.clock.setFixedTime(new Date("2026-11-13T04:00:00Z"));
      await sourceBackend(page, [
        normalizeSourceConfig(
          {
            id: "url-regression",
            name: "回归测试远程源",
            kind: "url",
            url: sourceURL,
            enabled: true,
          },
          0,
        ),
      ]);
      await page.goto("/");
      await expect(page.locator("#cfst-boot")).toHaveCount(0);
      await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
      await expect(page.locator(".ui-toast")).toHaveCount(0);
    });

    for (const theme of ["light", "dark"] as const) {
      test(`@visual captures all sections in ${theme}`, async ({ page }, testInfo) => {
        test.skip(
          testInfo.project.name !== "chromium",
          "Pixel baselines use Chromium; DOM and Aria checks run in every engine.",
        );
        await page.emulateMedia({ colorScheme: theme });
        await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
        for (const section of sections) {
          await test.step(section.id, async () => {
            await navigate(page, section.id, mobile);
            await page.mouse.move(0, 0);
            await expect(page).toHaveScreenshot(`${layout}-${section.id}-${theme}.png`);
          });
        }
      });
    }

    test("preserves page semantics across navigation and live theme changes", async ({ page }) => {
      for (const section of sections) {
        await navigate(page, section.id, mobile);
        await expect(page.locator("#app")).toMatchAriaSnapshot({
          name: `${layout}-${section.id}.aria.yml`,
        });
        const panel = page
          .locator(
            section.id === "settings" ? ".settings-domain .bg-white" : ".ui-card, .ui-card-subtle",
          )
          .first();
        await expect(panel).toBeVisible();
        const light = await panel.evaluate((element) => getComputedStyle(element).backgroundColor);
        await page.emulateMedia({ colorScheme: "dark" });
        await expect(page.locator("html")).toHaveClass(/dark/);
        await expect
          .poll(() => panel.evaluate((element) => getComputedStyle(element).backgroundColor))
          .not.toBe(light);
        await expect
          .poll(() => page.evaluate(() => localStorage.getItem("cfst.surface-theme")))
          .toBe("dark");
        await page.emulateMedia({ colorScheme: "light" });
        await expect(page.locator("html")).not.toHaveClass(/dark/);
        await expect
          .poll(() => panel.evaluate((element) => getComputedStyle(element).backgroundColor))
          .toBe(light);
        await expect(page.locator(".theme-shell")).toHaveCount(1);
        if (mobile) await expect(page.locator("html")).toHaveJSProperty("scrollWidth", 390);
      }
    });

    test("hides collapsed URLs and updates the existing editor without losing its draft", async ({
      page,
    }) => {
      await navigate(page, "sources", mobile);
      const card = page
        .locator("article")
        .filter({ has: page.getByRole("heading", { name: "回归测试远程源" }) });
      await expect(card).not.toContainText(sourceURL);
      await expect(page.locator(`input[value="${sourceURL}"]`)).toHaveCount(0);
      await page.getByLabel("回归测试远程源 更多操作").click();
      await page.getByRole("button", { name: "编辑", exact: true }).click();
      const field = page.getByPlaceholder("https://example.com/ips.txt 或 example.com/ips.txt");
      await expect(field).toHaveValue(sourceURL);
      await expect(page.getByRole("heading", { name: "回归测试远程源" })).toHaveCount(0);
      await expect(page.getByRole("button", { name: "删除输入源" })).toBeVisible();
      const enabled = page.getByRole("switch", { name: "停用输入源" });
      await expect(enabled).toBeChecked();
      await enabled.uncheck();
      await expect(page.getByRole("switch", { name: "启用输入源" })).not.toBeChecked();
      await page.getByRole("switch", { name: "启用输入源" }).check();
      await expect(field.locator("xpath=ancestor::article")).toMatchAriaSnapshot({
        name: `${layout}-source-editor.aria.yml`,
      });
      await field.fill(`${sourceURL}?draft=1`);
      await field.evaluate((element) => {
        element.dataset.regressionDraft = "kept";
      });
      await page.emulateMedia({ colorScheme: "dark" });
      await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
      await expect(field).toHaveValue(`${sourceURL}?draft=1`);
      await expect(field).toHaveAttribute("data-regression-draft", "kept");
      await page.getByRole("button", { name: "收起", exact: true }).click();
      await expect(field).toHaveCount(0);
      await expect(page.getByRole("heading", { name: "回归测试远程源" })).toHaveCount(1);
      await expect(page.locator("article").filter({ hasText: "回归测试远程源" })).not.toContainText(
        sourceURL,
      );
    });

    test("keeps an explicit theme when the system changes and restores it after reload", async ({
      page,
    }) => {
      await navigate(page, "settings", mobile);
      const appearance = page
        .locator("details")
        .filter({ has: page.getByRole("heading", { name: "外观与自动主题", exact: true }) });
      if ((await appearance.getAttribute("open")) === null)
        await appearance.locator("summary").click();
      const mode = page.getByRole("combobox", { name: "主题模式" });
      const savedTheme = page.waitForResponse(
        (response) =>
          response.url().endsWith("/config.save") &&
          response.request().postDataJSON()?.config_snapshot?.ui?.theme_mode === "dark",
      );
      await mode.focus();
      await mode.selectOption("dark");
      await mode.blur();
      await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
      await page.emulateMedia({ colorScheme: "light" });
      await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
      await savedTheme;
      await page.reload();
      await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
      await navigate(page, "settings", mobile);
      await expect(mode).toHaveValue("dark");
      await mode.selectOption("auto_system_time");
      await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
    });
  });
}

test("mounts only one shell while resizing across the desktop breakpoint", async ({ page }) => {
  await sourceBackend(page);
  await page.goto("/");
  for (const width of [1023, 1024, 390, 1280]) {
    await page.setViewportSize({ width, height: 844 });
    await expect(page.locator(".theme-shell")).toHaveCount(1);
    await expect(
      page.locator(width >= 1024 ? "nav[aria-label='Desktop sections']" : "nav.theme-nav"),
    ).toBeVisible();
    await expect(page.locator(".dashboard-workbench")).toHaveCount(1);
    await expect(page.getByRole("navigation").getByRole("button")).toHaveCount(5);
  }
});
