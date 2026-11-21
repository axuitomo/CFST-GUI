import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";

const mocks = vi.hoisted(() => ({
  selectPath: vi.fn(),
  importConfigArchive: vi.fn(),
  exportConfigArchive: vi.fn(),
  exportDebugLog: vi.fn(),
  exportDiagnosticPackage: vi.fn(),
  openLogDirectory: vi.fn(),
  openPath: vi.fn(),
}));

vi.mock("../lib/bridge", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/bridge")>();
  return { ...actual, ...mocks };
});

import { createDefaultSettingsForm } from "../lib/settingsForm";
import type { SourceDraft } from "./useSources";
import { useFileExport, type FileExportDeps } from "./useFileExport";

function makeDeps() {
  const settings = createDefaultSettingsForm();
  const sources = ref<SourceDraft[]>([{ id: "s1", kind: "file", path: "C:/a.txt", content: "", status_text: "" } as SourceDraft]);
  const deps: FileExportDeps = {
    settings,
    sources,
    configPath: ref("C:/cfg.json"),
    storageStatus: ref(null),
    isAndroidApp: () => false,
    buildConfigSnapshot: () => ({}) as never,
    applyConfigSnapshot: vi.fn(),
    applySourceProfileStore: vi.fn(),
    markSourceSaveBaselines: vi.fn(),
    applyStorageStatus: vi.fn(),
    applyNavigation: vi.fn(),
    appendLog: vi.fn(),
    showToast: vi.fn(),
    setStatus: vi.fn(),
  };
  return { deps, settings, sources };
}

describe("useFileExport", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("window", { confirm: () => true });
    mocks.selectPath.mockResolvedValue({ ok: true, message: "", warnings: [], data: { canceled: false, path: "C:/sel" } });
    mocks.importConfigArchive.mockResolvedValue({ ok: true, message: "导入成功", warnings: [], data: { config_snapshot: {}, source_profiles: [], storage: { current_dir: "C:/d" }, configPath: "C:/new.json" } });
    mocks.exportConfigArchive.mockResolvedValue({ ok: true, message: "", warnings: [], data: null });
    mocks.exportDebugLog.mockResolvedValue({ ok: true, message: "", warnings: [], data: { path: "C:/log.txt" } });
    mocks.exportDiagnosticPackage.mockResolvedValue({ ok: true, message: "", warnings: [], data: { path: "C:/diag.zip" } });
    mocks.openLogDirectory.mockResolvedValue({ ok: true, message: "", warnings: [], data: { path: "C:/logs" } });
  });

  it("pathLeafName：去查询串/尾分隔符，识别 browser-download 前缀", () => {
    const { deps } = makeDeps();
    const api = useFileExport(deps);
    expect(api.pathLeafName("C:\\dir\\file.csv?x=1")).toBe("file.csv");
    expect(api.pathLeafName("browser-download:sub/result.csv")).toBe("result.csv");
    expect(api.pathLeafName("C:/dir/")).toBe("dir");
    expect(api.pathLeafName("  ")).toBe("");
  });

  it("selectSourceFile：浏览器内联内容优先于路径", async () => {
    mocks.selectPath.mockResolvedValue({ ok: true, message: "", warnings: [], data: { canceled: false, content: "1.1.1.1\n2.2.2.2", display_name: "ips.txt" } });
    const { deps, sources } = makeDeps();
    const api = useFileExport(deps);
    await api.selectSourceFile("s1");
    expect(sources.value[0]).toMatchObject({ kind: "inline", content: "1.1.1.1\n2.2.2.2", path: "" });
    expect(deps.showToast).toHaveBeenCalledWith("已读取输入源文件", "success");
  });

  it("selectExportTarget：SAF uri 与桌面目录分别落位", async () => {
    mocks.selectPath.mockResolvedValue({ ok: true, message: "", warnings: [], data: { canceled: false, target_uri: "content://saf/dir" } });
    const { deps, settings } = makeDeps();
    const api = useFileExport(deps);
    await api.selectExportTarget();
    expect(settings.exportTargetUri).toBe("content://saf/dir");
    expect(settings.exportTargetDir).toBe("");
    mocks.selectPath.mockResolvedValue({ ok: true, message: "", warnings: [], data: { canceled: false, directory: "C:/out", file_name: "r.csv" } });
    await api.selectExportTarget();
    expect(settings.exportTargetDir).toBe("C:/out");
    expect(settings.exportFileName).toBe("r.csv");
    expect(settings.exportTargetUri).toBe("");
  });

  it("importConfigFromFile：应用导入数据并跳转设置页", async () => {
    const { deps } = makeDeps();
    const api = useFileExport(deps);
    await api.importConfigFromFile();
    expect(deps.applyConfigSnapshot).toHaveBeenCalled();
    expect(deps.applySourceProfileStore).toHaveBeenCalled();
    expect(deps.markSourceSaveBaselines).toHaveBeenCalled();
    expect(deps.applyStorageStatus).toHaveBeenCalledWith({ current_dir: "C:/d" });
    expect(deps.configPath.value).toBe("C:/new.json");
    expect(deps.applyNavigation).toHaveBeenCalledWith("settings");
    expect(deps.showToast).toHaveBeenCalledWith("导入成功", "success");
  });

  it("exportCurrentDebugLog：Android 未选目录时不发导出请求", async () => {
    mocks.selectPath.mockResolvedValue({ ok: true, message: "", warnings: [], data: { canceled: true } });
    const { deps } = makeDeps();
    deps.isAndroidApp = () => true;
    const api = useFileExport(deps);
    await api.exportCurrentDebugLog();
    expect(mocks.exportDebugLog).not.toHaveBeenCalled();
  });

  it("exportConfigToFile：拒绝确认则不选择路径", async () => {
    vi.stubGlobal("window", { confirm: () => false });
    const { deps } = makeDeps();
    const api = useFileExport(deps);
    await api.exportConfigToFile();
    expect(mocks.selectPath).not.toHaveBeenCalled();
  });
});