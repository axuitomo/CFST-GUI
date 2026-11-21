import type { TaskTone } from "./enums";

export interface DerivedTaskState {
  detail: string;
  title: string;
  tone: TaskTone;
}

export interface TaskProgress {
  failed: number;
  passed: number;
  processed: number;
  stage: string;
  total?: number | null;
}

export interface MCISProgress {
  candidate_count: number;
  completed: number;
  concurrency?: number | null;
  elapsed_ms?: number | null;
  failed: number;
  last_colo?: string | null;
  last_ip?: string | null;
  last_ok: boolean;
  source_id?: string | null;
  source_name?: string | null;
  succeeded: number;
  total: number;
}

export interface ExportRecord {
  file_name: string;
  format: string;
  last_write_at?: string | null;
  source_path?: string | null;
  target_dir: string;
  task_id: string;
  written_count: number;
}

export interface TaskSnapshot {
  completed_at?: string | null;
  config_digest?: string | null;
  current_stage?: string | null;
  export_record?: ExportRecord | null;
  failure_summary?: Record<string, unknown> | null;
  mcis_progress?: MCISProgress | null;
  progress?: TaskProgress | null;
  resume_capable?: boolean | null;
  runtime_attached?: boolean | null;
  session_state?: string | null;
  started_at?: string | null;
  status: string;
  task_context?: Record<string, unknown> | null;
  task_id: string;
  updated_at: string;
}

export interface TaskSnapshotList {
  count: number;
  items: TaskSnapshot[];
}

export interface TaskResultPage {
  count: number;
  results: ProbeResult[];
  source_kind?: string | null;
  source_path?: string | null;
  total_count?: number | null;
}

export interface ProbeResult {
  address: string;
  colo?: string | null;
  download_mbps?: number | null;
  export_status: string;
  last_error_code?: string | null;
  max_download_mbps?: number | null;
  note?: string | null;
  source_port?: number | null;
  stage_status: string;
  tcp_latency_ms?: number | null;
  test_port?: number | null;
  trace_latency_ms?: number | null;
}

export interface UploadNotification {
  cloudflare_status?: string;
  cloudflare_upload_count?: number;
  created_at: string;
  github_status?: string;
  github_upload_count?: number;
  message: string;
  source: string;
  status: string;
  task_id?: string;
  top_entries?: UploadNotificationTopEntry[];
}

export interface UploadNotificationTopEntry {
  colo?: string;
  download_speed_mb?: number;
  ip: string;
  rank: number;
  tcp_delay_ms?: number;
  trace_delay_ms?: number;
}

export interface ProbeEventEnvelope {
  event: string;
  payload: Record<string, unknown>;
  schema_version: string;
  seq: number;
  task_id: string;
  ts: string;
}

export interface ProbeRunResultPayload extends Record<string, unknown> {
  outputFile?: unknown;
  results?: unknown;
  source?: unknown;
  sourceStatuses?: unknown;
  startedAt?: unknown;
  summary?: unknown;
  task_context?: unknown;
  taskContext?: unknown;
  upload_notification?: UploadNotification | null;
  warnings?: unknown;
}
