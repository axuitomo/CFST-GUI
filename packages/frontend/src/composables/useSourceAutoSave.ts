// 输入源页自动保存编排（重构计划 §4.2）：合并保存、离开前保存、跳过日志去重。
// 从 App.vue 原样迁出；persistConfig 与签名/基线经 deps 注入。
import type { ToastTone } from "../lib/app/types";
import { snapshotSignature } from "../lib/config/snapshotBuild";

export interface SourceAutoSaveDeps {
  persistConfig: (options?: { redirectOnMaskedToken?: boolean; silentFailure?: boolean; silentMaskedToken?: boolean; silentSuccess?: boolean; skipIfUnchanged?: boolean }) => Promise<boolean>;
  currentSourceSignature: () => string;
  currentSourceProfileSignature: () => string;
  activeSourceProfileSourceSignature: () => string;
  sourcePageHasUnsavedChanges: () => boolean;
  lastSavedSourceSignature: () => string;
  lastSavedSourceProfileSignature: () => string;
  markSourceSaveBaselines: () => void;
  updateActiveSourceProfile: () => Promise<boolean>;
  updateActiveSourceProfileQuietly: () => Promise<boolean>;
  configHydrated: () => boolean;
  draftRestoring: () => boolean;
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
}

export function useSourceAutoSave(deps: SourceAutoSaveDeps) {
  const { persistConfig, currentSourceSignature, currentSourceProfileSignature, activeSourceProfileSourceSignature, sourcePageHasUnsavedChanges, markSourceSaveBaselines, updateActiveSourceProfile, updateActiveSourceProfileQuietly, appendLog, showToast } = deps;
  let lastSourceAutoSaveSkippedSignature = "";
  let sourceAutoSaveInFlight: Promise<boolean> | null = null;

  function currentSourceAutoSaveSignature() {
    return snapshotSignature({
      active_sources: currentSourceSignature(),
      source_profiles: currentSourceProfileSignature(),
      saved_sources: deps.lastSavedSourceSignature(),
      saved_source_profiles: deps.lastSavedSourceProfileSignature(),
    });
  }

  async function persistSourceConfigQuietly() {
    const saved = await persistConfig({
      redirectOnMaskedToken: false,
      silentFailure: true,
      silentMaskedToken: true,
      silentSuccess: true,
      skipIfUnchanged: true,
    });
    if (saved) {
      markSourceSaveBaselines();
    }
    return saved;
  }

  async function saveSourcePageQuietly() {
    try {
      if (currentSourceSignature() !== activeSourceProfileSourceSignature()) {
        const profileSaved = await updateActiveSourceProfileQuietly();
        if (!profileSaved) {
          return false;
        }
      }

      if (currentSourceSignature() !== deps.lastSavedSourceSignature() || currentSourceProfileSignature() !== deps.lastSavedSourceProfileSignature()) {
        return persistSourceConfigQuietly();
      }

      return true;
    } catch (error) {
      appendLog("source.auto_save.failed", error instanceof Error ? error.message : String(error));
      return false;
    }
  }

  async function autoSaveSourcePage(reason: string) {
    if (!deps.configHydrated() || deps.draftRestoring()) {
      return true;
    }
    if (sourceAutoSaveInFlight) {
      return sourceAutoSaveInFlight;
    }
    if (!sourcePageHasUnsavedChanges()) {
      lastSourceAutoSaveSkippedSignature = "";
      return true;
    }

    sourceAutoSaveInFlight = (async () => {
      while (sourcePageHasUnsavedChanges()) {
        const saved = await saveSourcePageQuietly();
        if (!saved) {
          const skippedSignature = currentSourceAutoSaveSignature();
          if (skippedSignature !== lastSourceAutoSaveSkippedSignature) {
            appendLog("source.auto_save.skipped", { reason });
            lastSourceAutoSaveSkippedSignature = skippedSignature;
          }
          return false;
        }
      }
      lastSourceAutoSaveSkippedSignature = "";
      return true;
    })();

    try {
      return await sourceAutoSaveInFlight;
    } finally {
      sourceAutoSaveInFlight = null;
    }
  }

  async function saveSourcePageBeforeLeave() {
    try {
      if (sourceAutoSaveInFlight) {
        await sourceAutoSaveInFlight;
        if (!sourcePageHasUnsavedChanges()) {
          return true;
        }
      }

      if (currentSourceSignature() !== activeSourceProfileSourceSignature()) {
        const profileSaved = await updateActiveSourceProfile();
        if (!profileSaved) {
          return false;
        }
      }

      if (currentSourceSignature() !== deps.lastSavedSourceSignature() || currentSourceProfileSignature() !== deps.lastSavedSourceProfileSignature()) {
        return persistConfig({ redirectOnMaskedToken: false });
      }

      return true;
    } catch (error) {
      showToast(error instanceof Error ? error.message : "输入源保存失败", "error");
      return false;
    }
  }

  return {
    autoSaveSourcePage,
    saveSourcePageBeforeLeave,
  };
}
