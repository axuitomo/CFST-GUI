// useSourceAutoSave 行为锁定（重构计划 §4.2）：水合守卫、合并保存、跳过日志去重、离开前保存。
import { describe, expect, it, vi } from "vitest";
import { useSourceAutoSave, type SourceAutoSaveDeps } from "./useSourceAutoSave";

function makeDeps(overrides: Partial<SourceAutoSaveDeps> = {}) {
  const logs: Array<{ event: string; payload?: unknown }> = [];
  const toasts: Array<{ message: string; tone?: string }> = [];
  const persistConfig = vi.fn(async () => true);
  const updateActiveSourceProfile = vi.fn(async () => true);
  const updateActiveSourceProfileQuietly = vi.fn(async () => true);
  const markSourceSaveBaselines = vi.fn();
  const deps: SourceAutoSaveDeps = {
    persistConfig,
    currentSourceSignature: () => "current",
    currentSourceProfileSignature: () => "profile",
    activeSourceProfileSourceSignature: () => "current",
    sourcePageHasUnsavedChanges: () => true,
    lastSavedSourceSignature: () => "saved",
    lastSavedSourceProfileSignature: () => "saved-profile",
    markSourceSaveBaselines,
    updateActiveSourceProfile,
    updateActiveSourceProfileQuietly,
    configHydrated: () => true,
    draftRestoring: () => false,
    appendLog: (event, payload) => logs.push({ event, payload }),
    showToast: (message, tone) => toasts.push({ message, tone }),
    ...overrides,
  };
  return { deps, logs, toasts, persistConfig, updateActiveSourceProfile, updateActiveSourceProfileQuietly, markSourceSaveBaselines };
}

describe("guards", () => {
  it("returns true without saving before hydration or during draft restore", async () => {
    const a = makeDeps({ configHydrated: () => false });
    expect(await useSourceAutoSave(a.deps).autoSaveSourcePage("interaction")).toBe(true);
    expect(a.persistConfig).not.toHaveBeenCalled();
    const b = makeDeps({ draftRestoring: () => true });
    expect(await useSourceAutoSave(b.deps).autoSaveSourcePage("interaction")).toBe(true);
    expect(b.persistConfig).not.toHaveBeenCalled();
  });

  it("skips when no unsaved changes", async () => {
    const { deps, persistConfig } = makeDeps({ sourcePageHasUnsavedChanges: () => false });
    expect(await useSourceAutoSave(deps).autoSaveSourcePage("interaction")).toBe(true);
    expect(persistConfig).not.toHaveBeenCalled();
  });
});

describe("autoSaveSourcePage", () => {
  it("updates profile quietly then persists silently and marks baselines", async () => {
    const { deps, persistConfig, updateActiveSourceProfileQuietly, markSourceSaveBaselines } = makeDeps({
      activeSourceProfileSourceSignature: () => "different",
      sourcePageHasUnsavedChanges: (() => {
        let calls = 0;
        return () => calls++ < 2;
      })(),
    });
    const ok = await useSourceAutoSave(deps).autoSaveSourcePage("interaction");
    expect(ok).toBe(true);
    expect(updateActiveSourceProfileQuietly).toHaveBeenCalledTimes(1);
    expect(persistConfig).toHaveBeenCalledWith({
      redirectOnMaskedToken: false,
      silentFailure: true,
      silentMaskedToken: true,
      silentSuccess: true,
      skipIfUnchanged: true,
    });
    expect(markSourceSaveBaselines).toHaveBeenCalled();
  });

  it("logs skipped once per signature when persist fails", async () => {
    const { deps, logs, persistConfig } = makeDeps({
      sourcePageHasUnsavedChanges: () => true,
    });
    persistConfig.mockResolvedValue(false);
    const api = useSourceAutoSave(deps);
    expect(await api.autoSaveSourcePage("interaction")).toBe(false);
    expect(await api.autoSaveSourcePage("interaction")).toBe(false);
    const skipped = logs.filter((l) => l.event === "source.auto_save.skipped");
    expect(skipped).toHaveLength(1);
    expect(skipped[0].payload).toEqual({ reason: "interaction" });
  });
});

describe("saveSourcePageBeforeLeave", () => {
  it("uses loud profile update and non-silent persist", async () => {
    const { deps, persistConfig, updateActiveSourceProfile, updateActiveSourceProfileQuietly } = makeDeps({
      activeSourceProfileSourceSignature: () => "different",
      sourcePageHasUnsavedChanges: () => true,
    });
    const ok = await useSourceAutoSave(deps).saveSourcePageBeforeLeave();
    expect(ok).toBe(true);
    expect(updateActiveSourceProfile).toHaveBeenCalledTimes(1);
    expect(updateActiveSourceProfileQuietly).not.toHaveBeenCalled();
    expect(persistConfig).toHaveBeenCalledWith({ redirectOnMaskedToken: false });
  });

  it("returns true when already consistent with baselines", async () => {
    const { deps, persistConfig } = makeDeps({
      lastSavedSourceSignature: () => "current",
      lastSavedSourceProfileSignature: () => "profile",
    });
    expect(await useSourceAutoSave(deps).saveSourcePageBeforeLeave()).toBe(true);
    expect(persistConfig).not.toHaveBeenCalled();
  });

  it("toasts error when profile update fails", async () => {
    const { deps, toasts, updateActiveSourceProfile } = makeDeps({
      activeSourceProfileSourceSignature: () => "different",
    });
    updateActiveSourceProfile.mockResolvedValue(false);
    expect(await useSourceAutoSave(deps).saveSourcePageBeforeLeave()).toBe(false);
    expect(toasts).toHaveLength(0);
  });
});