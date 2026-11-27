import { expect, test } from "@playwright/test";

// 桌面结果表格窗口化：只渲染滚动窗口内的行，用上下占位行撑起整份历史的滚动高度。
const schemaVersion = "phase1-bridge-v1";
const HOUR = 60 * 60 * 1000;
const HISTORY_COUNT = 12000;

function historyRecords() {
  const now = Date.now();
  return Array.from({ length: HISTORY_COUNT }, (_, i) => ({
    address: `1.2.${Math.floor(i / 250)}.${i % 250}`,
    batch_id: `batch-${i % 20}`,
    colo: "HKG",
    download_mbps: 50 + (i % 300),
    max_download_mbps: 80 + (i % 400),
    source_port: null,
    stage_status: "completed",
    tcp_latency_ms: 20 + (i % 200),
    test_port: 443,
    timestamp: now - HOUR - i * 1000,
    trace_latency_ms: 30 + (i % 150),
  }));
}

test("desktop results table renders only the scroll window of a large history", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "chromium", "窗口化依赖 Chromium 的 sticky 表头与滚动容器行为");

  const storedRecords = historyRecords();
  await page.route((url) => url.pathname.startsWith("/api/"), async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === "/api/health") {
      await route.fulfill({ json: { auth_required: false, ok: true, service: "cfst-webui" } });
      return;
    }
    if (url.pathname === "/api/events/probe") {
      await route.fulfill({ body: ": connected\n\n", contentType: "text/event-stream" });
      return;
    }
    const command = decodeURIComponent(url.pathname.split("/").pop() || "");
    let data: unknown = {};
    if (command === "GetAppInfo") {
      data = { current_version: "test", install_mode: "webui", platform: "webui", release_url: "" };
    } else if (command === "history.query") {
      data = { records: storedRecords, total_count: storedRecords.length };
    } else if (command === "history.import") {
      data = { records: storedRecords, total_count: storedRecords.length, imported_count: storedRecords.length };
    }
    await route.fulfill({ json: { code: "TEST_OK", data, message: "", ok: true, schema_version: schemaVersion, task_id: null, warnings: [] } });
  });

  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/");
  await page.getByRole("navigation").getByRole("button", { name: "当前结果", exact: true }).click();
  await page.getByRole("combobox", { name: "时间范围" }).selectOption("all");

  const rows = page.locator("tbody tr.desktop-data-row");
  const scrollContainer = page.locator(".table-scroll.desktop-data-table-scroll");
  await expect(rows.first()).toBeVisible();

  // DOM 只保留窗口内的行，历史规模不影响节点数。
  await expect.poll(() => rows.count()).toBeLessThanOrEqual(40);
  // 占位行把整份历史撑开，滚动高度接近“行数 × 行高”。
  expect(await scrollContainer.evaluate((element) => element.scrollHeight)).toBeGreaterThan(HISTORY_COUNT * 40);

  const firstAddress = await rows.first().innerText();
  await scrollContainer.evaluate((element) => {
    element.scrollTop = element.scrollHeight;
  });
  await expect.poll(async () => (await rows.first().innerText()) !== firstAddress).toBe(true);
  await expect.poll(() => rows.count()).toBeLessThanOrEqual(40);
});
