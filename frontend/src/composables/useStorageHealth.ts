// 存储健康域（重构计划 §4.2）：从 App.vue 原样迁出。
import { ref } from "vue";
import { checkStorageHealth, openPath, type StorageStatus, type TaskTone } from "../lib/bridge";
import { asRecord, asString } from "../lib/valueCoerce";
import type { ToastTone } from "../lib/appTypes";
export interface UseStorageHealthDeps {
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
  commandDiagnosticPayload: (result: { code?: string; data?: unknown; message?: string; warnings?: string[] }) => unknown;
}
export function useStorageHealth(deps: UseStorageHealthDeps) {
  const { appendLog, showToast, setStatus, commandDiagnosticPayload } = deps;
  const storageStatus = ref<StorageStatus | null>(null);

  function applyStorageStatus(value: unknown) {
    const source = asRecord(value);
    if (Object.keys(source).length === 0) {
      return;
    }
    storageStatus.value = {
      backend: (asString(source.backend) || undefined) as StorageStatus["backend"],
      bootstrap_path: asString(source.bootstrap_path || source.bootstrapPath),
      current_dir: asString(source.current_dir || source.currentDir),
      default_dir: asString(source.default_dir || source.defaultDir),
      display_name: asString(source.display_name || source.displayName),
      health: asRecord(source.health) as unknown as StorageStatus["health"],
      last_sync_at: asString(source.last_sync_at || source.lastSyncAt),
      last_sync_error: asString(source.last_sync_error || source.lastSyncError),
      log_uri: asString(source.log_uri || source.logUri),
      permission_ok: source.permission_ok !== false && source.permissionOk !== false,
      portable_mode: Boolean(source.portable_mode || source.portableMode),
      runtime_dir: asString(source.runtime_dir || source.runtimeDir),
      setup_completed: Boolean(source.setup_completed || source.setupCompleted),
      setup_required: Boolean(source.setup_required || source.setupRequired),
      storage_uri: asString(source.storage_uri || source.storageUri),
      writable: source.writable !== false,
    };
  }

  function storageContextDetail(storage: StorageStatus | null | undefined) {
    if (!storage) {
      return "";
    }
    const details: string[] = [];
    const current = storage.current_dir?.trim() || "";
    const runtime = storage.runtime_dir?.trim() || "";

    if (current) {
      details.push(`应用数据目录：${current}`);
    }

    if (runtime && runtime !== current) {
      details.push(`运行时目录：${runtime}`);
    }

    return details.join("；");
  }

  function statusDetailWithStorage(message: string, storage: StorageStatus | null | undefined) {
    return [message.trim(), storageContextDetail(storage)].filter(Boolean).join(" ");
  }

  function storageStatusTone(storage: StorageStatus | null | undefined, fallback: TaskTone = "idle"): TaskTone {
    if (!storage) {
      return fallback;
    }
    if (storage.permission_ok === false || storage.writable === false) {
      return "failed";
    }
    if ((storage.last_sync_error || "").trim()) {
      return "warning";
    }
    return fallback;
  }

  async function checkCurrentStorageHealth() {
    try {
      const result = await checkStorageHealth({ path: storageStatus.value?.current_dir || "" });
      appendLog("bridge.storage_health", commandDiagnosticPayload(result));
      if (!result.ok) {
        const message = result.message || "应用数据目录健康检查失败";
        setStatus({
          detail: statusDetailWithStorage(message, storageStatus.value),
          title: "应用数据目录健康检查失败",
          tone: "failed",
        });
        showToast(message, "error");
        return;
      }
      applyStorageStatus(asRecord(asRecord(result.data).storage));
      const health = asRecord(asRecord(result.data).health);
      const message = asString(health.message) || result.message || "应用数据目录健康检查完成";
      setStatus({
        detail: statusDetailWithStorage(message, storageStatus.value),
        title: storageStatusTone(storageStatus.value) === "failed" ? "应用数据目录异常" : storageStatusTone(storageStatus.value) === "warning" ? "存储状态异常" : "应用数据目录已检查",
        tone: storageStatusTone(storageStatus.value),
      });
      showToast(message, storageStatusTone(storageStatus.value) === "failed" ? "error" : storageStatusTone(storageStatus.value) === "warning" ? "info" : "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "应用数据目录健康检查失败", "error");
    }
  }

  async function openStorageDirectory() {
    const target = storageStatus.value?.current_dir || "";
    if (!target) {
      showToast("尚未读取应用数据目录", "info");
      return;
    }
    try {
      await openPath(target);
    } catch (error) {
      showToast(error instanceof Error ? error.message : "打开应用数据目录失败", "error");
    }
  }

  return { storageStatus, applyStorageStatus, storageContextDetail, statusDetailWithStorage, storageStatusTone, checkCurrentStorageHealth, openStorageDirectory };
}
