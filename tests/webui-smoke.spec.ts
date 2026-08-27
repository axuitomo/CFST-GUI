import { expect, test } from "@playwright/test";

const schemaVersion = "phase1-bridge-v1";

test.beforeEach(async ({ page }) => {
  await page.route("**/api/**", async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === "/api/health") {
      await route.fulfill({ json: { auth_required: false } });
      return;
    }
    if (url.pathname === "/api/events/probe") {
      await route.fulfill({
        body: ": connected\n\n",
        contentType: "text/event-stream",
        headers: { "Cache-Control": "no-cache" },
      });
      return;
    }

    const method = decodeURIComponent(url.pathname.slice("/api/app/".length));
    const data =
      method === "GetAppInfo"
        ? {
            current_version: "1.8.9-test",
            install_mode: "webui",
            platform: "webui",
            release_url: "",
          }
        : {};
    await route.fulfill({
      json: {
        code: "TEST_OK",
        data,
        message: "",
        ok: true,
        schema_version: schemaVersion,
        task_id: null,
        warnings: [],
      },
    });
  });
});

test("loads the WebUI and navigates its primary sections", async ({ page }) => {
  await page.goto("/");

  await expect(page).toHaveTitle("CFST-GUI");
  await expect(
    page.getByRole("navigation", { name: "Desktop sections" }),
  ).toBeVisible();

  const sections = [
    ["任务看板", "任务看板"],
    ["当前结果", "当前测速结果"],
    ["输入源", "输入源管理"],
    ["系统配置", "系统配置"],
    ["DNS 读取", "DNS 记录读取"],
  ];
  for (const [buttonName, heading] of sections) {
    await page.getByRole("button", { name: buttonName, exact: true }).click();
    await expect(page.locator("header h1")).toHaveText(heading);
  }
});

test("keeps navigation available when config hydration fails", async ({
  page,
}) => {
  await page.unroute("**/api/**");
  await page.route("**/api/**", async (route) => {
    await route.fulfill({
      json: { message: "WebUI unavailable", ok: false },
      status: 404,
    });
  });

  await page.goto("/");
  await page.getByRole("button", { name: "输入源", exact: true }).click();
  await expect(page.locator("header h1")).toHaveText("输入源管理");

  await page.getByRole("button", { name: "系统配置", exact: true }).click();
  await expect(page.locator("header h1")).toHaveText("系统配置");
});
