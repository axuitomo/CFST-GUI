// 命令信封、应用/存储/运行时诊断等跨域基础类型。
export interface CommandResult<T = Record<string, unknown> | null> {
  code: string;
  data: T | null;
  message: string;
  ok: boolean;
  schema_version: string;
  task_id: string | null;
  warnings: string[];
}

export interface AppInfo {
  current_version: string;
  install_mode: string;
  platform: string;
  release_url: string;
}

export interface UpdateInfo extends AppInfo {
  asset_name: string;
  download_url: string;
  docker_image: string;
  latest_version: string;
  release_name: string;
  sha256: string;
  update_available: boolean;
}

export interface UpdateInstallResult extends UpdateInfo {
  downloaded_path: string;
  install_started: boolean;
  next_action: string;
}

export interface PathSelectionPayload {
  androidExportUri?: string;
  canceled?: boolean;
  content?: string;
  content_base64?: string;
  directory?: string;
  display_name?: string;
  file_name?: string;
  mode?: string;
  path?: string;
  storage_uri?: string;
  target_uri?: string;
  uri?: string;
}

export interface StorageHealth {
  checked_at: string;
  exists: boolean;
  free_bytes: number;
  is_dir: boolean;
  message: string;
  path: string;
  portable_mode: boolean;
  writable: boolean;
}

export interface StorageStatus {
  backend?: "private";
  bootstrap_path: string;
  current_dir: string;
  default_dir: string;
  display_name?: string;
  health?: StorageHealth;
  last_sync_at?: string;
  last_sync_error?: string;
  log_uri?: string;
  permission_ok?: boolean;
  portable_mode: boolean;
  runtime_dir?: string;
  setup_completed: boolean;
  setup_required: boolean;
  storage_uri?: string;
  writable: boolean;
}

export interface RuntimeDiagnostics {
  cleanup_count?: number;
  diagnostics_enabled: boolean;
  goroutines?: number;
  heap_alloc_bytes?: number;
  heap_inuse_bytes?: number;
  heap_sys_bytes?: number;
  heavy_cleanup_count?: number;
  last_cleanup_at?: string;
  last_cleanup_reason?: string;
  last_heavy_cleanup_at?: string;
  last_skipped_heavy_at?: string;
  last_skipped_heavy_reason?: string;
  memory_sys_bytes?: number;
  task_snapshots?: number;
}

export interface TraceDiagnosticSample {
  colo?: string;
  error?: string;
  ip?: string;
  reason: string;
  retry_after_ms?: number;
  status_code?: number;
  url?: string;
}

export interface TraceDiagnostics {
  reason_counts?: Record<string, number>;
  samples?: TraceDiagnosticSample[];
  status_counts?: Record<string, number>;
  trace_colo_mode?: string;
  trace_url?: string;
}
