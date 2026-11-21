// 配置持久化编排（重构计划 §4.2）：保存/草稿/自动保存/签名基线。
// 从 App.vue 原样迁出；跨域回调与共享 ref 经 deps 注入。
import { computed, ref, type Ref } from "vue";
import { discardDraft, normalizeConfigSnapshot, saveConfig, saveDraft, type ConfigSnapshot, type TaskTone } from "../lib/bridge";
import { createSettingsAutoSaver } from "../lib/settingsAutoSave";
import { applyConfigSnapshotFrom } from "../lib/configSnapshotApply";
import { buildConfigSnapshot as buildConfigSnapshotFrom, snapshotSignature } from "../lib/configSnapshotBuild";
import type { SettingsForm } from "../lib/settingsForm";
import type { SourceDraft } from "./useSources";
import type { SourceGroupIPMode } from "../lib/sourceGroups";
import { asRecord, asString } from "../lib/valueCoerce";
import type { ToastTone, ViewName } from "../lib/appTypes";

export interface ConfigPersistenceDeps {
  settings: SettingsForm;
  loading: Ref<boolean>;
  probeWarnings: Ref<string[]>;
  configPath: Ref<string>;
  sources: Ref<SourceDraft[]>;
  sourceGroupIPMode: Ref<SourceGroupIPMode>;
  buildConfigSnapshot: () => ReturnType<typeof buildConfigSnapshotFrom>;
  currentSourceSignature: () => string;
  markSourceSaveBaselines: () => void;
  applySourceProfileStore: (value: unknown) => void;
  applyStorageStatus: (value: unknown) => void;
  refreshSchedulerStatus: () => Promise<void>;
  applyDNSProvidersFromSnapshot: (normalized: ConfigSnapshot) => void;
  isTaskActive: () => boolean;
  hasActiveTask: () => boolean;
  isDownloadSpeedTaskConfigCaptured: () => boolean;
  captureDownloadSpeedTaskConfig: (snapshot: ConfigSnapshot) => void;
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  pushActivity: (title: string, detail: string) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  applyNavigation: (view: ViewName) => void;
}

export interface PersistConfigOptions {
  redirectOnMaskedToken?: boolean;
  silentFailure?: boolean;
  silentMaskedToken?: boolean;
  silentSuccess?: boolean;
  skipIfUnchanged?: boolean;
}

