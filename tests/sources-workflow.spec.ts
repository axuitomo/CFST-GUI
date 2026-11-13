import { expect, test } from "@playwright/test";
import { normalizeSourceConfig } from "../frontend/src/lib/bridge/config";
import { sourceBackend } from "./helpers/source-backend";

for (const mobile of [false, true]) {
  test(`adds typed sources, checks locally and reuses the group (${mobile ? "mobile" : "desktop"})`, async ({
    page,
  }) => {
    if (mobile) await page.setViewportSize({ width: 390, height: 844 });
    const backend = await sourceBackend(page);
    await page.goto("/");
    await page.getByRole("button", { name: mobile ? "来源" : "输入源", exact: true }).click();
    await page.getByRole("button", { name: "展开输入组设置" }).click();
    const add = page.getByRole("combobox", { name: "添加输入源" }).first();
    await add.selectOption("inline");
    const content = page.locator("textarea");
    await expect(content).toBeVisible();
    await expect(
      page.getByText("请粘贴 IP、CIDR 或域名，每行一条。", { exact: true }),
    ).toBeVisible();
    await content.fill("1.1.1.1\nbad");
    await page.getByRole("button", { name: "检查并预览", exact: true }).click();
    await expect(page.getByText("已读取 1 条候选。", { exact: false })).toBeVisible();
    await expect(
      page.getByText("忽略 1 条非法输入，请检查原始名单。", { exact: true }),
    ).toBeVisible();
    await content.fill("1.0.0.1");
    await expect(page.getByText("内容已填写，尚未检查。", { exact: false })).toHaveCount(0);
    await expect(page.getByText("已读取 1 条候选。", { exact: false })).toHaveCount(0);

    await page.getByRole("combobox", { name: "输入组 IP 模式" }).selectOption("mcis");
    await add.selectOption("url");
    const url = page.getByPlaceholder("https://example.com/ips.txt 或 example.com/ips.txt");
    await url.fill("ftp://example.com/ips");
    await expect(
      page.getByText("链接必须是有效的 HTTP 或 HTTPS 地址。", { exact: true }),
    ).toBeVisible();
    await expect(
      url.locator("xpath=ancestor::article").getByRole("button", { name: "检查并预览" }),
    ).toBeDisabled();
    await url.fill("https://unavailable.example/ips.txt");
    await url
      .locator("xpath=ancestor::article")
      .getByRole("button", { name: "检查并预览" })
      .click();
    await expect(page.getByText("来源服务器返回 503", { exact: false })).toBeVisible();
    await url.fill("example.com/ips.txt");
    await expect(page.getByText("来源服务器返回 503", { exact: false })).toHaveCount(0);

    await page.getByRole("button", { name: "复制当前组并编辑" }).click();
    await expect.poll(() => backend.profiles.items.length).toBe(2);
    await expect.poll(() => backend.profiles.active_profile_id).not.toBe("original");
    expect(backend.profiles.items[0].sources).toHaveLength(2);
    expect(backend.profiles.items[0].sources.every((source) => source.ip_mode === "mcis")).toBe(
      true,
    );
    await expect(page.getByRole("combobox", { name: "输入组 IP 模式" })).toHaveValue("mcis");
    await expect.poll(() => backend.snapshot().source_ip_mode).toBe("mcis");
    await add.selectOption("file");
    const filePath = page.getByPlaceholder("/data/cfips/ip.txt");
    await expect(filePath).toBeVisible();
    await filePath.fill("copy-only.txt");
    await page.getByRole("button", { name: "展开输入组管理", exact: true }).click();
    await page.getByRole("button", { name: "更新并保存当前输入组" }).click();
    await expect.poll(() => backend.profiles.items[1].sources.length).toBe(3);
    expect(backend.profiles.items[0].sources).toHaveLength(2);
    expect(backend.profiles.items[1].sources[2].ip_mode).toBe("mcis");
    await page.screenshot({
      path: `test-results/sources-${mobile ? "mobile" : "desktop"}.png`,
      fullPage: true,
    });
  });
}

test("retains mixed legacy modes until the user unifies them", async ({ page }) => {
  const backend = await sourceBackend(page, [
    normalizeSourceConfig({ id: "a", kind: "inline", content: "1.1.1.1", ip_mode: "mcis" }, 0),
    normalizeSourceConfig({ id: "b", kind: "inline", content: "1.0.0.1", ip_mode: "traverse" }, 1),
  ]);
  await page.goto("/");
  await page.getByRole("button", { name: "输入源", exact: true }).click();
  await page.getByRole("button", { name: "展开输入组设置" }).click();
  await expect(page.getByText("旧配置中的来源模式不同", { exact: false })).toBeVisible();
  expect(backend.profiles.items[0].sources.map((source) => source.ip_mode)).toEqual([
    "mcis",
    "traverse",
  ]);
  await page.getByRole("combobox", { name: "输入组 IP 模式" }).selectOption("traverse");
  await page.getByRole("button", { name: "展开输入组管理", exact: true }).click();
  await page.getByRole("button", { name: "更新并保存当前输入组" }).click();
  await expect
    .poll(() => backend.profiles.items[0].sources.map((source) => source.ip_mode))
    .toEqual(["traverse", "traverse"]);
  await expect(page.getByText("旧配置中的来源模式不同", { exact: false })).toHaveCount(0);
});

