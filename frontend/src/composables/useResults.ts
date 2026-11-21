// 结果工作区域（重构计划 §4.2）：从 App.vue 原样迁出。
import { ref, type Ref } from "vue";
import { listTaskResults, type ProbeResult, type ProbeResultIPFilter, type ProbeResultOrder, type ProbeResultSortBy, type TaskSnapshot } from "../lib/bridge";
import { asCount } from "../lib/valueCoerce";

export interface ResultsDeps {
  task: { exportPath: string; taskId: string };
  taskSnapshot: Ref<TaskSnapshot | null>;
  buildResultFileFallbackPayload: (exportPath: string, snapshot?: TaskSnapshot | null) => { path: string };
  refreshTaskData: (taskId?: string) => Promise<void>;
  appendLog: (event: string, payload: unknown) => void;
}

export function useResults(deps: ResultsDeps) {
  const resultIpFilter = ref<ProbeResultIPFilter>("all");
  const resultOrder = ref<ProbeResultOrder>("desc");
  const resultRows = ref<ProbeResult[]>([]);
  const taskResultCSVPath = ref("");
  const resultWorkspaceTaskId = ref("");
  const resultSortBy = ref<ProbeResultSortBy>("download");
  const resultsLoading = ref(false);
  const resultsPageLimit = ref(200);
  const resultsTotalCount = ref(0);

  const resultIpFilterOptions: Array<{ label: string; value: ProbeResultIPFilter }> = [
    { label: "全部 IP", value: "all" },
    { label: "仅 IPv4", value: "ipv4" },
    { label: "仅 IPv6", value: "ipv6" },
  ];
  const resultSortOptions: Array<{ label: string; value: ProbeResultSortBy }> = [
    { label: "TCP 延迟", value: "tcp" },
    { label: "追踪延迟", value: "trace" },
    { label: "平均速率", value: "download" },
    { label: "最高速率", value: "max_download" },
  ];

  function clearCurrentTaskResultWorkspace(options: { preserveSnapshot?: boolean } = {}) {
    resultRows.value = [];
    taskResultCSVPath.value = "";
    resultsTotalCount.value = 0;
    resultWorkspaceTaskId.value = "";
    if (!options.preserveSnapshot) {
      deps.taskSnapshot.value = null;
    }
  }

  function applyCurrentTaskResultWorkspace(taskId: string, rows: ProbeResult[], totalCount: number) {
    resultWorkspaceTaskId.value = taskId.trim();
    resultRows.value = rows;
    resultsTotalCount.value = asCount(totalCount, rows.length);
  }

  function resetCurrentResultPage() {
    resultRows.value = [];
    resultsTotalCount.value = 0;
  }

  function updateResultSort(sortBy: ProbeResultSortBy) {
    if (resultSortBy.value === sortBy) {
      resultOrder.value = resultOrder.value === "asc" ? "desc" : "asc";
    } else {
      resultSortBy.value = sortBy;
      resultOrder.value = sortBy === "download" || sortBy === "max_download" ? "desc" : "asc";
    }

    resetCurrentResultPage();
    void deps.refreshTaskData();
  }

  function updateResultIpFilter(filter: ProbeResultIPFilter) {
    if (resultIpFilter.value === filter) {
      return;
    }
    resultIpFilter.value = filter;
    resetCurrentResultPage();
    void deps.refreshTaskData();
  }

  function updateResultOrder(order: ProbeResultOrder) {
    if (resultOrder.value === order) {
      return;
    }
    resultOrder.value = order;
    resetCurrentResultPage();
    void deps.refreshTaskData();
  }

  function currentVisibleResultRows() {
    return [...resultRows.value];
  }

  function refreshCurrentTaskData() {
    void deps.refreshTaskData();
  }

  async function loadMoreResults() {
    const normalizedTaskId = deps.task.taskId.trim();
    if (!normalizedTaskId || resultRows.value.length >= resultsTotalCount.value) {
      return;
    }
    resultsLoading.value = true;
    try {
      const result = await listTaskResults(normalizedTaskId, resultSortBy.value, resultOrder.value, deps.buildResultFileFallbackPayload(deps.task.exportPath, deps.taskSnapshot.value), resultIpFilter.value, {
        limit: resultsPageLimit.value,
        offset: resultRows.value.length,
      });
      deps.appendLog("bridge.list_task_results.more", result);
      if (!result.ok || !result.data) {
        return;
      }
      const nextRows = Array.isArray(result.data.results) ? result.data.results : [];
      resultWorkspaceTaskId.value = normalizedTaskId;
      resultRows.value = [...resultRows.value, ...nextRows];
      resultsTotalCount.value = asCount(result.data.total_count, resultRows.value.length);
    } finally {
      resultsLoading.value = false;
    }
  }

  return { resultIpFilter, resultOrder, resultRows, taskResultCSVPath, resultWorkspaceTaskId, resultSortBy, resultsLoading, resultsPageLimit, resultsTotalCount, resultIpFilterOptions, resultSortOptions, clearCurrentTaskResultWorkspace, applyCurrentTaskResultWorkspace, resetCurrentResultPage, updateResultSort, updateResultIpFilter, updateResultOrder, currentVisibleResultRows, refreshCurrentTaskData, loadMoreResults };
}
