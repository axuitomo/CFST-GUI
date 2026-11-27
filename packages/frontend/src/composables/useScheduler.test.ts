import { beforeEach, describe, expect, it, vi } from "vitest";

const { loadSchedulerStatus } = vi.hoisted(() => ({ loadSchedulerStatus: vi.fn() }));

vi.mock("../lib/bridge", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/bridge")>();
  return { ...actual, loadSchedulerStatus };
});

import { createDefaultSettingsForm } from "../lib/config/settingsForm";
import { useScheduler, type UseSchedulerDeps } from "./useScheduler";

function makeDeps() {
  const settings = createDefaultSettingsForm();
  const deps: UseSchedulerDeps = {
    settings,
    appendLog: vi.fn(),
    showToast: vi.fn(),
    pushActivity: vi.fn(),
    flushDraftSave: vi.fn(async () => {}),
    persistCurrentConfig: vi.fn(async () => true),
  };
  return { deps, settings };
}

describe("useScheduler", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    loadSchedulerStatus.mockResolvedValue({ ok: true, message: "", warnings: [], data: { enabled: true } });
  });

  it("读取定时状态：成功写入", async () => {
    const { deps } = makeDeps();
    const api = useScheduler(deps);
    await api.refreshSchedulerStatus();
    expect(api.schedulerStatus.value).toEqual({ enabled: true });
  });

  it("SCHEDULER_UNSUPPORTED：静默置空，不推送动态", async () => {
    loadSchedulerStatus.mockResolvedValue({ ok: false, code: "SCHEDULER_UNSUPPORTED", message: "", warnings: [], data: null });
    const { deps } = makeDeps();
    const api = useScheduler(deps);
    await api.refreshSchedulerStatus();
    expect(api.schedulerStatus.value).toBeNull();
    expect(deps.pushActivity).not.toHaveBeenCalled();
  });

  it("每日时间失焦：未启用只 flush 草稿", async () => {
    const { deps, settings } = makeDeps();
    settings.schedulerEnabled = false;
    const api = useScheduler(deps);
    await api.handleSchedulerDailyTimesBlur();
    expect(deps.flushDraftSave).toHaveBeenCalled();
    expect(deps.persistCurrentConfig).not.toHaveBeenCalled();
  });

  it("每日时间失焦：非法时间拦截，合法时间归一后保存并刷新", async () => {
    const { deps, settings } = makeDeps();
    settings.schedulerEnabled = true;
    settings.schedulerTriggerMode = "daily";
    settings.schedulerDailyTimes = "不是时间";
    const api = useScheduler(deps);
    await api.handleSchedulerDailyTimesBlur();
    expect(deps.showToast).toHaveBeenCalledWith("请填写有效的每日定时时间", "error");
    expect(deps.persistCurrentConfig).not.toHaveBeenCalled();
    settings.schedulerDailyTimes = "8:00\n20:00";
    await api.handleSchedulerDailyTimesBlur();
    expect(settings.schedulerDailyTimes).toBe("08:00\n20:00");
    expect(deps.persistCurrentConfig).toHaveBeenCalledWith(expect.objectContaining({ skipIfUnchanged: true, silentSuccess: true }));
    expect(loadSchedulerStatus).toHaveBeenCalled();
  });
});
