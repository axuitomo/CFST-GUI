// 桌面定时任务域（重构计划 §4.2）：从 App.vue 原样迁出。
import { ref } from "vue";
import { loadSchedulerStatus, type SchedulerStatus } from "../lib/bridge";
import { schedulerDailyTimesFromText } from "../lib/valueCoerce";
import type { SettingsForm } from "../lib/settingsForm";
import type { ToastTone } from "../lib/appTypes";
import type { PersistConfigOptions } from "./useConfigPersistence";
export interface UseSchedulerDeps {
  settings: SettingsForm;
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  pushActivity: (title: string, detail: string) => void;
  flushDraftSave: () => Promise<void>;
  persistCurrentConfig: (options?: PersistConfigOptions) => Promise<boolean>;
}
export function useScheduler(deps: UseSchedulerDeps) {
  const { settings, appendLog, showToast, pushActivity, flushDraftSave, persistCurrentConfig } = deps;
  const schedulerStatus = ref<SchedulerStatus | null>(null);

  async function refreshSchedulerStatus() {
    try {
      const result = await loadSchedulerStatus();
      appendLog("bridge.load_scheduler_status", result);
      if (!result.ok) {
        schedulerStatus.value = null;
        if (result.code !== "SCHEDULER_UNSUPPORTED") {
          pushActivity("读取定时任务状态失败", result.message || "无法读取桌面定时任务状态。");
        }
        return;
      }
      schedulerStatus.value = result.data || null;
    } catch (error) {
      schedulerStatus.value = null;
      appendLog("bridge.load_scheduler_status.failed", error instanceof Error ? error.message : String(error));
    }
  }

  async function handleSchedulerDailyTimesBlur() {
    await flushDraftSave();
    if (!settings.schedulerEnabled) {
      return;
    }
    if (settings.schedulerTriggerMode === "daily") {
      const dailyTimes = schedulerDailyTimesFromText(settings.schedulerDailyTimes);
      if (dailyTimes.length === 0) {
        showToast("请填写有效的每日定时时间", "error");
        return;
      }
      settings.schedulerDailyTimes = dailyTimes.join("\n");
    }
    const saved = await persistCurrentConfig({
      redirectOnMaskedToken: false,
      silentFailure: false,
      silentSuccess: true,
      skipIfUnchanged: true,
    });
    if (saved) {
      await refreshSchedulerStatus();
    }
  }

  return { schedulerStatus, refreshSchedulerStatus, handleSchedulerDailyTimesBlur };
}
