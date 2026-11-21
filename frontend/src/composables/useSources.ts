// 输入源域（重构计划 §4.2）：sources/分组状态、预览、签名与保存基线。
// 从 App.vue 原样迁出；输入组档案签名经 deps 注入（useSourceProfiles 后接线）。
import { computed, reactive, ref, watch } from "vue";
import {
  consumeDroppedSources,
  previewSource,
  type ColoFilterMode,
  type SourceConfig,
  type SourceIPMode,
  type SourceKind,
  type SourcePreviewPayload,
} from "../lib/bridge";
import { sourceInputIssue, sourcePreviewKey, type SourceGroupIPMode } from "../lib/sourceGroups";
import { detectSourceNameFromUrl, isDefaultSourceName } from "../lib/sourceNames";
import { asCount, asRecord, asString } from "../lib/valueCoerce";
import { buildConfigSnapshot as buildConfigSnapshotFrom, snapshotSignature } from "../lib/configSnapshotBuild";
import type { ToastTone } from "../lib/appTypes";

export interface SourceDraft extends SourceConfig {}

export interface SourcePreviewState {
  action: string;
  entries: string[];
  invalidCount: number;
  portSummary: Record<string, unknown> | null;
  totalCount: number;
  updatedAt: string;
  warnings: string[];
}

export interface SourcesDeps {
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  autoDetectSourceName: () => boolean;
  buildConfigSnapshot: () => ReturnType<typeof buildConfigSnapshotFrom>;
  commandDiagnosticPayload: (result: { code?: string; data?: unknown; message?: string; warnings?: string[] }) => unknown;
  currentSourceProfileSignature: () => string;
  activeSourceProfileSourceSignature: () => string;
}

