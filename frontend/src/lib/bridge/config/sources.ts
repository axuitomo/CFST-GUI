// 输入源与输入组（source profile）的归一化。
import { toBoolean, toInteger, toObjectRecord, toStringValue, toUnknownArray } from "../../bridgeValues";
import { resolveSourceGroupIPMode } from "../../sourceGroups";
import type { ColoFilterMode, SourceConfig, SourceProfileItem, SourceProfileStore } from "../types";
import { normalizeColoFilterMode, normalizeSourceIPMode, normalizeSourceKind } from "./scalars";

export function normalizeSourceConfig(input: unknown, index: number): SourceConfig {
  const source = toObjectRecord(input);

  return {
    colo_filter: toStringValue(source.colo_filter ?? source.coloFilter),
    colo_filter_mode: normalizeColoFilterMode(source.colo_filter_mode ?? source.coloFilterMode),
    content: toStringValue(source.content),
    enabled: toBoolean(source.enabled, true),
    id: toStringValue(source.id) || `source-${index + 1}`,
    // 输入源的 IP 上限已从界面移除，一律按“不限量”下发，实际由 probe.source_pool_limit 收敛。
    ip_limit: -1,
    ip_mode: normalizeSourceIPMode(source.ip_mode ?? source.ipMode),
    kind: normalizeSourceKind(source.kind ?? source.type),
    last_fetched_at: toStringValue(source.last_fetched_at ?? source.lastFetchedAt),
    last_fetched_count: Math.max(0, toInteger(source.last_fetched_count ?? source.lastFetchedCount, 0)),
    name: toStringValue(source.name) || `输入源 ${index + 1}`,
    path: toStringValue(source.path),
    status_text: toStringValue(source.status_text ?? source.statusText),
    url: toStringValue(source.url),
  };
}

function normalizeSourceProfileItem(input: unknown, index: number): SourceProfileItem {
  const source = toObjectRecord(input);
  let sources = toUnknownArray(source.sources).map((entry, sourceIndex) => normalizeSourceConfig(entry, sourceIndex));
  const ipMode = resolveSourceGroupIPMode(source.ip_mode ?? source.ipMode, sources);
  let coloFilter = toStringValue(source.colo_filter ?? source.coloFilter).trim();
  const rawColoMode = toStringValue(source.colo_filter_mode ?? source.coloFilterMode)
    .toLowerCase()
    .trim();
  let coloFilterMode: ColoFilterMode | "" = rawColoMode === "allow" || rawColoMode === "deny" ? rawColoMode : "";

  if (!coloFilterMode) {
    const filters = new Set(sources.map((entry) => entry.colo_filter.trim()));
    const modes = new Set(sources.map((entry) => entry.colo_filter_mode));
    if (filters.size <= 1 && modes.size <= 1) {
      coloFilter = sources[0]?.colo_filter.trim() || "";
      coloFilterMode = sources[0]?.colo_filter_mode || "allow";
    }
  }
  if (coloFilterMode) {
    sources = sources.map((entry) => ({ ...entry, colo_filter: coloFilter, colo_filter_mode: coloFilterMode as ColoFilterMode }));
  }

  return {
    colo_filter: coloFilter,
    colo_filter_mode: coloFilterMode,
    created_at: toStringValue(source.created_at ?? source.createdAt),
    id: toStringValue(source.id) || `source-profile-${index + 1}`,
    name: toStringValue(source.name) || `输入组 ${index + 1}`,
    ip_mode: ipMode,
    sources: sources.map((entry) => (ipMode ? { ...entry, ip_mode: ipMode } : entry)),
    updated_at: toStringValue(source.updated_at ?? source.updatedAt),
  };
}

export function normalizeSourceProfileStore(input: unknown): SourceProfileStore {
  const source = toObjectRecord(input);

  return {
    active_profile_id: toStringValue(source.active_profile_id ?? source.activeProfileId),
    items: toUnknownArray(source.items).map((entry, index) => normalizeSourceProfileItem(entry, index)),
    schema_version: toStringValue(source.schema_version ?? source.schemaVersion),
    updated_at: toStringValue(source.updated_at ?? source.updatedAt),
  };
}
