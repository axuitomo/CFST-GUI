// COLO 词典域（重构计划 §4.2）：从 App.vue 原样迁出。
import { ref } from "vue";
import { loadColoDictionaryStatus, processColoDictionary, updateColoDictionary, type ColoDictionaryStatus, type TaskTone } from "../lib/bridge";
import type { ToastTone } from "../lib/appTypes";
export interface UseColoDictionaryDeps {
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  setStatus: (next: { title: string; detail: string; tone: TaskTone }) => void;
}
export function useColoDictionary(deps: UseColoDictionaryDeps) {
  const { appendLog, showToast, setStatus } = deps;
  const coloDictionaryStatus = ref<ColoDictionaryStatus | null>(null);


  const coloDictionaryProcessing = ref(false);


  const coloDictionaryUpdating = ref(false);


  async function refreshColoDictionaryStatus() {
    try {
      const result = await loadColoDictionaryStatus();
      appendLog("bridge.load_colo_dictionary_status", result);
      if (!result.ok) {
        showToast(result.message || "读取 COLO 词典状态失败", "error");
        return;
      }
      coloDictionaryStatus.value = result.data || null;
    } catch (error) {
      showToast(error instanceof Error ? error.message : "读取 COLO 词典状态失败", "error");
    }
  }

  async function refreshColoDictionary() {
    coloDictionaryUpdating.value = true;
    try {
      const result = await updateColoDictionary({});
      appendLog("bridge.update_colo_dictionary", result);
      if (!result.ok) {
        setStatus({
          detail: result.message || "拉取 COLO 原始词典失败。",
          title: "词典拉取失败",
          tone: "failed",
        });
        showToast("拉取 COLO 原始词典失败", "error");
        return;
      }
      coloDictionaryStatus.value = result.data || null;
      setStatus({
        detail: result.message || "COLO 原始词典已拉取，可继续本地处理。",
        title: "COLO 原始词典已拉取",
        tone: "idle",
      });
      showToast("COLO 原始词典已拉取", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "拉取 COLO 原始词典失败", "error");
    } finally {
      coloDictionaryUpdating.value = false;
    }
  }

  async function processLocalColoDictionary() {
    coloDictionaryProcessing.value = true;
    try {
      const result = await processColoDictionary({});
      appendLog("bridge.process_colo_dictionary", result);
      if (!result.ok) {
        setStatus({
          detail: result.message || "处理 COLO 词典失败。",
          title: "词典处理失败",
          tone: "failed",
        });
        showToast("处理 COLO 词典失败", "error");
        return;
      }
      coloDictionaryStatus.value = result.data || null;
      setStatus({
        detail: result.message || "COLO 词典已本地处理。",
        title: "COLO 词典已处理",
        tone: "idle",
      });
      showToast("COLO 词典已处理", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "处理 COLO 词典失败", "error");
    } finally {
      coloDictionaryProcessing.value = false;
    }
  }

  return { coloDictionaryStatus, coloDictionaryProcessing, coloDictionaryUpdating, refreshColoDictionaryStatus, refreshColoDictionary, processLocalColoDictionary };
}
