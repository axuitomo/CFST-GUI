// 视图导航（重构计划 §4.2）：从 App.vue 原样迁出。
// 离开 sources 页先静默保存（并发去重），离开 settings 页先自动保存；
// 桌面端非 Android 且未开减弱动效时走 View Transition 切视图。
import type { Ref } from "vue";
import type { ViewName } from "../lib/appTypes";

export interface NavigationDeps {
  autoSaveSettings: (reason: string) => Promise<boolean>;
  configHydrated: () => boolean;
  draftRestoring: () => boolean;
  isAndroidApp: () => boolean;
  saveSourcePageBeforeLeave: () => Promise<boolean>;
  selectedView: Ref<ViewName>;
  sourcePageHasUnsavedChanges: () => boolean;
}

export function useNavigation(deps: NavigationDeps) {
  let sourceLeaveSaveInFlight = false;
  let sourceLeaveSaveTarget: ViewName | null = null;

  function applyNavigation(view: ViewName) {
    if (deps.isAndroidApp() || view === deps.selectedView.value || window.matchMedia("(prefers-reduced-motion: reduce)").matches || typeof document.startViewTransition !== "function") {
      deps.selectedView.value = view;
      return;
    }
    void document
      .startViewTransition(() => {
        deps.selectedView.value = view;
      })
      .finished.catch(() => {});
  }

  function shouldSaveSourcesBeforeNavigation(target: ViewName) {
    if (!deps.configHydrated() || deps.draftRestoring()) {
      return false;
    }
    if (deps.selectedView.value !== "sources") {
      return false;
    }
    if (target === "sources") {
      return false;
    }
    return deps.sourcePageHasUnsavedChanges();
  }

  async function navigateTo(nextTarget: ViewName) {
    if (shouldSaveSourcesBeforeNavigation(nextTarget)) {
      sourceLeaveSaveTarget = nextTarget;
      if (sourceLeaveSaveInFlight) {
        return false;
      }

      sourceLeaveSaveInFlight = true;
      try {
        const saved = await deps.saveSourcePageBeforeLeave();
        if (saved && sourceLeaveSaveTarget) {
          applyNavigation(sourceLeaveSaveTarget);
        }
        return saved;
      } finally {
        sourceLeaveSaveInFlight = false;
        sourceLeaveSaveTarget = null;
      }
    }

    if (deps.selectedView.value === "settings" && nextTarget !== "settings") {
      await deps.autoSaveSettings("navigation");
    }

    applyNavigation(nextTarget);
    return true;
  }

  function changeSingleView(nextView: ViewName) {
    void navigateTo(nextView);
  }

  return { applyNavigation, changeSingleView, navigateTo };
}
