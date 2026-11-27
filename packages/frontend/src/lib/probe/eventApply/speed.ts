// 测速与边缘拒绝事件处理器（stage3 采样、403/1034 观察）。
import { asBoolean, asCount, asNumber, asString } from "../../app/valueCoerce";
import { downloadSpeedRowTtlMs, upsertDownloadSpeedRow } from "../../results/downloadSpeedRows";
import type { ProbeEventApplyDeps, ProbeEventContext } from "./types";

export function handleSpeed(ctx: ProbeEventContext, deps: ProbeEventApplyDeps): boolean {
  const { task, taskSessionState, finishTaskAction, downloadSpeedState, downloadSpeedRows, activeDownloadConcurrency, activeDownloadSampleIntervalMs, resetDownloadSpeedState, ensureDownloadSpeedRowPruneTimer, updateDownloadSpeedTrace } = deps;
  const { event, refreshProgressUI } = ctx;
  finishTaskAction("start");
  finishTaskAction("rerun");
  finishTaskAction("resume");
  taskSessionState.value = "active_runtime";
  task.stage = asString(event.payload.stage) || "stage3_get";
  const ip = asString(event.payload.ip).trim() || "当前 IP";
  const currentSpeed = asNumber(event.payload.current_speed_mb_s, 0);
  const averageSpeed = asNumber(event.payload.average_speed_mb_s, 0);
  const currentReadyValue = event.payload.current_ready ?? event.payload.currentReady;
  const averageReadyValue = event.payload.average_ready ?? event.payload.averageReady;
  const currentReady = currentReadyValue === undefined ? true : asBoolean(currentReadyValue, false);
  const averageReady = averageReadyValue === undefined ? true : asBoolean(averageReadyValue, false);
  const bytesRead = asCount(event.payload.bytes_read, 0);
  const measuredBytes = asCount(event.payload.measured_bytes ?? event.payload.measuredBytes, 0);
  const bodyReadValue = event.payload.body_read ?? event.payload.bodyRead;
  const transferCompleteValue = event.payload.transfer_complete ?? event.payload.transferComplete;
  const bodyRead = bodyReadValue === undefined ? bytesRead > 0 : asBoolean(bodyReadValue, false);
  const transferComplete = transferCompleteValue === undefined ? false : asBoolean(transferCompleteValue, false);
  const elapsedMs = asCount(event.payload.elapsed_ms, 0);
  const colo = asString(event.payload.colo).trim();
  const hasReadyFlag = currentReadyValue !== undefined || averageReadyValue !== undefined;
  const isInitialEmptySample = elapsedMs === 0 && bytesRead === 0 && ((hasReadyFlag && !currentReady && !averageReady) || (!hasReadyFlag && currentSpeed === 0 && averageSpeed === 0 && !downloadSpeedState.ip));
  if (isInitialEmptySample) {
    return false;
  }
  // 单并发时保留原来“换 IP 就重置面板”的行为；多并发时所有 IP 共用一张表，不能互相清空。
  if (activeDownloadConcurrency.value <= 1 && downloadSpeedState.ip && downloadSpeedState.ip !== ip) {
    resetDownloadSpeedState();
  }
  // Android 的 200ms 进度节流只限制状态文案与进程日志；每 IP 的实时行必须逐采样更新。
  // 否则同一秒内并发采样的其它 IP 会全部落在节流窗口里，表里只剩第一颗 IP。
  downloadSpeedState.active = true;
  const averageDisplayReady = averageReady && (averageSpeed !== 0 || bytesRead > 0 || measuredBytes > 0 || bodyRead || transferComplete);
  const currentDisplaySpeed = currentReady ? currentSpeed : null;
  const averageDisplaySpeed = averageDisplayReady ? averageSpeed : null;
  downloadSpeedState.averageSpeedMbS = averageDisplaySpeed;
  downloadSpeedState.bytesRead = bytesRead;
  downloadSpeedState.colo = colo;
  downloadSpeedState.currentSpeedMbS = currentDisplaySpeed;
  downloadSpeedState.elapsedMs = elapsedMs;
  downloadSpeedState.ip = ip;
  const rowNow = Date.now();
  upsertDownloadSpeedRow(
    downloadSpeedRows,
    {
      averageSpeedMbS: averageDisplaySpeed,
      bytesRead,
      colo,
      currentSpeedMbS: currentDisplaySpeed,
      elapsedMs,
      ip,
      updatedAt: rowNow,
    },
    { now: rowNow, ttlMs: downloadSpeedRowTtlMs(activeDownloadSampleIntervalMs.value) },
  );
  ensureDownloadSpeedRowPruneTimer();
  if (refreshProgressUI) {
    updateDownloadSpeedTrace(ip, colo, event.ts);
  }
  return true;
}

export function handleEdgeRejected(ctx: ProbeEventContext, deps: ProbeEventApplyDeps) {
  const { pushProcessTrace, edgeRejectedItems } = deps;
  const { event } = ctx;
  // 403 / 1034 淘汰不受 debug 开关约束：进程面板要能直接看到被边缘拒绝的 IP。
  // 后端按 250ms 窗口把同一批拒绝合并进 payload.items，早先的单条形态照样能读。
  for (const item of edgeRejectedItems(event)) {
    const note = asString(item.note).trim() || "403";
    const rejectedIp = asString(item.ip).trim();
    pushProcessTrace({
      detail: asString(item.message).trim() || `${rejectedIp || "当前 IP"} 被边缘拒绝（${note}）。`,
      stage: asString(item.stage).trim() || "stage3_get",
      title: note === "1034" ? "边缘 IP 受限 1034" : "HTTP 403 拒绝",
      tone: "warning",
      ts: event.ts,
    });
  }
}
