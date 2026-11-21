// Toast 队列（重构计划 §4.2）：从 App.vue 原样迁出，3200ms 自动消失，id 单调递增。
import { ref } from "vue";
import type { ToastEntry, ToastTone } from "../lib/appTypes";

export function useToasts() {
  const toasts = ref<ToastEntry[]>([]);
  let toastId = 0;

  function showToast(message: string, tone: ToastTone = "success") {
    const nextId = toastId;
    toastId += 1;
    toasts.value.push({
      id: nextId,
      message,
      tone,
    });
    setTimeout(() => {
      toasts.value = toasts.value.filter((toast) => toast.id !== nextId);
    }, 3200);
  }

  return { showToast, toasts };
}
