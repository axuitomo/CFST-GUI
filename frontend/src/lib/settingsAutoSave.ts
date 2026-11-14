export const SETTINGS_INTERACTION_SAVE_DELAY_MS = 200;

/**
 * 设置页的点击与失焦会连发。合并成一个延迟任务，避免每次点击都重建整份配置快照；
 * 离开页面时用 takePending 取消挂起的合并并按需立即保存。
 */
export function createSettingsAutoSaver(run: (reason: string) => void, delayMs = SETTINGS_INTERACTION_SAVE_DELAY_MS) {
  let timer: ReturnType<typeof setTimeout> | undefined;

  return {
    schedule(reason: string) {
      if (timer !== undefined) clearTimeout(timer);
      timer = setTimeout(() => {
        timer = undefined;
        run(reason);
      }, delayMs);
    },
    takePending() {
      if (timer === undefined) return false;
      clearTimeout(timer);
      timer = undefined;
      return true;
    },
  };
}
