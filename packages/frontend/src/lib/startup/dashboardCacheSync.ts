// 看板乐观 UI 缓存（冷启动首帧回填 + 节流回写），从 useStartupSync 拆出。
import { watch, type Ref } from "vue";
import type { TaskSnapshot } from "../bridge";
import { readDashboardCache, writeDashboardCache, type DashboardCacheStatus, type DashboardCacheSummary } from "../results/dashboardCache";
import { ACTIVITY_FEED_LIMIT, type ActivityEntry } from "../results/activityFeed";

export interface DashboardCacheSyncOptions {
  status: DashboardCacheStatus;
  summary: DashboardCacheSummary;
  activityFeed: Ref<ActivityEntry[]>;
  taskSnapshot: Ref<TaskSnapshot | null>;
  task: { taskId: string };
  startupSyncing: Ref<boolean>;
}

export function createDashboardCacheSync(options: DashboardCacheSyncOptions) {
  const { status, summary, activityFeed, taskSnapshot, task, startupSyncing } = options;
  let dashboardCacheTimer: number | undefined;
  let dashboardCachePending = false;

  // 乐观 UI：挂载前用上次缓存的看板展示状态填充首帧，冷启动时不再先出现一块空看板。
  // 只填展示数据（状态、统计、动态、任务快照），不填运行态：任务动作在 startupSyncing 期间
  // 保持禁用，缓存值不需要专门清理：真实状态到达时会逐项覆盖它。
  function restoreDashboardCache() {
    const cached = readDashboardCache();
    if (!cached) {
      return;
    }
    Object.assign(status, cached.status);
    Object.assign(summary, cached.summary);
    activityFeed.value = cached.activityFeed;
    // 快照用于展示任务上下文（端口等）；任务号只从它里面取，没有快照就不显示任务号，
    // 不让缓存里的任务号比快照活得更久。
    taskSnapshot.value = cached.taskSnapshot;
    task.taskId = cached.taskSnapshot?.task_id || "";
    startupSyncing.value = true;
  }

  // 写缓存要节流：探测事件会高频改动 task 与 activityFeed，逐次写 localStorage 会拖慢主线程。
  function scheduleDashboardCacheSave() {
    if (startupSyncing.value || dashboardCachePending) {
      return;
    }
    dashboardCachePending = true;
    dashboardCacheTimer = window.setTimeout(() => {
      dashboardCacheTimer = undefined;
      dashboardCachePending = false;
      writeDashboardCache({
        activityFeed: activityFeed.value.slice(0, ACTIVITY_FEED_LIMIT),
        status: { ...status },
        summary: { ...summary },
        taskSnapshot: taskSnapshot.value,
        updatedAt: new Date().toISOString(),
      });
    }, 1000);
  }

  function disposeDashboardCacheTimer() {
    if (dashboardCacheTimer !== undefined) {
      window.clearTimeout(dashboardCacheTimer);
    }
  }

  // 看板展示状态的回写：状态、统计、任务身份与最近动态任一变化都会调度一次节流写入。
  watch([() => status.title, () => status.detail, () => status.tone, summary, task, taskSnapshot, activityFeed], scheduleDashboardCacheSave, { deep: true });

  return { restoreDashboardCache, scheduleDashboardCacheSave, disposeDashboardCacheTimer };
}
