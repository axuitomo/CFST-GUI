import { computed, ref, watch, type Ref } from "vue";
import { queryResultHistory, type ProbeResult, type ProbeResultIPFilter, type ProbeResultOrder, type ProbeResultSortBy, type TaskSnapshot } from "../lib/bridge";
import { buildResultDisplay, migrateLegacyResultHistory, type ResultHistoryRecord, type ResultTimeRange } from "../lib/results/history";

export interface ResultDisplayDeps {
  taskSnapshot: Ref<TaskSnapshot | null>;
  resultWorkspaceTaskId: Ref<string>;
  resultRows: Ref<ProbeResult[]>;
  resultIpFilter: Ref<ProbeResultIPFilter>;
  resultOrder: Ref<ProbeResultOrder>;
  resultSortBy: Ref<ProbeResultSortBy>;
  resultsTotalCount: Ref<number>;
  appendLog: (event: string, payload: unknown) => void;
}

export function useResultDisplay(deps: ResultDisplayDeps) {
  // 结果页展示管道:时间范围 → 跨天去重 → IP/国家筛选 → 排序。历史由后端持久化,这里只保留展示镜像,导出与复制共用同一份展示行。
  const resultTimeRange = ref<ResultTimeRange>("current");
  const resultMergeDuplicates = ref(true);
  const resultCountryFilter = ref("");
  const resultDisplayNow = ref(Date.now());
  // 历史由后端保存(桌面/WebUI/Android 共用一份):这里维护展示所需的本地镜像,
  // 只在切到历史范围或写入新批次后按需拉取,「本次结果」范围完全不读历史。
  const resultHistoryRecords = ref<ResultHistoryRecord[]>([]);
  let resultHistoryInFlight = false;
  let resultHistoryPending = false;

  async function refreshResultHistory() {
    if (resultTimeRange.value === "current") {
      return;
    }
    if (resultHistoryInFlight) {
      // 合并并发请求:写完一批结果紧接着又切了范围时,只补一次刷新。
      resultHistoryPending = true;
      return;
    }
    resultHistoryInFlight = true;
    try {
      do {
        resultHistoryPending = false;
        const result = await queryResultHistory();
        deps.appendLog("bridge.result_history", result);
        if (!result.ok || !result.data) {
          return;
        }
        resultHistoryRecords.value = result.data.records;
      } while (resultHistoryPending);
    } finally {
      resultHistoryInFlight = false;
    }
  }

  // 旧版把历史放在各宿主自己的 localStorage 里:首启灌进后端存储,成功后删旧键;
  // 失败留待下次启动重试,后端按键与时间戳去重,重复导入不会产生重复记录。
  async function migrateResultHistory() {
    const result = await migrateLegacyResultHistory();
    if (!result) {
      return;
    }
    deps.appendLog("bridge.result_history.import", result);
    if (result.ok) {
      void refreshResultHistory();
    }
  }

  // 单独包一层 computed：快照被替换但 updated_at 没变时，下游展示管道不再重算。
  const currentBatchTimestamp = computed(() => Date.parse(deps.taskSnapshot.value?.updated_at || "") || undefined);
  const resultDisplay = computed(() =>
    buildResultDisplay({
      countryFilter: resultCountryFilter.value,
      currentBatchId: deps.resultWorkspaceTaskId.value,
      currentBatchTimestamp: currentBatchTimestamp.value,
      currentRows: deps.resultRows.value,
      historyRecords: resultHistoryRecords.value,
      ipFilter: deps.resultIpFilter.value,
      mergeDuplicates: resultMergeDuplicates.value,
      now: resultDisplayNow.value,
      order: deps.resultOrder.value,
      range: resultTimeRange.value,
      sortBy: deps.resultSortBy.value,
    }),
  );
  // 当前范围走后端分页总数；历史范围是本地全量，桌面表格窗口化后不再显示"继续加载"。
  const resultViewTotalCount = computed(() => (resultTimeRange.value === "current" ? deps.resultsTotalCount.value : resultDisplay.value.rows.length));

  function updateResultTimeRange(range: ResultTimeRange) {
    resultTimeRange.value = range;
    resultDisplayNow.value = Date.now();
    void refreshResultHistory();
  }

  function updateResultMergeDuplicates(value: boolean) {
    resultMergeDuplicates.value = value;
    resultDisplayNow.value = Date.now();
  }

  function updateResultCountryFilter(value: string) {
    resultCountryFilter.value = value;
  }

  function refreshResultDisplayNow() {
    resultDisplayNow.value = Date.now();
  }

  watch(
    () => resultDisplay.value.countryOptions,
    (options) => {
      if (resultCountryFilter.value && !options.some((option) => option.value === resultCountryFilter.value)) {
        resultCountryFilter.value = "";
      }
    },
  );
  // 新结果落地后把「近 N 天」的时间窗推到当下，常驻页面跨天也不会停在旧的 now 上。
  watch(
    () => deps.resultRows.value,
    () => {
      resultDisplayNow.value = Date.now();
    },
  );

  return {
    migrateResultHistory,
    refreshResultDisplayNow,
    refreshResultHistory,
    resultCountryFilter,
    resultDisplay,
    resultMergeDuplicates,
    resultTimeRange,
    resultViewTotalCount,
    updateResultCountryFilter,
    updateResultMergeDuplicates,
    updateResultTimeRange,
  };
}
