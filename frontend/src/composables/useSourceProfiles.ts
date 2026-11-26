// 输入组档案域（重构计划 §4.2）：sourceProfiles 状态、档案 CRUD 与档案签名。
// 从 App.vue 原样迁出；sources/分组 ref 与跨域回调经 deps 注入。
import { ref, type Ref } from "vue";
import { deleteSourceProfile, normalizeConfigSnapshot, normalizeSourceProfileStore, saveSourceProfile, switchSourceProfile, updateCurrentSourceProfile, type ColoFilterMode, type SourceConfig, type SourceProfileStore } from "../lib/bridge";
import { resolveSourceGroupIPMode, type SourceGroupIPMode } from "../lib/sourceGroups";
import { asRecord, asString } from "../lib/valueCoerce";
import { snapshotSignature } from "../lib/configSnapshotBuild";
import type { SourceDraft } from "./useSources";
import type { ToastTone } from "../lib/appTypes";

export interface SourceProfilesDeps {
  sources: Ref<SourceDraft[]>;
  sourceGroupIPMode: Ref<SourceGroupIPMode>;
  sourceGroupColoFilter: Ref<string>;
  sourceGroupColoFilterMode: Ref<ColoFilterMode | "">;
  sourcePayloads: () => SourceConfig[];
  currentSourceSignature: () => string;
  sourcePageHasUnsavedChanges: () => boolean;
  autoSaveSourcePage: (reason: string) => Promise<boolean>;
  selectedView: () => string;
  configPath: Ref<string>;
  appendLog: (event: string, payload: unknown) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  setLastSavedSourceProfileSignature: (signature: string) => void;
}

