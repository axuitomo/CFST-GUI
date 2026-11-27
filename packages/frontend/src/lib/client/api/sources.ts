import { getPlatformAdapter } from "../../adapters/registry";
import { normalizeSourceProfileUpdatePayload } from "../../bridge/config";
import type { CommandResult, SourcePreviewPayload, SourceProfileStore, SourceProfileUpdatePayload } from "../../bridge/types";
import { invokeCore } from "../invoke";

export async function consumeDroppedSources() {
  return getPlatformAdapter().consumeDroppedSources();
}

export async function loadSourceProfiles() {
  return invokeCore<SourceProfileStore>("source_profiles.load");
}

export async function saveSourceProfile(payload: Record<string, unknown>) {
  return invokeCore<SourceProfileStore>("source_profiles.save", payload);
}

export async function updateCurrentSourceProfile(payload: Record<string, unknown>) {
  const result = await invokeCore("source_profiles.update_current", payload);
  return {
    ...result,
    data: result.data ? normalizeSourceProfileUpdatePayload(result.data) : null,
  } as CommandResult<SourceProfileUpdatePayload | null>;
}

export async function saveSourceProfileStore(payload: Record<string, unknown>) {
  return invokeCore<SourceProfileStore>("source_profiles.save_store", payload);
}

export async function switchSourceProfile(payload: Record<string, unknown>) {
  return invokeCore("source_profiles.switch", payload);
}

export async function deleteSourceProfile(payload: Record<string, unknown>) {
  return invokeCore<SourceProfileStore>("source_profiles.delete", payload);
}

export async function previewSource(payload: Record<string, unknown>) {
  return invokeCore<SourcePreviewPayload>("source.preview", payload);
}