test("applies COLO filtering from the input group to source previews", async ({ page }) => {
  const backend = await sourceBackend(page);
  await page.goto("/");
  await expect(page.locator(".ui-toast").filter({ hasText: "配置已加载。" })).toBeVisible();
  await page.getByRole("button", { name: "输入源", exact: true }).click();
  await page.getByRole("button", { name: "展开输入组设置" }).click();
  await page.getByPlaceholder("JP,HKG,NRT,US,UK").first().fill("JP,HKG");
  await page.getByRole("button", { name: "黑名单", exact: true }).first().click();
  await expect(page.getByRole("button", { name: "黑名单", exact: true }).first()).toHaveClass(
    /bg-white/,
  );
  await page.getByRole("combobox", { name: "添加输入源" }).first().selectOption("inline");
  await page.locator("textarea").first().fill("1.1.1.1");
  await expect(page.getByRole("button", { name: "黑名单", exact: true }).first()).toHaveClass(
    /bg-white/,
  );
  await page.getByRole("button", { name: "检查并预览", exact: true }).first().click();
  await expect.poll(() => backend.preview()?.colo_filter).toBe("JP,HKG");
  expect(backend.preview()?.colo_filter_mode).toBe("deny");
  await expect.poll(() => backend.profiles.items[0].colo_filter).toBe("JP,HKG");
  expect(backend.profiles.items[0].colo_filter_mode).toBe("deny");
});

test("retains empty-group edits when an older config save completes", async ({ page }) => {
  const backend = await sourceBackend(page);
  await page.goto("/");
  await expect(page.locator(".ui-toast").filter({ hasText: "配置已加载。" })).toBeVisible();
  await page.getByRole("button", { name: "输入源", exact: true }).click();
  await page.getByRole("button", { name: "展开输入组设置" }).click();

  let releaseSave: () => void = () => {};
  const heldSave = new Promise<void>((resolve) => {
    releaseSave = resolve;
  });
  let requested = false;
  await page.route(
    "**/api/command/config.save",
    async (route) => {
      const data = {
        config_snapshot: route.request().postDataJSON().config_snapshot,
        source_profiles: structuredClone(backend.profiles),
      };
      requested = true;
      await heldSave;
      await route.fulfill({ json: { ok: true, code: "TEST_OK", data } });
    },
    { times: 1 },
  );
  try {
    await page.getByPlaceholder("JP,HKG,NRT,US,UK").fill("JP,HKG");
    await page.getByRole("combobox", { name: "输入组 IP 模式" }).focus();
    await page.getByRole("combobox", { name: "输入组 IP 模式" }).selectOption("mcis");
    await page.getByRole("button", { name: "白名单", exact: true }).click();
    await expect.poll(() => requested).toBe(true);
    await page.getByRole("button", { name: "黑名单", exact: true }).click();
    const completed = page.waitForResponse((response) => response.url().endsWith("/config.save"));
    releaseSave();
    await completed;
    await page.getByRole("combobox", { name: "添加输入源" }).first().selectOption("inline");
    await page.locator("textarea").fill("1.1.1.1");
    await expect(page.getByRole("button", { name: "黑名单", exact: true })).toHaveClass(/bg-white/);
    await page.getByRole("button", { name: "检查并预览", exact: true }).click();
    await expect.poll(() => backend.preview()?.colo_filter_mode).toBe("deny");
    expect(backend.preview()?.colo_filter).toBe("JP,HKG");
    await expect.poll(() => backend.profiles.items[0]?.colo_filter_mode).toBe("deny");
    expect(backend.profiles.items[0]?.colo_filter).toBe("JP,HKG");
  } finally {
    releaseSave();
  }
});

test("ignores a preview response after its input changes", async ({ page }) => {
  await sourceBackend(page);
  let releasePreview: () => void = () => {};
  const holdPreview = new Promise<void>((resolve) => {
    releasePreview = resolve;
  });
  let requested = false;
  await page.route("**/api/command/source.preview", async (route) => {
    requested = true;
    await holdPreview;
    await route.fulfill({
      json: {
        ok: true,
        data: { summary: { total_count: 999, invalid_count: 0 }, preview_entries: ["1.1.1.1"] },
      },
    });
  });
  await page.goto("/");
  await page.getByRole("button", { name: "输入源", exact: true }).click();
  await page.getByRole("combobox", { name: "添加输入源" }).first().selectOption("inline");
  await page.locator("textarea").fill("1.1.1.1");
  await page.getByRole("button", { name: "检查并预览", exact: true }).click();
  await expect.poll(() => requested).toBe(true);
  await page.locator("textarea").fill("1.0.0.1");
  const completed = page.waitForResponse((response) => response.url().endsWith("/source.preview"));
  releasePreview();
  await completed;
  await expect(page.getByRole("button", { name: "检查并预览", exact: true })).toBeEnabled();
  await expect(page.getByText("内容已填写，尚未检查。", { exact: false })).toHaveCount(0);
  await expect(page.getByText("已读取 999", { exact: false })).toHaveCount(0);
});