export function useConfigPersistence(deps: ConfigPersistenceDeps) {
  const {
    settings,
    loading,
    probeWarnings,
    configPath,
    sources,
    sourceGroupIPMode,
    buildConfigSnapshot,
    currentSourceSignature,
    markSourceSaveBaselines,
    applySourceProfileStore,
    applyStorageStatus,
    refreshSchedulerStatus,
    appendLog,
    showToast,
    pushActivity,
    setStatus,
    applyNavigation,
  } = deps;
  let draftSaveTimer: number | undefined;
  let configHydrated = false;
  let draftRestoring = false;
  let configSaveInFlight: Promise<boolean> | null = null;
  let lastDraftSnapshotSignature = "";
  let lastSavedSnapshotSignature = "";
  let lastSettingsAutoSaveSkippedSignature = "";
  const saveState = ref<"error" | "idle" | "saved" | "saving">("idle");
  let saveStateTimer: number | undefined;

  function applyConfigSnapshot(snapshot: ConfigSnapshot) {
    applyConfigSnapshotFrom(snapshot, {
      settings,
      sourceGroupIPMode,
      sources,
      applyDNSProvidersFromSnapshot: deps.applyDNSProvidersFromSnapshot,
      isTaskActive: deps.isTaskActive,
      isDownloadSpeedTaskConfigCaptured: deps.isDownloadSpeedTaskConfigCaptured,
      captureDownloadSpeedTaskConfig: deps.captureDownloadSpeedTaskConfig,
    });
  }

function markSaveState(state: "error" | "saved" | "saving") {
  if (saveStateTimer !== undefined) {
    window.clearTimeout(saveStateTimer);
    saveStateTimer = undefined;
  }
  saveState.value = state;
  if (state === "saved") {
    saveStateTimer = window.setTimeout(() => {
      saveState.value = "idle";
      saveStateTimer = undefined;
    }, 4000);
  }
}

const saveBlockedByMaskedToken = computed(() =>
  settings.dnsProviders.some((entry) => {
    if (entry.provider === "dnshe") {
      return Boolean(entry.maskedKey || entry.maskedSecret) && (!entry.apiKey.trim() || !entry.apiSecret.trim());
    }
    return Boolean(entry.maskedToken) && !entry.apiToken.trim();
  }),
);

function currentSnapshotSignature() {
  return snapshotSignature(buildConfigSnapshot());
}

function scheduleDraftSave() {
  if (!configHydrated || draftRestoring || deps.hasActiveTask()) {
    return;
  }
  const signature = currentSnapshotSignature();
  if (signature === lastSavedSnapshotSignature || signature === lastDraftSnapshotSignature) {
    return;
  }
  if (draftSaveTimer !== undefined) {
    window.clearTimeout(draftSaveTimer);
  }
  draftSaveTimer = window.setTimeout(() => {
    draftSaveTimer = undefined;
    void saveDraftNow();
  }, 500);
}

async function saveDraftNow() {
  if (!configHydrated || draftRestoring) {
    return;
  }
  const snapshot = buildConfigSnapshot();
  const signature = snapshotSignature(snapshot);
  if (signature === lastSavedSnapshotSignature || signature === lastDraftSnapshotSignature) {
    return;
  }
  try {
    const result = await saveDraft({ config_snapshot: snapshot });
    appendLog("bridge.save_desktop_draft", result);
    if (result.ok) {
      lastDraftSnapshotSignature = signature;
    }
  } catch (error) {
    appendLog("bridge.save_desktop_draft.failed", error instanceof Error ? error.message : String(error));
  }
}

async function flushDraftSave() {
  if (draftSaveTimer !== undefined) {
    window.clearTimeout(draftSaveTimer);
    draftSaveTimer = undefined;
  }
  await saveDraftNow();
}

async function maybeRestoreDraft(statusValue: unknown) {
  const statusRecord = asRecord(statusValue);
  if (!statusRecord.exists || !statusRecord.is_newer_than_saved) {
    return;
  }
  const snapshot = asRecord(statusRecord.config_snapshot);
  if (Object.keys(snapshot).length === 0) {
    return;
  }
  const restore = window.confirm("检测到比正式配置更新的未保存草稿，是否恢复？取消将保留正式配置并丢弃草稿。");
  if (!restore) {
    const discarded = await discardDraft();
    appendLog("bridge.discard_desktop_draft", discarded);
    if (discarded.ok) {
      lastDraftSnapshotSignature = "";
    }
    return;
  }
  draftRestoring = true;
  applyConfigSnapshot(normalizeConfigSnapshot(snapshot));
  draftRestoring = false;
  lastDraftSnapshotSignature = currentSnapshotSignature();
  pushActivity("草稿已恢复", "已恢复上次未保存的桌面配置草稿。");
  showToast("已恢复未保存草稿", "success");
}

const settingsAutoSaver = createSettingsAutoSaver(() => {
  void runAutoSaveSettings("interaction");
});

async function autoSaveSettings(reason: string) {
  if (reason === "interaction") {
    settingsAutoSaver.schedule(reason);
    return true;
  }
  settingsAutoSaver.takePending();
  return runAutoSaveSettings(reason);
}

async function runAutoSaveSettings(reason: string) {
  if (!configHydrated || draftRestoring) {
    return true;
  }
  while (currentSnapshotSignature() !== lastSavedSnapshotSignature) {
    const saved = await persistConfig({
      redirectOnMaskedToken: false,
      silentFailure: true,
      silentMaskedToken: true,
      silentSuccess: true,
      skipIfUnchanged: true,
    });
    if (!saved) {
      const skippedSignature = currentSnapshotSignature();
      if (skippedSignature !== lastSettingsAutoSaveSkippedSignature) {
        appendLog("settings.auto_save.skipped", { reason });
        lastSettingsAutoSaveSkippedSignature = skippedSignature;
      }
      return false;
    }
  }
  lastSettingsAutoSaveSkippedSignature = "";
  return true;
}

async function persistConfig(options: PersistConfigOptions = {}) {
  if (configSaveInFlight) {
    return configSaveInFlight;
  }

  configSaveInFlight = persistConfigNow(options);
  try {
    return await configSaveInFlight;
  } finally {
    configSaveInFlight = null;
  }
}

async function persistCurrentConfig(options: PersistConfigOptions = {}) {
  while (currentSnapshotSignature() !== lastSavedSnapshotSignature) {
    const joinedExistingSave = configSaveInFlight !== null;
    const saved = await persistConfig(options);
    if (!saved) {
      if (joinedExistingSave && currentSnapshotSignature() !== lastSavedSnapshotSignature) {
        continue;
      }
      return false;
    }
  }
  return true;
}

async function persistConfigNow(options: PersistConfigOptions = {}) {
  const snapshot = buildConfigSnapshot();
  const requestedSignature = snapshotSignature(snapshot);
  const requestedSourceSignature = currentSourceSignature();
  if (options.skipIfUnchanged && requestedSignature === lastSavedSnapshotSignature) {
    return true;
  }

  if (saveBlockedByMaskedToken.value) {
    markSaveState("error");
    if (options.silentMaskedToken) {
      return false;
    }
    setStatus({
      detail: "当前只拿到了脱敏 Token。请重新输入完整 API Token 后再保存。",
      title: "需要完整 Token",
      tone: "failed",
    });
    pushActivity("保存被阻止", "检测到脱敏 Token，占位值不能直接回写。");
    if (options.redirectOnMaskedToken !== false) {
      applyNavigation("settings");
    }
    showToast("需要重新输入完整 Token", "error");
    return false;
  }

  markSaveState("saving");
  if (!options.silentSuccess) {
    loading.value = true;
  }

  try {
    const result = await saveConfig({
      config_snapshot: snapshot,
    });
    const data = asRecord(result.data);
    appendLog("bridge.save_config", result);
    probeWarnings.value = result.warnings || [];

    if (!result.ok) {
      markSaveState("error");
      if (!options.silentFailure) {
        setStatus({
          detail: result.message || "保存配置失败。",
          title: "保存失败",
          tone: "failed",
        });
        pushActivity("保存失败", result.message || "保存配置失败。");
        showToast("保存配置失败", "error");
      }
      return false;
    }

    const normalizedSnapshot = normalizeConfigSnapshot(data.config_snapshot || data.configSnapshot || snapshot);
    const currentStillMatchesRequest = currentSnapshotSignature() === requestedSignature && currentSourceSignature() === requestedSourceSignature;
    if (currentStillMatchesRequest) {
      applyConfigSnapshot(normalizedSnapshot);
    }
    applyStorageStatus(data.storage);
    if (currentStillMatchesRequest) {
      if (data.source_profiles || data.sourceProfiles) {
        applySourceProfileStore(data.source_profiles || data.sourceProfiles);
      }
    }
    if (draftSaveTimer !== undefined) {
      window.clearTimeout(draftSaveTimer);
      draftSaveTimer = undefined;
    }
    lastSavedSnapshotSignature = currentStillMatchesRequest ? currentSnapshotSignature() : snapshotSignature(normalizedSnapshot);
    lastDraftSnapshotSignature = "";
    if (currentStillMatchesRequest) {
      markSourceSaveBaselines();
    }
    configPath.value = asString(data.configPath || data.config_path || configPath.value);
    if (!options.silentSuccess) {
      setStatus({
        detail: result.message || "配置已保存。",
        title: "配置已保存",
        tone: "idle",
      });
      pushActivity("配置已保存", result.message || "设置已保存并可用于后续任务。");
      showToast("配置已保存");
    }
    markSaveState("saved");
    await refreshSchedulerStatus();
    return true;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    appendLog("bridge.save_config.failed", message);
    markSaveState("error");
    if (!options.silentFailure) {
      setStatus({
        detail: message || "保存配置失败。",
        title: "保存失败",
        tone: "failed",
      });
      pushActivity("保存失败", message || "保存配置失败。");
      showToast("保存配置失败", "error");
    }
    return false;
  } finally {
    if (!options.silentSuccess) {
      loading.value = false;
    }
  }
}
  // Android WebView 未必触发 beforeunload，挂起的合并保存要在 pagehide/隐藏时补发。
  function flushPendingSettingsSave() {
    if (settingsAutoSaver.takePending()) void runAutoSaveSettings("pagehide");
  }

  function cancelPersistenceTimers() {
    if (saveStateTimer !== undefined) {
      window.clearTimeout(saveStateTimer);
    }
    if (draftSaveTimer !== undefined) {
      window.clearTimeout(draftSaveTimer);
    }
  }

  return {
    applyConfigSnapshot,
    autoSaveSettings,
    cancelPersistenceTimers,
    currentSnapshotSignature,
    flushDraftSave,
    flushPendingSettingsSave,
    maybeRestoreDraft,
    persistConfig,
    persistCurrentConfig,
    saveState,
    scheduleDraftSave,
    configHydrated: () => configHydrated,
    draftRestoring: () => draftRestoring,
    markConfigHydrated: () => {
      configHydrated = true;
    },
    lastSavedSnapshotSignature: () => lastSavedSnapshotSignature,
    lastDraftSnapshotSignature: () => lastDraftSnapshotSignature,
    setLastSavedSnapshotSignature: (signature: string) => {
      lastSavedSnapshotSignature = signature;
    },
  };
}