export function useSources(deps: SourcesDeps) {
  const { appendLog, showToast, buildConfigSnapshot, commandDiagnosticPayload, currentSourceProfileSignature, activeSourceProfileSourceSignature } = deps;
  const sourceSeed = ref(0);
  const sourcePreviewStates = reactive<Record<string, SourcePreviewState>>({});
  const sourcePreviewErrors = reactive<Record<string, string>>({});
  const sourceRequestStates = reactive<Record<string, string>>({});
  const sourceGroupIPMode = ref<SourceGroupIPMode>("traverse");
  const sourceGroupColoFilter = ref("");
  const sourceGroupColoFilterMode = ref<ColoFilterMode | "">("allow");
  const sources = ref<SourceDraft[]>([]);
  let lastSavedSourceProfileSignature = "";
  let lastSavedSourceSignature = "";

  const sourcePayloads = computed(() =>
    sources.value.map((source, index) => ({
      colo_filter: sourceGroupColoFilterMode.value ? sourceGroupColoFilter.value.trim() : source.colo_filter.trim(),
      colo_filter_mode: sourceGroupColoFilterMode.value || source.colo_filter_mode,
      content: source.content.trim(),
      enabled: source.enabled,
      id: source.id,
      ip_limit: -1,
      ip_mode: sourceGroupIPMode.value || source.ip_mode,
      kind: source.kind,
      last_fetched_at: source.last_fetched_at,
      last_fetched_count: source.last_fetched_count,
      name: sourceNameForPayload(source, index),
      path: source.path.trim(),
      status_text: source.status_text,
      url: source.url.trim(),
    })),
  );
  const preparedSources = computed(() => sourcePayloads.value.filter((source) => source.enabled && hasUsableSourceInput(source)));
  const noSources = computed(() => preparedSources.value.length === 0);

  function createSourceDraft(kind: SourceKind): SourceDraft {
    sourceSeed.value += 1;
    return {
      content: "",
      colo_filter: "",
      colo_filter_mode: "allow",
      enabled: true,
      id: `source-${Date.now()}-${sourceSeed.value}`,
      ip_limit: -1,
      ip_mode: sourceGroupIPMode.value || "traverse",
      kind,
      last_fetched_at: "",
      last_fetched_count: 0,
      name: `输入源 ${sourceSeed.value}`,
      path: "",
      status_text: "",
      url: "",
    };
  }

  function applyDroppedSourcePaths(value: unknown) {
    const paths = Array.isArray(value) ? value.map((item) => asString(item)).filter((item) => item !== "") : [];
    let added = 0;
    for (const path of paths) {
      if (sources.value.some((source) => source.kind === "file" && source.path === path)) {
        continue;
      }
      const draft = createSourceDraft("file");
      draft.path = path;
      const fileName = path.split(/[\\/]/).pop() || "";
      if (fileName) {
        draft.name = fileName;
      }
      sources.value.push(draft);
      added += 1;
    }
    if (added > 0) {
      showToast(`已把 ${added} 个拖放文件加入输入源，保存配置后生效。`, "success");
    }
  }

  async function refreshDroppedSources() {
    try {
      const result = await consumeDroppedSources();
      appendLog("bridge.consume_dropped_sources", commandDiagnosticPayload(result));
      if (!result.ok) {
        return;
      }
      applyDroppedSourcePaths(asRecord(result.data).paths);
    } catch (error) {
      appendLog("bridge.consume_dropped_sources_failed", {
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  function detectedSourceName(source: Pick<SourceDraft, "kind" | "url">) {
    if (source.kind !== "url") {
      return "";
    }
    return detectSourceNameFromUrl(source.url);
  }

  function shouldAutoFillSourceName(source: Pick<SourceDraft, "kind" | "name" | "url">) {
    return deps.autoDetectSourceName() && source.kind === "url" && isDefaultSourceName(source.name) && Boolean(source.url.trim());
  }

  function sourceNameForPayload(source: SourceDraft, index: number) {
    if (shouldAutoFillSourceName(source)) {
      const detected = detectedSourceName(source);
      if (detected) {
        return detected;
      }
    }
    return source.name.trim() || `输入源 ${index + 1}`;
  }

  function applyDetectedSourceName(sourceId: string) {
    const source = sources.value.find((entry) => entry.id === sourceId);
    if (!source || !shouldAutoFillSourceName(source)) {
      return;
    }
    const detected = detectedSourceName(source);
    if (detected) {
      source.name = detected;
    }
  }

  function hasUsableSourceInput(source: Pick<SourceDraft, "content" | "kind" | "path" | "url">) {
    if (source.kind === "inline") {
      return Boolean(source.content.trim());
    }

    if (source.kind === "file") {
      return Boolean(source.path.trim());
    }

    return Boolean(source.url.trim());
  }

  function addSource(kind: SourceKind) {
    sources.value.push(createSourceDraft(kind));
  }

  function setSourceGroupIPMode(mode: SourceIPMode) {
    sourceGroupIPMode.value = mode;
    for (const source of sources.value) source.ip_mode = mode;
  }

  function setSourceGroupColoFilter(filter: string) {
    sourceGroupColoFilter.value = filter;
  }

  function setSourceGroupColoFilterMode(mode: ColoFilterMode) {
    sourceGroupColoFilterMode.value = mode;
  }

  function removeSource(sourceId: string) {
    const source = sources.value.find((item) => item.id === sourceId);
    if (source && !window.confirm(`确定删除“${source.name || "未命名输入源"}”吗？删除后需要重新添加。`)) {
      return;
    }
    sources.value = sources.value.filter((source) => source.id !== sourceId);
    delete sourcePreviewStates[sourceId];
    delete sourceRequestStates[sourceId];
    delete sourcePreviewErrors[sourceId];
  }

  function applySourceStatuses(statusesValue: unknown) {
    if (!Array.isArray(statusesValue) || statusesValue.length === 0) {
      return;
    }

    const nextStatusMap = new Map(
      statusesValue
        .map((entry) => {
          const source = asRecord(entry);
          const sourceId = asString(source.id).trim();

          if (!sourceId) {
            return null;
          }

          return [
            sourceId,
            {
              last_fetched_at: asString(source.last_fetched_at || source.lastFetchedAt).trim(),
              last_fetched_count: Math.max(0, asCount(source.last_fetched_count || source.lastFetchedCount, 0)),
              status_text: asString(source.status_text || source.statusText).trim(),
            },
          ] as const;
        })
        .filter((entry): entry is readonly [string, { last_fetched_at: string; last_fetched_count: number; status_text: string }] => Boolean(entry)),
    );

    if (nextStatusMap.size === 0) {
      return;
    }

    sources.value = sources.value.map((source) => {
      const statusEntry = nextStatusMap.get(source.id);
      if (!statusEntry) {
        return source;
      }

      return {
        ...source,
        last_fetched_at: statusEntry.last_fetched_at,
        last_fetched_count: statusEntry.last_fetched_count,
        status_text: statusEntry.status_text,
      };
    });
  }

  function applySourcePreview(sourceId: string, dataValue: unknown, warnings: string[]) {
    const data = asRecord(dataValue);
    const summaryRecord = asRecord(data.summary);
    const entries = Array.isArray(data.preview_entries) ? data.preview_entries.map((entry) => asString(entry).trim()).filter(Boolean) : [];

    sourcePreviewStates[sourceId] = {
      action: asString(summaryRecord.action || "预览"),
      entries,
      invalidCount: asCount(summaryRecord.invalid_count, 0),
      portSummary: Object.keys(asRecord(data.port_summary)).length > 0 ? asRecord(data.port_summary) : null,
      totalCount: asCount(summaryRecord.total_count, entries.length),
      updatedAt: new Date().toISOString(),
      warnings,
    };
  }

  async function inspectSource(sourceId: string) {
    if (sourceRequestStates[sourceId]) return;
    applyDetectedSourceName(sourceId);
    const sourceIndex = sources.value.findIndex((entry) => entry.id === sourceId);
    const source = sources.value[sourceIndex];
    if (!source) return;
    const issue = sourceInputIssue(source);
    if (issue) {
      sourcePreviewErrors[sourceId] = issue;
      return;
    }
    const inputKey = sourcePreviewKey(source, sourceGroupColoFilter.value, sourceGroupColoFilterMode.value);
    const isCurrent = () => {
      const current = sources.value.find((entry) => entry.id === sourceId);
      return current && sourcePreviewKey(current, sourceGroupColoFilter.value, sourceGroupColoFilterMode.value) === inputKey;
    };
    sourceRequestStates[sourceId] = "preview";
    delete sourcePreviewErrors[sourceId];
    try {
      const result = await previewSource({
        config: buildConfigSnapshot(),
        preview_limit: 16,
        source: { ...sourcePayloads.value[sourceIndex], content: source.content.trim(), name: sourceNameForPayload(source, sourceIndex), path: source.path.trim(), url: source.url.trim() },
      });
      appendLog("bridge.source_preview", result);
      if (!isCurrent()) return;
      if (!result.ok) {
        sourcePreviewErrors[sourceId] = result.message || "读取来源失败。";
        return;
      }
      applySourcePreview(sourceId, asRecord(result.data as SourcePreviewPayload | null), result.warnings || []);
    } catch (error) {
      if (isCurrent()) sourcePreviewErrors[sourceId] = error instanceof Error ? error.message : "读取来源失败。";
    } finally {
      delete sourceRequestStates[sourceId];
    }
  }

  function currentSourceSignature() {
    return snapshotSignature({ ip_mode: sourceGroupIPMode.value, colo_filter: sourceGroupColoFilter.value, colo_filter_mode: sourceGroupColoFilterMode.value, sources: sourcePayloads.value.map((source) => ({ ...source, status_text: source.status_text.trim() })) });
  }

  function markSourceSaveBaselines() {
    lastSavedSourceSignature = currentSourceSignature();
    lastSavedSourceProfileSignature = currentSourceProfileSignature();
  }

  function sourcePageHasUnsavedChanges() {
    return currentSourceSignature() !== lastSavedSourceSignature || currentSourceProfileSignature() !== lastSavedSourceProfileSignature || currentSourceSignature() !== activeSourceProfileSourceSignature();
  }

  watch(
    () => sources.value.map((source) => [source.id, sourcePreviewKey(source, sourceGroupColoFilter.value, sourceGroupColoFilterMode.value)] as const),
    (current, previous) => {
      const previousKeys = new Map(previous);
      const currentKeys = new Map(current);
      for (const [id, key] of previousKeys) {
        if (currentKeys.get(id) !== key) {
          delete sourcePreviewStates[id];
          delete sourcePreviewErrors[id];
        }
      }
    },
  );

  return {
    sources,
    sourceGroupIPMode,
    sourceGroupColoFilter,
    sourceGroupColoFilterMode,
    sourcePreviewStates,
    sourcePreviewErrors,
    sourceRequestStates,
    sourcePayloads,
    preparedSources,
    noSources,
    addSource,
    setSourceGroupIPMode,
    setSourceGroupColoFilter,
    setSourceGroupColoFilterMode,
    removeSource,
    applyDroppedSourcePaths,
    refreshDroppedSources,
    applyDetectedSourceName,
    applySourceStatuses,
    applySourcePreview,
    inspectSource,
    currentSourceSignature,
    markSourceSaveBaselines,
    sourcePageHasUnsavedChanges,
    lastSavedSourceSignature: () => lastSavedSourceSignature,
    lastSavedSourceProfileSignature: () => lastSavedSourceProfileSignature,
    setLastSavedSourceProfileSignature: (signature: string) => {
      lastSavedSourceProfileSignature = signature;
    },
  };
}