export function useSourceProfiles(deps: SourceProfilesDeps) {
  const { sources, sourceGroupIPMode, sourceGroupColoFilter, sourceGroupColoFilterMode, currentSourceSignature, sourcePageHasUnsavedChanges, autoSaveSourcePage, configPath, appendLog, showToast } = deps;
  const sourceProfiles = ref<SourceProfileStore>({
    active_profile_id: "",
    items: [],
    schema_version: "",
    updated_at: "",
  });

  function sourceDraftsFromProfileSources(value: unknown): SourceDraft[] {
    const nextSources = Array.isArray(value) ? value : [];
    return normalizeConfigSnapshot({ sources: nextSources }).sources.map((source) => ({ ...source }));
  }

  /**
   * keepGroupColoFilter:输入组更新还在写的时候,服务端返回的输入组快照可能落后于
   * 已经写进 config 的组级筛选。此时保留内存里的筛选值,避免用户刚点的筛选模式被旧值弹回去;
   * 后续自动保存会把内存值写回输入组,最终仍然一致。
   */
  function applySourceProfileStore(value: unknown, options: { keepGroupColoFilter?: boolean } = {}) {
    const currentColoFilter = sourceGroupColoFilter.value;
    const currentColoFilterMode = sourceGroupColoFilterMode.value;
    sourceProfiles.value = normalizeSourceProfileStore(value);
    if (options.keepGroupColoFilter) {
      sourceGroupColoFilter.value = currentColoFilter;
      sourceGroupColoFilterMode.value = currentColoFilterMode;
      return;
    }
    const active = sourceProfiles.value.items.find((profile) => profile.id === sourceProfiles.value.active_profile_id);
    sourceGroupColoFilter.value = active?.colo_filter || "";
    sourceGroupColoFilterMode.value = active ? active.colo_filter_mode : "allow";
  }

  function currentSourceProfileSignature() {
    return snapshotSignature(sourceProfiles.value);
  }

  function activeSourceProfileSourceSignature() {
    const active = sourceProfiles.value.items.find((profile) => profile.id === sourceProfiles.value.active_profile_id);
    const snapshot = normalizeConfigSnapshot({ sources: active?.sources || [], source_ip_mode: active?.ip_mode });
    return snapshotSignature({
      ip_mode: snapshot.source_ip_mode,
      colo_filter: active?.colo_filter || "",
      colo_filter_mode: active?.colo_filter_mode || "",
      sources: snapshot.sources.map((source) => ({ ...source, status_text: source.status_text.trim() })),
    });
  }

  async function saveCurrentSourceProfile(name: string, profileId = "", profileSources?: unknown, setActive = true, ipMode?: SourceGroupIPMode, coloFilter = sourceGroupColoFilter.value, coloFilterMode = sourceGroupColoFilterMode.value) {
    try {
      if (setActive && profileId !== sourceProfiles.value.active_profile_id && sourcePageHasUnsavedChanges()) {
        if (!(await autoSaveSourcePage("create_source_profile"))) {
          showToast("当前输入组保存失败，已取消创建或复制", "error");
          return false;
        }
      }
      const payloadMode = resolveSourceGroupIPMode(ipMode ?? sourceGroupIPMode.value, Array.isArray(profileSources) ? profileSources : sources.value);
      const payloadSources = Array.isArray(profileSources)
        ? profileSources.map((entry, index) => ({
            ...entry,
            name: asString(asRecord(entry).name).trim() || `输入源 ${index + 1}`,
          }))
        : deps.sourcePayloads();
      const result = await saveSourceProfile({
        name: name.trim() || "当前输入源",
        profile_id: profileId,
        set_active: setActive,
        ip_mode: payloadMode,
        colo_filter: coloFilter.trim(),
        colo_filter_mode: coloFilterMode,
        sources: payloadSources,
      });
      appendLog("bridge.save_source_profile", result);
      if (!result.ok) {
        showToast(result.message || "保存输入组失败", "error");
        return false;
      }
      applySourceProfileStore(result.data);
      if (setActive) {
        sourceGroupIPMode.value = payloadMode;
        sources.value = sourceDraftsFromProfileSources(payloadSources);
      }
      deps.setLastSavedSourceProfileSignature(currentSourceProfileSignature());
      showToast("输入组已保存", "success");
      return true;
    } catch (error) {
      showToast(error instanceof Error ? error.message : "保存输入组失败", "error");
      return false;
    }
  }

  async function copyCurrentSourceProfile() {
    const active = sourceProfiles.value.items.find((profile) => profile.id === sourceProfiles.value.active_profile_id);
    await saveCurrentSourceProfile(
      `${active?.name || "当前输入源"} 副本`,
      "",
      deps.sourcePayloads().map((source) => ({ ...source })),
      true,
      sourceGroupIPMode.value,
    );
  }

  async function updateActiveSourceProfile() {
    try {
      const active = sourceProfiles.value.items.find((profile) => profile.id === sourceProfiles.value.active_profile_id);
      const submittedSignature = currentSourceSignature();
      const result = await updateCurrentSourceProfile({
        name: active?.name || "当前输入源",
        profile_id: active?.id || "",
        ip_mode: sourceGroupIPMode.value,
        colo_filter: sourceGroupColoFilter.value.trim(),
        colo_filter_mode: sourceGroupColoFilterMode.value,
        sources: deps.sourcePayloads(),
      });
      appendLog("bridge.update_current_source_profile", result);
      if (!result.ok) {
        showToast(result.message || "更新输入组失败", "error");
        return false;
      }
      const changedDuringSave = currentSourceSignature() !== submittedSignature;
      const latestColoFilter = sourceGroupColoFilter.value;
      const latestColoFilterMode = sourceGroupColoFilterMode.value;
      applySourceProfileStore(result.data?.source_profiles);
      if (changedDuringSave) {
        sourceGroupColoFilter.value = latestColoFilter;
        sourceGroupColoFilterMode.value = latestColoFilterMode;
      } else {
        sources.value = sourceDraftsFromProfileSources(result.data?.sources || deps.sourcePayloads());
      }
      deps.setLastSavedSourceProfileSignature(currentSourceProfileSignature());
      showToast("输入组已更新并保存", "success");
      return true;
    } catch (error) {
      showToast(error instanceof Error ? error.message : "更新输入组失败", "error");
      return false;
    }
  }

  async function updateActiveSourceProfileQuietly() {
    try {
      const active = sourceProfiles.value.items.find((profile) => profile.id === sourceProfiles.value.active_profile_id);
      const submittedSignature = currentSourceSignature();
      const result = await updateCurrentSourceProfile({
        name: active?.name || "当前输入源",
        profile_id: active?.id || "",
        ip_mode: sourceGroupIPMode.value,
        colo_filter: sourceGroupColoFilter.value.trim(),
        colo_filter_mode: sourceGroupColoFilterMode.value,
        sources: deps.sourcePayloads(),
      });
      appendLog("bridge.update_current_source_profile", result);
      if (!result.ok) {
        appendLog("source.profile_quiet_save.failed", result.message || "更新输入组失败");
        return false;
      }
      const changedDuringSave = currentSourceSignature() !== submittedSignature;
      const latestColoFilter = sourceGroupColoFilter.value;
      const latestColoFilterMode = sourceGroupColoFilterMode.value;
      applySourceProfileStore(result.data?.source_profiles);
      if (changedDuringSave) {
        sourceGroupColoFilter.value = latestColoFilter;
        sourceGroupColoFilterMode.value = latestColoFilterMode;
      } else {
        sources.value = sourceDraftsFromProfileSources(result.data?.sources || deps.sourcePayloads());
      }
      deps.setLastSavedSourceProfileSignature(currentSourceProfileSignature());
      return true;
    } catch (error) {
      appendLog("source.profile_quiet_save.failed", error instanceof Error ? error.message : String(error));
      return false;
    }
  }

  async function switchToSourceProfile(profileId: string) {
    if (deps.selectedView() === "sources" && sourcePageHasUnsavedChanges()) {
      const saved = await autoSaveSourcePage("switch_source_profile");
      if (!saved) {
        showToast("输入源保存失败，已取消切换", "error");
        return;
      }
    }

    try {
      const result = await switchSourceProfile({ profile_id: profileId });
      appendLog("bridge.switch_source_profile", result);
      const data = asRecord(result.data);
      if (!result.ok) {
        showToast(result.message || "切换输入组失败", "error");
        return;
      }
      applySourceProfileStore(data.source_profiles || data.sourceProfiles);
      const nextSources = Array.isArray(data.sources) ? data.sources : [];
      sourceGroupIPMode.value = resolveSourceGroupIPMode(sourceProfiles.value.items.find((profile) => profile.id === sourceProfiles.value.active_profile_id)?.ip_mode, sourceDraftsFromProfileSources(nextSources));
      sources.value = sourceDraftsFromProfileSources(nextSources);
      deps.setLastSavedSourceProfileSignature(currentSourceProfileSignature());
      if (data.config_snapshot || data.configSnapshot) {
        configPath.value = asString(data.configPath || data.config_path || configPath.value);
      }
      showToast("输入组已切换", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "切换输入组失败", "error");
    }
  }

  async function removeSourceProfile(profileId: string) {
    const deletedActiveProfile = sourceProfiles.value.active_profile_id === profileId;
    if (!window.confirm("删除输入组后无法恢复。若删除当前输入组，当前输入源会切换到新的当前输入组。")) {
      return;
    }
    if (deletedActiveProfile && deps.selectedView() === "sources" && sourcePageHasUnsavedChanges()) {
      const saved = await autoSaveSourcePage("delete_active_source_profile");
      if (!saved) {
        showToast("输入源保存失败，已取消删除", "error");
        return;
      }
    }
    try {
      const result = await deleteSourceProfile({ profile_id: profileId });
      appendLog("bridge.delete_source_profile", result);
      if (!result.ok) {
        showToast(result.message || "删除输入组失败", "error");
        return;
      }
      const data = asRecord(result.data);
      const nextStore = normalizeSourceProfileStore(data.source_profiles || data.sourceProfiles || result.data);
      applySourceProfileStore(nextStore);
      if (deletedActiveProfile) {
        const activeProfile = nextStore.items.find((profile) => profile.id === nextStore.active_profile_id) || null;
        const nextSources = Array.isArray(data.sources) ? data.sources : activeProfile?.sources || [];
        sourceGroupIPMode.value = resolveSourceGroupIPMode(activeProfile?.ip_mode, sourceDraftsFromProfileSources(nextSources));
        sources.value = sourceDraftsFromProfileSources(nextSources);
      }
      deps.setLastSavedSourceProfileSignature(currentSourceProfileSignature());
      showToast("输入组已删除", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "删除输入组失败", "error");
    }
  }

  return {
    sourceProfiles,
    applySourceProfileStore,
    sourceDraftsFromProfileSources,
    currentSourceProfileSignature,
    activeSourceProfileSourceSignature,
    saveCurrentSourceProfile,
    copyCurrentSourceProfile,
    updateActiveSourceProfile,
    updateActiveSourceProfileQuietly,
    switchToSourceProfile,
    removeSourceProfile,
  };
}
