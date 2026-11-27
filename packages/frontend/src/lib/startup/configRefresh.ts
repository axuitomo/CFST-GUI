// 启动配置读取与状态落地，从 useStartupSync 拆出。
import type { Ref } from "vue";
import { loadConfig, normalizeConfigSnapshot, type StorageStatus, type TaskTone } from "../bridge";
import { asRecord, asString } from "../app/valueCoerce";

export interface ConfigRefreshDeps {
  loading: Ref<boolean>;
  probeWarnings: Ref<string[]>;
  configPath: Ref<string>;
  appendLog: (event: string, payload: unknown) => void;
  commandDiagnosticPayload: (result: { code?: string; data?: unknown; message?: string; warnings?: string[] }) => unknown;
  applyStorageStatus: (value: unknown) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  statusDetailWithStorage: (message: string, storage: StorageStatus | null) => string;
  storageStatus: Ref<StorageStatus | null>;
  applyConfigSnapshot: (snapshot: ReturnType<typeof normalizeConfigSnapshot>) => void;
  applySourceProfileStore: (value: unknown) => void;
  markSourceSaveBaselines: () => void;
  maybeRestoreDraft: (draftStatus: unknown) => Promise<void> | void;
  setLastSavedSnapshotSignature: (signature: string) => void;
  currentSnapshotSignature: () => string;
  markConfigHydrated: () => void;
  refreshDroppedSources: () => Promise<void>;
  startupTimedOut: Ref<boolean>;
  pushActivity: (title: string, detail: string) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
}

export async function refreshConfig(deps: ConfigRefreshDeps) {
  const {
    loading,
    probeWarnings,
    configPath,
    appendLog,
    commandDiagnosticPayload,
    applyStorageStatus,
    setStatus,
    statusDetailWithStorage,
    storageStatus,
    applyConfigSnapshot,
    applySourceProfileStore,
    markSourceSaveBaselines,
    maybeRestoreDraft,
    setLastSavedSnapshotSignature,
    currentSnapshotSignature,
    markConfigHydrated,
    refreshDroppedSources,
    startupTimedOut,
    pushActivity,
    showToast,
  } = deps;
  loading.value = true;

  try {
    const result = await loadConfig();
    const data = asRecord(result.data);
    appendLog("bridge.load_config", commandDiagnosticPayload(result));
    probeWarnings.value = result.warnings || [];
    applyStorageStatus(data.storage);
    configPath.value = asString(data.configPath || data.config_path || configPath.value);

    if (!result.ok) {
      const failureMessage = result.message || "读取配置失败。";
      setStatus({
        detail: statusDetailWithStorage(failureMessage, storageStatus.value),
        title: storageStatus.value?.permission_ok === false ? "权限失效" : "读取失败",
        tone: "failed",
      });
      pushActivity("读取配置失败", failureMessage);
      showToast(failureMessage, "error");
      return;
    }

    applyConfigSnapshot(normalizeConfigSnapshot(data.config_snapshot || {}));
    if (data.source_profiles || data.sourceProfiles) {
      applySourceProfileStore(data.source_profiles || data.sourceProfiles);
    }
    markSourceSaveBaselines();
    await maybeRestoreDraft(data.draft_status || data.draftStatus);
    setLastSavedSnapshotSignature(currentSnapshotSignature());
    markConfigHydrated();
    await refreshDroppedSources();
    startupTimedOut.value = false;
    configPath.value = asString(data.configPath || data.config_path || configPath.value);
    const successMessage = result.message || "配置已加载。";
    const syncWarning = storageStatus.value?.last_sync_error?.trim() || "";
    const successDetail = syncWarning ? statusDetailWithStorage(`${successMessage} 但存储状态异常：${syncWarning}`, storageStatus.value) : statusDetailWithStorage(successMessage, storageStatus.value);
    setStatus({
      detail: successDetail,
      title: syncWarning ? "存储状态异常" : "配置已加载",
      tone: syncWarning ? "warning" : "idle",
    });
    pushActivity(syncWarning ? "配置已加载（存储状态异常）" : "配置已加载", syncWarning || successMessage || "已读取当前配置快照。");
    showToast(syncWarning ? `配置已加载，但存储状态异常：${syncWarning}` : successMessage || "配置已加载", syncWarning ? "info" : "success");
  } finally {
    loading.value = false;
  }
}
