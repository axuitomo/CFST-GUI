export interface DownloadSpeedRow {
  averageSpeedMbS: number | null;
  bytesRead: number;
  colo: string;
  currentSpeedMbS: number | null;
  elapsedMs: number;
  ip: string;
  updatedAt: number;
}

export const DOWNLOAD_SPEED_ROW_LIMIT = 16;

const DOWNLOAD_SPEED_ROW_MIN_TTL_MS = 6000;

/** 采样间隔可配，TTL 至少留三个采样窗口，避免慢采样时行刚写入就被删掉。 */
export function downloadSpeedRowTtlMs(sampleIntervalMs: number) {
  const interval = Number.isFinite(sampleIntervalMs) && sampleIntervalMs > 0 ? sampleIntervalMs : 1000;
  return Math.max(DOWNLOAD_SPEED_ROW_MIN_TTL_MS, interval * 3);
}

export function pruneDownloadSpeedRows(rows: DownloadSpeedRow[], now: number, ttlMs: number) {
  for (let index = rows.length - 1; index >= 0; index -= 1) {
    if (now - rows[index].updatedAt > ttlMs) {
      rows.splice(index, 1);
    }
  }
}

export function upsertDownloadSpeedRow(rows: DownloadSpeedRow[], row: DownloadSpeedRow, options: { limit?: number; now: number; ttlMs: number }) {
  pruneDownloadSpeedRows(rows, options.now, options.ttlMs);
  const existing = rows.find((item) => item.ip === row.ip);
  if (existing) {
    Object.assign(existing, row);
    return;
  }
  rows.push(row);
  const limit = options.limit ?? DOWNLOAD_SPEED_ROW_LIMIT;
  if (rows.length > limit) {
    rows.splice(0, rows.length - limit);
  }
}

export function clearDownloadSpeedRows(rows: DownloadSpeedRow[]) {
  rows.splice(0, rows.length);
}
