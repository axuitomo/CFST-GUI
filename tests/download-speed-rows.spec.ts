import { expect, test } from "@playwright/test";

const schemaVersion = "phase1-bridge-v1";

test("keeps every concurrent IP row inside the Android 200ms throttle window", async ({ page }) => {
  test.setTimeout(30000);

  // 必须同时 stub 健康检查与 platform 自述：前端据此选择 WebUI 通道，并按 platform=android
  // 打开 200ms 进度节流。只写 /api/app/GetAppInfo 不会命中真实端点，测试会退化成桌面路径。
  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (url.pathname === "/api/health") {
      await route.fulfill({ json: { auth_required: false, ok: true, service: "cfst-webui" } });
      return;
    }
    if (url.pathname === "/api/platform/GetAppInfo") {
      await route.fulfill({
        json: {
          code: "TEST_OK",
          data: {
            current_version: "test",
            install_mode: "webui",
            platform: "android",
            release_url: "",
          },
          message: "",
          ok: true,
          schema_version: schemaVersion,
          task_id: null,
          warnings: [],
        },
      });
      return;
    }
    if (url.pathname === "/api/command/config.load") {
      await route.fulfill({
        json: {
          code: "TEST_OK",
          data: {
            config_snapshot: {
              probe: {
                concurrency: { stage1: 200, stage2: 6, stage3: 4 },
                download_speed_sample_interval_ms: 1000,
                strategy: "full",
              },
            },
          },
          message: "",
          ok: true,
          schema_version: schemaVersion,
          task_id: null,
          warnings: [],
        },
      });
      return;
    }
    await route.fulfill({
      json: { code: "TEST_OK", data: {}, message: "", ok: true, schema_version: schemaVersion, task_id: null, warnings: [] },
    });
  });

  const sample = (seq: number, ip: string, colo: string) =>
    JSON.stringify({
      event: "probe.speed",
      payload: {
        average_ready: true,
        average_speed_mb_s: 10,
        bytes_read: 1024,
        colo,
        current_ready: true,
        current_speed_mb_s: 12,
        elapsed_ms: 1000,
        ip,
        measured_bytes: 1024,
        measured_elapsed_ms: 1000,
        sample_bytes: 1024,
        sample_elapsed_ms: 1000,
        stage: "stage3_get",
        transfer_complete: false,
      },
      schema_version: schemaVersion,
      seq,
      task_id: "task-speed-1",
      ts: "2026-11-19T10:00:00.000Z",
    });

  const frames = [
    sample(1, "1.1.1.1", "LAX"),
    sample(2, "2.2.2.2", "SJC"),
    sample(3, "3.3.3.3", "HKG"),
    sample(4, "4.4.4.4", "NRT"),
  ];

  // 用假 EventSource 在同一 tick 内推四条采样，复现安卓 200ms 节流窗口；
  // 真 SSE 在这里只会让测试多等一次连接建立，和被测逻辑无关。
  await page.addInitScript((payloads: string[]) => {
    class FakeEventSource {
      onerror: (() => void) | null = null;
      onmessage: ((message: { data: string }) => void) | null = null;

      constructor(_url: string) {
        window.setTimeout(() => {
          for (const payload of payloads) {
            this.onmessage?.({ data: payload });
          }
        }, 2000);
      }

      close() {
        // no-op
      }
    }

    Object.defineProperty(window, "EventSource", {
      configurable: true,
      value: FakeEventSource,
    });
  }, frames);

  await page.goto("/");

  // 表格逐采样更新，不受 200ms 节流影响：同一 tick 的四颗 IP 必须四行都在。
  const table = page.getByRole("table", { name: "实时测速详情" });
  await expect(table).toBeVisible();
  await expect(page.getByText("并发 4", { exact: true })).toBeVisible();
  await expect(table.locator("tbody tr")).toHaveCount(4);
  for (const ip of ["1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4"]) {
    await expect(table).toContainText(ip);
  }
  for (const [ip, colo] of [
    ["1.1.1.1", "LAX"],
    ["2.2.2.2", "SJC"],
    ["3.3.3.3", "HKG"],
    ["4.4.4.4", "NRT"],
  ]) {
    await expect(table.getByText(ip, { exact: true })).toBeVisible();
    await expect(table.getByText(colo, { exact: true })).toBeVisible();
  }

  // 进程日志仍受节流约束：同一窗口内只留第一条，这是刻意保留的行为，不是回归。
  await expect(page.getByText("1.1.1.1(LAX) 正在测速中", { exact: true })).toHaveCount(1);
  await expect(page.getByText("2.2.2.2(SJC) 正在测速中", { exact: true })).toHaveCount(0);
});
