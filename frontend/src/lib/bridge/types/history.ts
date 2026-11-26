// 结果页跨批次历史(近 30 天)的 wire 类型:持久化与合并都在后端 appcore,
// 前端只按这些字段做展示管道,字段名与 internal/appcore/result_history.go 一一对应。
export interface ResultHistoryRecord {
  address: string;
  batch_id: string;
  colo: string;
  download_mbps: number | null;
  max_download_mbps: number | null;
  source_port: number | null;
  stage_status: string;
  tcp_latency_ms: number | null;
  test_port: number | null;
  timestamp: number;
  trace_latency_ms: number | null;
}

export interface ResultHistoryMergeResult {
  changed: boolean;
  dropped: number;
  merged: number;
  records: number;
  skipped: number;
}

export interface ResultHistoryPage {
  records: ResultHistoryRecord[];
  /** 后端文件内的记录总数,含读路径按保留期过滤掉的过期记录。 */
  total_count: number;
}
