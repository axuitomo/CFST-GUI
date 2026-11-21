import { beforeEach, describe, expect, it, vi } from "vitest";

const { checkStorageHealth, openPath } = vi.hoisted(() => ({ checkStorageHealth: vi.fn(), openPath: vi.fn() }));

vi.mock("../lib/bridge", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/bridge")>();
  return { ...actual, checkStorageHealth, openPath };
});

import { useStorageHealth, type UseStorageHealthDeps } from "./useStorageHealth";

function makeDeps() {
  const deps: UseStorageHealthDeps = {
    appendLog: vi.fn(),
    showToast: vi.fn(),
    setStatus: vi.fn(),
    commandDiagnosticPayload: (result) => result,
  };
  return deps;
}

describe("useStorageHealth", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    checkStorageHealth.mockResolvedValue({ ok: true, message: "", warnings: [], data: { storage: { current_dir: "C:/data", writable: true, permission_ok: true }, health: { message: "目录健康" } } });
    openPath.mockResolvedValue(undefined);
  });

  it("applyStorageStatus：兼容 snake/camel 字段并归一布尔", () => {
    const api = useStorageHealth(makeDeps());
    api.applyStorageStatus({ currentDir: "C:/data", permissionOk: false, portable_mode: true, writable: false });
    expect(api.storageStatus.value).toMatchObject({ current_dir: "C:/data", permission_ok: false, portable_mode: true, writable: false });
    api.applyStorageStatus({}); // 空对象不覆盖
    expect(api.storageStatus.value?.current_dir).toBe("C:/data");
  });

  it("storageStatusTone：权限/可写失败优先，其次同步错误", () => {
    const api = useStorageHealth(makeDeps());
    expect(api.storageStatusTone(null, "completed")).toBe("completed");
    expect(api.storageStatusTone({ writable: false } as never)).toBe("failed");
    expect(api.storageStatusTone({ last_sync_error: "超时" } as never)).toBe("warning");
  });

  it("statusDetailWithStorage：拼接数据目录与运行时目录", () => {
    const api = useStorageHealth(makeDeps());
    const detail = api.statusDetailWithStorage("检查完成", { current_dir: "C:/data", runtime_dir: "C:/run" } as never);
    expect(detail).toBe("检查完成 应用数据目录：C:/data；运行时目录：C:/run");
  });

  it("健康检查成功：刷新存储状态并按 tone 提示", async () => {
    const deps = makeDeps();
    const api = useStorageHealth(deps);
    await api.checkCurrentStorageHealth();
    expect(api.storageStatus.value?.current_dir).toBe("C:/data");
    expect(deps.setStatus).toHaveBeenCalledWith(expect.objectContaining({ title: "应用数据目录已检查", tone: "idle" }));
    expect(deps.showToast).toHaveBeenCalledWith("目录健康", "success");
  });

  it("打开存储目录：未读取时提示，已读取时调用 openPath", async () => {
    const deps = makeDeps();
    const api = useStorageHealth(deps);
    await api.openStorageDirectory();
    expect(deps.showToast).toHaveBeenCalledWith("尚未读取应用数据目录", "info");
    api.applyStorageStatus({ current_dir: "C:/data" });
    await api.openStorageDirectory();
    expect(openPath).toHaveBeenCalledWith("C:/data");
  });
});