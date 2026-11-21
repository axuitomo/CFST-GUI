// Android 前台任务/保活/通知权限相关的状态快照。
import type { RuntimeDiagnostics } from "./core";
import type { TaskSnapshot } from "./task";

export interface AndroidBatteryStatus {
  brand: string;
  ignoring_optimizations: boolean;
  manufacturer: string;
  model: string;
  needs_guidance: boolean;
  settings_hint: string;
  supported: boolean;
}

export interface AndroidNotificationPermissionStatus {
  can_request: boolean;
  granted: boolean;
  message: string;
  open_settings_recommended: boolean;
  request_already_attempted: boolean;
  should_show_rationale: boolean;
  state: string;
  supported: boolean;
}

export interface AndroidKeepAliveStatus {
  enabled: boolean;
  message: string;
  notification_permission_granted: boolean;
  running: boolean;
  supported: boolean;
}

export interface AndroidRuntimeStatus {
  battery?: AndroidBatteryStatus | null;
  foreground_service_running: boolean;
  has_task_snapshot: boolean;
  keep_alive?: AndroidKeepAliveStatus | null;
  resume_capable: boolean;
  runtime?: RuntimeDiagnostics | null;
  runtime_attached: boolean;
  session_state: string;
  task_id: string;
  task_snapshot?: TaskSnapshot | null;
}
