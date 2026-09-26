import type { TaskSnapshot, TaskTone } from "./bridge/types";

/**
 * 看板的乐观 UI 缓存。
 *
 * 冷启动时前端要等原生桥（Android 上还有 Go runtime 初始化与存储迁移）返回才能知道真实状态，
 * 这段时间看板只能显示初始值，看起来像「卡在空白界面」。这里把上一次已知的看板状态写进
 * `localStorage`，由 `App.vue` 在挂载前同步回填，让首帧就是有内容的看板，真实状态到达后覆盖。
 *
 * 只存展示数据，不存运行态：任务动作按钮在同步期间保持禁用（`useProbeTask` 的
 * `startupSyncing`），不会拿缓存值判断任务是否在跑；任务身份也只从缓存的快照里取，
 * 没有快照就不显示任务号，避免缓存里的任务号比快照活得更久。
 */
export const DASHBOARD_CACHE_STORAGE_KEY = "cfst.dashboard.state.v1";

const ACTIVITY_LIMIT = 10;

const TASK_TONES: TaskTone[] = ["idle", "preparing", "running", "partial", "cooling", "warning", "completed", "no_results", "cancelled", "failed"];

export interface DashboardCacheActivity {
  detail: string;
  title: string;
  ts: string;
}

export interface DashboardCacheStatus {
  detail: string;
  title: string;
  tone: TaskTone;
}

export interface DashboardCacheSummary {
  accepted: number;
  exported: number;
  failed: number;
  filtered: number;
  invalid: number;
  passed: number;
  processed: number;
  total: number;
}

export interface DashboardCache {
  activityFeed: DashboardCacheActivity[];
  status: DashboardCacheStatus;
  summary: DashboardCacheSummary;
  taskSnapshot: TaskSnapshot | null;
  updatedAt: string;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function asText(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function asCount(value: unknown): number {
  const numeric = Number(value);
  return Number.isFinite(numeric) && numeric > 0 ? Math.floor(numeric) : 0;
}

function readActivityFeed(value: unknown): DashboardCacheActivity[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .filter(isRecord)
    .map((entry) => ({ detail: asText(entry.detail), title: asText(entry.title), ts: asText(entry.ts) }))
    .filter((entry) => Boolean(entry.title || entry.detail))
    .slice(0, ACTIVITY_LIMIT);
}

function readStatus(value: unknown): DashboardCacheStatus {
  const record = isRecord(value) ? value : {};
  const tone = TASK_TONES.includes(record.tone as TaskTone) ? (record.tone as TaskTone) : "idle";
  return { detail: asText(record.detail), title: asText(record.title), tone };
}

function readSummary(value: unknown): DashboardCacheSummary {
  const record = isRecord(value) ? value : {};
  return {
    accepted: asCount(record.accepted),
    exported: asCount(record.exported),
    failed: asCount(record.failed),
    filtered: asCount(record.filtered),
    invalid: asCount(record.invalid),
    passed: asCount(record.passed),
    processed: asCount(record.processed),
    total: asCount(record.total),
  };
}

function readTaskSnapshot(value: unknown): TaskSnapshot | null {
  // 快照字段多且只用于展示（视图侧全部可选链读取），这里只确认它是本应用写下的任务快照。
  if (!isRecord(value) || !asText(value.task_id).trim() || !asText(value.status).trim()) {
    return null;
  }
  return value as unknown as TaskSnapshot;
}

/** 读取缓存的看板状态；缺失、损坏或没有展示价值时返回 null，调用方按无缓存处理。 */
export function readDashboardCache(): DashboardCache | null {
  let raw: string | null = null;
  try {
    raw = window.localStorage.getItem(DASHBOARD_CACHE_STORAGE_KEY);
  } catch {
    // 隐私模式等场景下存储不可用：退化成没有缓存。
    return null;
  }
  if (!raw) {
    return null;
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!isRecord(parsed) || !asText(parsed.updatedAt).trim()) {
    return null;
  }

  const status = readStatus(parsed.status);
  const summary = readSummary(parsed.summary);
  const activityFeed = readActivityFeed(parsed.activityFeed);
  if (!status.title && activityFeed.length === 0 && !Object.values(summary).some((entry) => entry > 0)) {
    return null;
  }

  return {
    activityFeed,
    status,
    summary,
    taskSnapshot: readTaskSnapshot(parsed.taskSnapshot),
    updatedAt: asText(parsed.updatedAt),
  };
}

export function writeDashboardCache(cache: DashboardCache): void {
  try {
    window.localStorage.setItem(DASHBOARD_CACHE_STORAGE_KEY, JSON.stringify(cache));
  } catch {
    // 存储不可用或配额不足：只影响下次冷启动的首帧内容。
  }
}
