# Agent Log Observation Rules

Read this document before diagnosing a probe run, adding a debug event, or changing what the debug log records. It describes the log sinks, the event dictionary, the observation order, and the volume limits that keep the log useful.

## Log Sinks

| Sink | Format | Gate | Rotation | Purpose |
| --- | --- | --- | --- | --- |
| `cfip-log.txt` | JSONL by default, `freeform` optional | only written when `debug` is on (`SetEnabled(cfg.Debug)`) | 10 MB × 5 files, rotates on start | Whole-run narrative: `probe.*`, `stage.*` |
| `error-log.txt` | always JSONL, `level=error` | always appended | 10 MB × 10 files, no rotate on start | Failures only |
| `bridge-debug.log` | JSONL, `level=trace` | mobile/frontend call `bridge.trace` | 4 MB × 3 files, payload truncated at 2000 bytes | Cross-layer calls from Kotlin/frontend |
| diagnostics zip | zip of the above plus state | manual export | — | One-shot evidence bundle |

Three facts decide what you can see at all:

- **Debug off means an empty `cfip-log.txt`.** Check the `debug` flag before concluding "no logs".
- **`simple` verbosity writes only five events**: `probe.start`, `stage.complete`, `probe.export`, `probe.complete`, `probe.failed`. Every per-IP and detail event is dropped. Diagnose with `detailed`.
- **`freeform` destroys aggregation.** Any field-based query below stops working; use it only for human reading.

The diagnostics zip carries `logs/cfip-log.txt`, `logs/error-log.txt`, `status/scheduler.json`, `status/runtime.json`, `config/config-summary.json` and the last 20 terminal task snapshots, all passed through `RedactSensitiveText` with credential fields blanked. Ask for it before asking for screenshots.

## Event Dictionary

```
task      probe.start → probe.progress → stage.start → stage.complete → probe.export → probe.complete
failure   probe.failed                        (also written to error-log.txt)
rejection stage.reject                        (reason carries the cause)
detail    stage.detail / stage.fallback / stage.cooldown
peripheral debug.message, *_task_failure_notification_failed, mobile.*_persist_failed
```

Stage ladder, in execution order:

```
stage0_pool, stage0_mcis
  → stage1_tcp, stage1_ms, stage1_ping
    → stage2_trace, stage2_ms, stage2_limit
      → stage3_get, stage3_download, stage3_ms, stage3_limit
```

`stage.detail` is a single event name with a `reason` discriminator:

| `reason` | Meaning |
| --- | --- |
| `stage_source` | One input source: entries, invalid count, elapsed, failure text, status text |
| `stage_progress` | Stage progress, about ten records per stage |
| `stage_distribution` | `min`/`p50`/`p90`/`p99`/`max` of the survivors of a stage |
| `stage_sample` | Up to 10 surviving IPs with their metrics |
| `stage_limit_dropped` | How many IPs a sort/limit step dropped, and which limit did it |
| `trace_soft_pass` | Trace hit a transient error and the IP was kept anyway |

`stage.reject` reasons are a closed set: `tcp_delay_above_limit`, `tcp_delay_below_min`, `tcp_loss_above_limit`, `tcp_no_response`, `httping_error`, `trace_error`, `trace_read_error`, `trace_latency_limit`, `trace_transport_all_failed`, `trace_worker_panic`, `status_mismatch`, `edge_ip_restricted`, `forbidden`, `download_reconnect`, `request_create_failed`, `rate_limited`, `colo_filter`, `source_colo_filter`, `colo_dictionary_unavailable`, `consecutive_failures`, `pause_requested`.

`edge_ip_restricted`（Cloudflare 403 `error code: 1034`）和 `forbidden`（普通 403）额外带 `note`（`1034` / `403`）与 `ignored`（是否按配置跳过）。两个开关都不保留 IP：`ignored=false` 表示这颗 IP 按普通失败处理并计入连续失败冷却。

同一次淘汰还会走一条**不受 debug 开关约束**的界面事件流记录：`probe.edge_rejected`（payload `stage`、`ip`、`note`、`reason`、`ignored`、`status_code`、`message`）。它不进 `cfip-log.txt`，只在任务进程面板和 WebUI SSE 里出现，所以不打开调试日志也能看到被边缘拒绝的 IP。排查时两者互补：`probe.edge_rejected` 回答“哪些 IP 被 403/1034 淘汰”，`stage.reject` 回答“这次淘汰的完整请求上下文”。

`reason` is finite, `message` is free text. Aggregate on `reason`; read `message` last.

## Observation Order

1. **Funnel shape.** Join `stage.complete` counts per stage. A cliff at one stage localizes the problem; nothing else needs reading yet.
2. **Rejection mix.** Group `stage.reject` by `stage` + `reason`. Mostly `tcp_no_response`/`rate_limited` points at the network path; mostly `trace_*` at the CDN edge; `colo_filter`/`colo_dictionary_unavailable` at filter config or a missing dictionary.
3. **Truncation loss.** Read `stage_limit_dropped`. A large `dropped` with a healthy funnel means the run found IPs and then threw them away, which looks identical to "few results" in the UI.
4. **Distribution.** Read `stage_distribution` for the stage before the cliff. Survivors clustered just past the threshold means the threshold is the problem, not the network.
5. **Single task timeline.** Filter one `task_id`, sort by `ts`, compare `stage.start` → `stage.complete` `duration_ms` to find where time went.
6. **Single IP trail.** Filter one `ip` to see exactly which stage and reason removed it. Compare its metric to the limit in the same event to judge near-miss versus genuine failure.

