import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";
import type { ViewName } from "../lib/app/types";
import { useNavigation, type NavigationDeps } from "./useNavigation";

function makeDeps(overrides: Partial<NavigationDeps> = {}) {
  const selectedView = ref<ViewName>("dashboard");
  const deps: NavigationDeps = {
    autoSaveSettings: vi.fn(async () => true),
    configHydrated: () => true,
    draftRestoring: () => false,
    isAndroidApp: () => false,
    saveSourcePageBeforeLeave: vi.fn(async () => true),
    selectedView,
    sourcePageHasUnsavedChanges: () => false,
    ...overrides,
  };
  return { deps, selectedView };
}

function stubBrowser(options: { reducedMotion?: boolean; viewTransition?: boolean } = {}) {
  vi.stubGlobal("window", { matchMedia: () => ({ matches: options.reducedMotion ?? false }) });
  const startViewTransition = options.viewTransition
    ? vi.fn((callback: () => void) => {
        callback();
        return { finished: Promise.resolve() };
      })
    : undefined;
  vi.stubGlobal("document", { startViewTransition });
  return { startViewTransition };
}

describe("useNavigation", () => {
  beforeEach(() => {
    stubBrowser();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("普通导航直接切视图", async () => {
    const { deps, selectedView } = makeDeps();
    const { navigateTo } = useNavigation(deps);
    await expect(navigateTo("settings")).resolves.toBe(true);
    expect(selectedView.value).toBe("settings");
    expect(deps.autoSaveSettings).not.toHaveBeenCalled();
  });

  it("离开 settings 触发自动保存，进入不触发", async () => {
    const { deps, selectedView } = makeDeps();
    selectedView.value = "settings";
    const { navigateTo } = useNavigation(deps);
    await navigateTo("dashboard");
    expect(deps.autoSaveSettings).toHaveBeenCalledWith("navigation");
    vi.mocked(deps.autoSaveSettings).mockClear();
    await navigateTo("settings");
    expect(deps.autoSaveSettings).not.toHaveBeenCalled();
  });

  it("离开 sources 有未保存改动时先保存，保存失败则留在原页", async () => {
    const { deps, selectedView } = makeDeps({ sourcePageHasUnsavedChanges: () => true });
    selectedView.value = "sources";
    const { navigateTo } = useNavigation(deps);
    await expect(navigateTo("results")).resolves.toBe(true);
    expect(deps.saveSourcePageBeforeLeave).toHaveBeenCalledTimes(1);
    expect(selectedView.value).toBe("results");

    vi.mocked(deps.saveSourcePageBeforeLeave).mockResolvedValue(false);
    selectedView.value = "sources";
    await expect(navigateTo("results")).resolves.toBe(false);
    expect(selectedView.value).toBe("sources");
  });

  it("保存进行中再次导航被去重拒绝", async () => {
    let resolveSave: (value: boolean) => void = () => {};
    const savePromise = new Promise<boolean>((resolve) => {
      resolveSave = resolve;
    });
    const { deps, selectedView } = makeDeps({
      saveSourcePageBeforeLeave: vi.fn(() => savePromise),
      sourcePageHasUnsavedChanges: () => true,
    });
    selectedView.value = "sources";
    const { navigateTo } = useNavigation(deps);
    const first = navigateTo("results");
    await expect(navigateTo("dns")).resolves.toBe(false);
    resolveSave(true);
    await expect(first).resolves.toBe(true);
    // 原行为：target 在去重检查前被最后一次请求覆盖，保存完成后跳到最新目标。
    expect(selectedView.value).toBe("dns");
    expect(deps.saveSourcePageBeforeLeave).toHaveBeenCalledTimes(1);
  });

  it("未 hydrate / 草稿恢复中 / 目标仍是 sources 时跳过离页保存", async () => {
    for (const overrides of [{ configHydrated: () => false }, { draftRestoring: () => true }]) {
      const { deps, selectedView } = makeDeps({ sourcePageHasUnsavedChanges: () => true, ...overrides });
      selectedView.value = "sources";
      const { navigateTo } = useNavigation(deps);
      await navigateTo("results");
      expect(deps.saveSourcePageBeforeLeave).not.toHaveBeenCalled();
      expect(selectedView.value).toBe("results");
    }
    const { deps, selectedView } = makeDeps({ sourcePageHasUnsavedChanges: () => true });
    selectedView.value = "sources";
    const { navigateTo } = useNavigation(deps);
    await navigateTo("sources");
    expect(deps.saveSourcePageBeforeLeave).not.toHaveBeenCalled();
  });

  it("Android / 减弱动效 / 无 View Transition 均直切", async () => {
    const { startViewTransition } = stubBrowser({ viewTransition: true });
    const android = makeDeps({ isAndroidApp: () => true });
    await useNavigation(android.deps).navigateTo("dns");
    expect(android.selectedView.value).toBe("dns");
    expect(startViewTransition).not.toHaveBeenCalled();

    stubBrowser({ reducedMotion: true, viewTransition: true });
    const reduced = makeDeps();
    await useNavigation(reduced.deps).navigateTo("dns");
    expect(reduced.selectedView.value).toBe("dns");

    stubBrowser({ viewTransition: false });
    const plain = makeDeps();
    await useNavigation(plain.deps).navigateTo("dns");
    expect(plain.selectedView.value).toBe("dns");
  });

  it("桌面端走 View Transition 回调切视图", async () => {
    const { startViewTransition } = stubBrowser({ viewTransition: true });
    const { deps, selectedView } = makeDeps();
    const { navigateTo } = useNavigation(deps);
    await navigateTo("results");
    expect(startViewTransition).toHaveBeenCalledTimes(1);
    expect(selectedView.value).toBe("results");
  });

  it("changeSingleView 异步触发导航", async () => {
    const { deps, selectedView } = makeDeps();
    const { changeSingleView } = useNavigation(deps);
    changeSingleView("sources");
    await vi.waitFor(() => {
      expect(selectedView.value).toBe("sources");
    });
  });
});
