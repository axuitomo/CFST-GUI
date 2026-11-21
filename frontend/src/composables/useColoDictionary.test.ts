import { beforeEach, describe, expect, it, vi } from "vitest";

const { loadColoDictionaryStatus, processColoDictionary, updateColoDictionary } = vi.hoisted(() => ({ loadColoDictionaryStatus: vi.fn(), processColoDictionary: vi.fn(), updateColoDictionary: vi.fn() }));

vi.mock("../lib/bridge", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/bridge")>();
  return { ...actual, loadColoDictionaryStatus, processColoDictionary, updateColoDictionary };
});

import { useColoDictionary, type UseColoDictionaryDeps } from "./useColoDictionary";

function makeDeps() {
  const deps: UseColoDictionaryDeps = { appendLog: vi.fn(), showToast: vi.fn(), setStatus: vi.fn() };
  return deps;
}

describe("useColoDictionary", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    loadColoDictionaryStatus.mockResolvedValue({ ok: true, message: "", warnings: [], data: { total: 100 } });
    updateColoDictionary.mockResolvedValue({ ok: true, message: "拉取完成", warnings: [], data: { total: 100 } });
    processColoDictionary.mockResolvedValue({ ok: true, message: "处理完成", warnings: [], data: { total: 100 } });
  });

  it("读取状态：成功写入，失败提示", async () => {
    const deps = makeDeps();
    const api = useColoDictionary(deps);
    await api.refreshColoDictionaryStatus();
    expect(api.coloDictionaryStatus.value).toEqual({ total: 100 });
    loadColoDictionaryStatus.mockResolvedValue({ ok: false, message: "离线", warnings: [], data: null });
    await api.refreshColoDictionaryStatus();
    expect(deps.showToast).toHaveBeenCalledWith("离线", "error");
  });

  it("拉取词典：updating 标志复位并提示成功", async () => {
    const deps = makeDeps();
    const api = useColoDictionary(deps);
    await api.refreshColoDictionary();
    expect(api.coloDictionaryUpdating.value).toBe(false);
    expect(deps.showToast).toHaveBeenCalledWith("COLO 原始词典已拉取", "success");
  });

  it("本地处理失败：failed 状态且 processing 复位", async () => {
    processColoDictionary.mockResolvedValue({ ok: false, message: "磁盘满", warnings: [], data: null });
    const deps = makeDeps();
    const api = useColoDictionary(deps);
    await api.processLocalColoDictionary();
    expect(api.coloDictionaryProcessing.value).toBe(false);
    expect(deps.setStatus).toHaveBeenCalledWith(expect.objectContaining({ title: "词典处理失败", tone: "failed" }));
    expect(deps.showToast).toHaveBeenCalledWith("处理 COLO 词典失败", "error");
  });
});
