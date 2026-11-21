// Android select 仿真 + 键盘视口跟踪（重构计划 §4.2）：从 App.vue 原样迁出。
// Android WebView 的原生 select 弹窗不可控，用底部面板仿真替代；
// 键盘弹出时通过 visualViewport 与基线高度差计算 inset，写入 CSS 变量与 dataset。
import { reactive } from "vue";
import { computeAndroidViewportState, type AndroidViewportState } from "../lib/androidViewport";

export interface AndroidSelectOption {
  disabled: boolean;
  label: string;
  selected: boolean;
  value: string;
}

export function useAndroidSelectPicker(deps: { isAndroidApp: () => boolean }) {
  const androidSelectPicker = reactive({
    open: false,
    options: [] as AndroidSelectOption[],
    title: "选择",
    value: "",
  });
  let androidViewportFrame: number | undefined;
  let androidViewportTrackingInstalled = false;
  let androidViewportState: AndroidViewportState | null = null;
  // 无键盘时的视口高度，键盘判定拿它当基线（见 lib/androidViewport）。
  let androidViewportBaselineHeight = 0;
  let androidSelectElement: HTMLSelectElement | null = null;
  const androidSelectCaptureOptions = { capture: true, passive: false } as const;

  // androidTextControlFocused 判断焦点是否在文本输入控件上：只有这时键盘才可能弹出，
  // 基线高度也就不能跟着收缩，否则 adjustResize 下永远测不出键盘。
  function androidTextControlFocused() {
    const active = document.activeElement;
    if (active instanceof HTMLTextAreaElement) {
      return true;
    }
    if (active instanceof HTMLInputElement) {
      return !["button", "checkbox", "file", "radio", "reset", "submit"].includes(active.type);
    }
    return active instanceof HTMLElement && active.isContentEditable;
  }

  function applyAndroidViewportState() {
    if (typeof window === "undefined" || !deps.isAndroidApp()) {
      return;
    }
    const visualViewport = window.visualViewport;
    const nextState = computeAndroidViewportState(androidViewportBaselineHeight, {
      editing: androidTextControlFocused(),
      innerHeight: window.innerHeight,
      visualHeight: visualViewport?.height ?? 0,
      visualTop: visualViewport?.offsetTop ?? 0,
    });
    androidViewportBaselineHeight = nextState.baselineHeight;
    document.documentElement.dataset.cfstAndroid = "true";
    if (androidViewportState?.keyboardInset !== nextState.keyboardInset) {
      document.documentElement.style.setProperty("--cfst-keyboard-inset-bottom", `${nextState.keyboardInset}px`);
    }
    if (androidViewportState?.keyboardOpen !== nextState.keyboardOpen) {
      document.documentElement.dataset.cfstAndroidKeyboard = nextState.keyboardOpen ? "open" : "closed";
    }
    androidViewportState = nextState;
  }

  function closeAndroidSelectPicker() {
    androidSelectPicker.open = false;
    androidSelectPicker.options = [];
    androidSelectPicker.value = "";
    androidSelectElement = null;
  }

  function androidSelectTitle(selectElement: HTMLSelectElement) {
    const labelElement = selectElement.closest("label");
    const labelText = labelElement?.querySelector(".ui-label")?.textContent || "";
    return labelText.trim() || selectElement.getAttribute("aria-label") || selectElement.name || "选择";
  }

  function openAndroidSelectPicker(selectElement: HTMLSelectElement) {
    if (selectElement.disabled) {
      return;
    }
    androidSelectElement = selectElement;
    androidSelectPicker.title = androidSelectTitle(selectElement);
    androidSelectPicker.value = selectElement.value;
    androidSelectPicker.options = Array.from(selectElement.options).map((option) => ({
      disabled: option.disabled,
      label: option.label || option.textContent?.trim() || option.value,
      selected: option.selected,
      value: option.value,
    }));
    androidSelectPicker.open = true;
  }

  function handleAndroidSelectPointerDown(event: PointerEvent) {
    if (!deps.isAndroidApp()) {
      return;
    }
    const target = event.target;
    if (!(target instanceof Element)) {
      return;
    }
    const selectElement = target.closest("select");
    if (!(selectElement instanceof HTMLSelectElement)) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    selectElement.blur();
    openAndroidSelectPicker(selectElement);
  }

  function handleAndroidSelectTouchStart(event: TouchEvent) {
    if (!deps.isAndroidApp()) {
      return;
    }
    const target = event.target;
    if (!(target instanceof Element)) {
      return;
    }
    const selectElement = target.closest("select");
    if (!(selectElement instanceof HTMLSelectElement)) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    selectElement.blur();
    openAndroidSelectPicker(selectElement);
  }

  function handleAndroidSelectClick(event: MouseEvent) {
    if (!deps.isAndroidApp()) {
      return;
    }
    const target = event.target;
    if (!(target instanceof Element)) {
      return;
    }
    const selectElement = target.closest("select");
    if (!(selectElement instanceof HTMLSelectElement)) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    openAndroidSelectPicker(selectElement);
  }

  function chooseAndroidSelectOption(option: AndroidSelectOption) {
    const selectElement = androidSelectElement;
    if (!selectElement || option.disabled) {
      return;
    }
    selectElement.value = option.value;
    selectElement.dispatchEvent(new Event("input", { bubbles: true }));
    selectElement.dispatchEvent(new Event("change", { bubbles: true }));
    closeAndroidSelectPicker();
  }

  function handleAndroidSelectKeydown(event: KeyboardEvent) {
    if (event.key === "Escape" && androidSelectPicker.open) {
      closeAndroidSelectPicker();
    }
  }

  function scheduleAndroidViewportState() {
    if (androidViewportFrame !== undefined) {
      window.cancelAnimationFrame(androidViewportFrame);
    }
    androidViewportFrame = window.requestAnimationFrame(() => {
      androidViewportFrame = undefined;
      applyAndroidViewportState();
    });
  }

  function handleAndroidControlFocus() {
    scheduleAndroidViewportState();
  }

  function installAndroidViewportTracking() {
    if (androidViewportTrackingInstalled || typeof window === "undefined") {
      return;
    }
    androidViewportTrackingInstalled = true;
    applyAndroidViewportState();
    window.addEventListener("resize", scheduleAndroidViewportState);
    window.addEventListener("focusin", handleAndroidControlFocus);
    window.addEventListener("focusout", scheduleAndroidViewportState);
    document.addEventListener("touchstart", handleAndroidSelectTouchStart, androidSelectCaptureOptions);
    document.addEventListener("pointerdown", handleAndroidSelectPointerDown, true);
    document.addEventListener("click", handleAndroidSelectClick, true);
    document.addEventListener("keydown", handleAndroidSelectKeydown);
    window.visualViewport?.addEventListener("resize", scheduleAndroidViewportState);
  }

  function uninstallAndroidViewportTracking() {
    if (!androidViewportTrackingInstalled || typeof window === "undefined") {
      return;
    }
    androidViewportTrackingInstalled = false;
    window.removeEventListener("resize", scheduleAndroidViewportState);
    window.removeEventListener("focusin", handleAndroidControlFocus);
    window.removeEventListener("focusout", scheduleAndroidViewportState);
    document.removeEventListener("touchstart", handleAndroidSelectTouchStart, androidSelectCaptureOptions);
    document.removeEventListener("pointerdown", handleAndroidSelectPointerDown, true);
    document.removeEventListener("click", handleAndroidSelectClick, true);
    document.removeEventListener("keydown", handleAndroidSelectKeydown);
    window.visualViewport?.removeEventListener("resize", scheduleAndroidViewportState);
    if (androidViewportFrame !== undefined) {
      window.cancelAnimationFrame(androidViewportFrame);
      androidViewportFrame = undefined;
    }
    document.documentElement.style.removeProperty("--cfst-keyboard-inset-bottom");
    androidViewportState = null;
    androidViewportBaselineHeight = 0;
    delete document.documentElement.dataset.cfstAndroid;
    delete document.documentElement.dataset.cfstAndroidKeyboard;
    closeAndroidSelectPicker();
  }

  return {
    androidSelectPicker,
    chooseAndroidSelectOption,
    closeAndroidSelectPicker,
    installAndroidViewportTracking,
    uninstallAndroidViewportTracking,
  };
}
