export const MOBILE_PROGRESS_REFRESH_INTERVAL_MS = 200;

export function shouldRefreshProgress(lastUpdatedAt: number, lastStage: string, stage: string, completed: number, total: number, now: number) {
  return stage !== lastStage || (total > 0 && completed >= total) || now - lastUpdatedAt >= MOBILE_PROGRESS_REFRESH_INTERVAL_MS;
}
