import { expect, test } from "@playwright/test";

const schemaVersion = "phase1-bridge-v1";

test.beforeEach(async ({ page }) => {
  await page.route("**/api/**", async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === "/api/health") {
      // 必须带上服务自述字段：前端用 service=cfst-webui 认定这是 CFST WebUI 服务（并据此
      // 决定要不要提示访问令牌）。缺这个字段会被当成普通静态页面：通道仍走 WebUI，但带
      // 令牌的部署无法弹出令牌输入。
      await route.fulfill({ json: { auth_required: false, ok: true, service: "cfst-webui" } });
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

for (const platform of ["android", "webui"] as const) {
  test(`uses the expected navigation transition on ${platform}`, async ({ page }) => {
    await page.route("**/api/platform/GetAppInfo", async (route) => {
      await route.fulfill({
        json: {
          code: "TEST_OK",
          data: {
            current_version: "test",
            install_mode: "webui",
            platform,
            release_url: "",
          },
          message: "",
          ok: true,
          schema_version: schemaVersion,
          task_id: null,
          warnings: [],
        },
      });
    });
    await page.addInitScript(() => {
      Object.defineProperty(document, "startViewTransition", {
        configurable: true,
        value: (callback: () => void) => {
          document.documentElement.dataset.transitionCalled = "true";
          callback();
          return { finished: Promise.resolve() };
        },
      });
    });
    const appInfo = page.waitForResponse((response) => response.url().includes("/api/platform/GetAppInfo"));
    await page.goto("/");
    await appInfo;
    await page.getByRole("button", { name: "当前结果", exact: true }).click();
    await expect(page.locator("header h1")).toHaveText("当前测速结果");
    if (platform === "android") {
      await expect(page.locator("html")).not.toHaveAttribute("data-transition-called", "true");
    } else {
      await expect(page.locator("html")).toHaveAttribute("data-transition-called", "true");
    }
    await page.getByRole("button", { name: "系统配置", exact: true }).click();
    await expect(page.locator("header h1")).toHaveText("系统配置");
    await expect(page.getByRole("switch", { name: "高级设置" })).toHaveCount(0);
    for (const domain of ["通用设置", "数据与存储", "网络与任务", "自动化与导出", "安全与诊断"]) {
      await expect(page.getByRole("heading", { name: domain })).toBeVisible();
    }
  });
}

test("builds settings section content on first expand", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "系统配置", exact: true }).click();
  await expect(page.getByRole("heading", { name: "通用设置" })).toBeVisible();
  const sectionContent = page.locator(".settings-view-root details > div");
  expect(await sectionContent.count()).toBe(0);
  await page.locator(".settings-view-root summary").first().click();
  await expect.poll(() => sectionContent.count()).toBeGreaterThan(0);
});
test("adds sources from the group header and remembers collapsed group settings", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/");
  await page.getByRole("button", { name: "输入源", exact: true }).click();
  await expect(page.locator("header h1")).toHaveText("输入源管理");

  const expandGroup = page.getByRole("button", { name: "展开输入组设置" });
  await expect(expandGroup).toHaveAttribute("aria-expanded", "false");
  await expandGroup.click();
  await expect(page.getByLabel("输入组 IP 模式")).toBeVisible();
  await page.getByRole("button", { name: "折叠输入组设置" }).click();
  await page.reload();
  await page.getByRole("button", { name: "输入源", exact: true }).click();
  await expect(page.locator("header h1")).toHaveText("输入源管理");
  await expect(page.getByRole("button", { name: "展开输入组设置" })).toHaveAttribute(
    "aria-expanded",
    "false",
  );

  await page.getByLabel("添加输入源").first().selectOption("inline");
  await expect(page.locator("textarea")).toBeVisible();
});

for (const reducedMotion of ["no-preference", "reduce"] as const) {
  test(`resets mobile scroll on navigation (${reducedMotion})`, async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 500 });
    await page.emulateMedia({ reducedMotion });
    await page.goto("/");
    await page.getByRole("button", { name: "配置", exact: true }).click();
    const main = page.locator(".mobile-main");
    await expect(page.getByRole("heading", { name: "通用设置" })).toBeVisible();
    await main.evaluate((element) => {
      element.scrollTop = 600;
    });
    await expect.poll(() => main.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
    await page.getByRole("button", { name: "配置", exact: true }).click();
    await expect.poll(() => main.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
    await page.getByRole("button", { name: "来源", exact: true }).click();
    await expect(page.getByRole("heading", { name: "输入组" })).toBeVisible();
    await expect.poll(() => main.evaluate((element) => element.scrollTop)).toBe(0);
  });
}

test("keeps general layout transitions disabled on phone while animating group settings", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page.getByRole("button", { name: "来源", exact: true }).click();
  await expect(page.getByRole("heading", { name: "输入组" })).toBeVisible();

  await page.getByLabel("添加输入源").first().selectOption("inline");
  const content = page.locator("textarea");
  await expect(content).toBeVisible();
  await content.fill("1.1.1.1");
  await expect(page.getByText(/内容已填写，尚未检查/)).toHaveCount(0);

  const transitionDurations = await page.locator("body").evaluate((body) => {
    const general = document.createElement("div");
    general.className = "ui-reveal-enter-active";
    const group = document.createElement("div");
    group.className = "source-group-enter-active";
    body.append(general, group);
    const groupStyle = getComputedStyle(group);
    const durations = [getComputedStyle(general).transitionDuration, groupStyle.transitionDuration, groupStyle.transitionProperty];
    general.remove();
    group.remove();
    return durations;
  });
  expect(transitionDurations[0]).toBe("0s");
  expect(transitionDurations[1]).toContain("0.18s");
  expect(transitionDurations[2]).toBe("opacity, transform");
  const scrollbar = await page.locator("html").evaluate((element) => getComputedStyle(element).scrollbarWidth);
  expect(scrollbar).toBe("none");
  await expect(page.locator(".mobile-main")).toHaveCSS("overflow-y", "auto");
  await expect(page.locator("html")).toHaveJSProperty("scrollWidth", 390);
});


test("keeps navigation available when config hydration fails", async ({ page }) => {
  await page.unroute("**/api/**");
  await page.route("**/api/**", async (route) => {
    const url = new URL(route.request().url());
    // 健康检查仍然可用：本用例覆盖的是业务命令失败后导航不受影响，而不是后端整体不可达。
    if (url.pathname === "/api/health") {
      await route.fulfill({ json: { auth_required: false, ok: true, service: "cfst-webui" } });
      return;
    }
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