## Symptom to First Field

| Symptom | Read first | Judge by |
| --- | --- | --- |
| Task fails immediately | `probe.failed` in `error-log.txt` | `stage` field, and `completed_stages` |
| Too few results | `stage.complete` counts, then `stage_limit_dropped` | Whether the loss is rejection or truncation |
| Results look low quality | `stage_distribution` for stage2/stage3 | Survivors sitting at the threshold |
| Speed settings appear ignored | `stage.reject` `tcp.*_ms` vs configured limits | `_above_limit`/`_below_min` direction |
| Only some IPs pass COLO filter | `reason=colo_filter`, `colo_dictionary_unavailable` | Missing dictionary first, filter second |
| One input source is bad | `stage_source` | `error`, `status_text`, `counts.entries` |
| Task appears stuck | Last `stage_progress`, plus `pause_requested`/`consecutive_failures` | Pause, backoff, or a real hang |
| Failure notification missing | `*_task_failure_notification_failed` | Independent of probe success |
| Mobile/frontend disagrees with backend | `bridge-debug.log` | Same `task_id`, aligned by `ts` |

## Query Recipes

```bash
jq -r .event cfip-log.txt | sort | uniq -c | sort -rn                                  # event census
jq -r 'select(.event=="stage.reject") | "\(.stage)\t\(.reason)"' cfip-log.txt \
  | sort | uniq -c | sort -rn                                                          # rejection mix
jq -c 'select(.event=="stage.complete") | {ts,stage,counts,duration_ms}' cfip-log.txt   # funnel
jq -c 'select(.reason=="stage_limit_dropped") | {ts,stage,limit_kind,counts}' cfip-log.txt
jq -c 'select(.reason=="stage_distribution") | {stage,unit,metrics}' cfip-log.txt
jq -c 'select(.reason=="stage_source") | {source_name,counts,duration_ms,error}' cfip-log.txt
jq -c 'select(.task_id=="T-123") | {ts,event,stage,reason,duration_ms}' cfip-log.txt    # timeline
jq -c 'select(.ip=="1.1.1.1") | {ts,event,stage,reason}' cfip-log.txt                   # one IP
jq -c 'select(.event=="probe.failed")' cfip-log.txt error-log.txt                       # failure scene
```

PowerShell without `jq`:

```powershell
Get-Content .\logs\cfip-log.txt | ConvertFrom-Json |
  Where-Object event -eq 'stage.reject' | Group-Object stage,reason |
  Sort-Object Count -Descending | Select-Object Count,Name
```

## Volume Limits

These limits are the reason a 3000-IP run stays readable. Respect them when adding events.

| Limit | Value | Where |
| --- | --- | --- |
| Per-IP sample per stage | 10 | `stageSampleLimit` in `internal/probecore/stage_workflow.go` |
| Progress records per stage | about 10 (decile gate) | `shouldLogProgress` in `internal/appcore/probe_service.go` |
| Truncation events per task | at most 3 | Emitted only when `input > kept` |
| Buffer and flush | 64 KB buffer, 2 s flush | `defaultLogRotationConfig` |
| Bridge payload | 2000 bytes per entry | `bridgeTracePayloadLimit` |

Adding a per-IP event in a hot loop is a regression: it either floods the file or pushes earlier archives out through rotation. Prefer one bounded aggregate event, or sample.

## Adding an Event

- Reuse an existing event name plus a new `reason` before inventing a name; the `stage.detail` family exists for this.
- Always include `stage`, `reason`, and a `counts` map so the event joins the funnel vocabulary.
- Never pass a map you later mutate to `PublishProbeEvent`; copy it first, because the event stream keeps the reference.
- Do not add credential-bearing fields. `RedactSensitiveText` covers `api_token`/`authorization`/`cookie`/`password`/`secret`/`token` keys, bearer patterns, Telegram bot tokens and sensitive URL query parameters, but the safest field is the one never written.
- New events are suppressed under `simple` verbosity by construction (`shouldWriteDebugEvent` is an allowlist). Keep it that way.
- Test the path end to end: a callback test alone does not prove the line reaches `cfip-log.txt`. See `TestStageObservationsReachDebugLogFile`.

## Known Blind Spots

- **Port groups repeat stage events.** With several port groups, `stage1`/`stage2`/`stage3` distribution events repeat per group under one `task_id`; group by `stage` when aggregating.
- **Truncation records counts, not identities.** `stage_limit_dropped` says how many IPs were cut, never which ones. Correlate with `stage_sample` to see who survived.
- **Stage0 per-IP rejection is not logged.** Invalid and duplicate entries are only counted (`counts.invalid`, `counts.filtered`) in `stage.complete`.
- **`error-log.txt` appends synchronously** (open, write, close, no buffer, no process-wide lock). Frequent `*_persist_failed` entries on mobile can partly be that path's own IO cost.
- **Encoding failures surface as `debug.encode_failed`.** Seeing only that line means a field was not serializable, not that the probe failed.

## Baseline Discipline

- Each run is a natural sample: `RotateOnStart` makes the archive timestamp the run boundary, so compare archives rather than slicing one file.
- Record `passed/total` per stage for three runs on a fixed source and fixed settings. Compare against that baseline before reading logs line by line; without one, observation degrades into guessing.
- Keep `detailed` scoped to diagnosis. The buffer keeps write pressure low, but long-running `detailed` mode grows the file and widens the redaction surface.
