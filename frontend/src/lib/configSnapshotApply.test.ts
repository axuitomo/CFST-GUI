// configSnapshotApply 特征测试（重构计划 §4.2：persist 链先锁输入→输出）。
import { describe, expect, it, vi } from "vitest";
import { reactive, ref } from "vue";
import { normalizeConfigSnapshot } from "./bridge";
import { applyConfigSnapshotFrom, type ConfigSnapshotApplyDeps } from "./configSnapshotApply";
import { createDefaultSettingsForm, DEFAULT_FILE_TEST_URL, DEFAULT_PROBE_USER_AGENT } from "./settingsForm";
import type { SourceDraft } from "../composables/useSources";
import type { SourceGroupIPMode } from "./sourceGroups";

function makeDeps(overrides: Partial<ConfigSnapshotApplyDeps> = {}) {
  const settings = reactive(createDefaultSettingsForm());
  const sourceGroupIPMode = ref<SourceGroupIPMode>("traverse");
  const sources = ref<SourceDraft[]>([]);
  const applyDNSProvidersFromSnapshot = vi.fn();
  const captureDownloadSpeedTaskConfig = vi.fn();
  const deps: ConfigSnapshotApplyDeps = {
    settings,
    sourceGroupIPMode,
    sources,
    applyDNSProvidersFromSnapshot,
    isTaskActive: () => false,
    isDownloadSpeedTaskConfigCaptured: () => false,
    captureDownloadSpeedTaskConfig,
    ...overrides,
  };
  return { deps, settings, sourceGroupIPMode, sources, applyDNSProvidersFromSnapshot, captureDownloadSpeedTaskConfig };
}

describe("hydration defaults", () => {
  it("empty snapshot restores canonical defaults", () => {
    const { deps, settings } = makeDeps();
    applyConfigSnapshotFrom(normalizeConfigSnapshot({}), deps);
    expect(settings.probeURL).toBe(DEFAULT_FILE_TEST_URL);
    expect(settings.probeUserAgent).toBe(DEFAULT_PROBE_USER_AGENT);
    expect(settings.themeMode).toBe("auto_system_time");
    expect(settings.webdavRemotePath).toBe("cfst-gui-config.zip");
    expect(settings.githubBranch).toBe("main");
    expect(settings.exportOverwrite).toBe("replace_on_start");
    expect(settings.probeStrategy).toBe("fast");
    expect(settings.probeDisableDownload).toBe(true);
  });

  it("scheduler draft follows committed interval or falls back to 60", () => {
    const { deps, settings } = makeDeps();
    applyConfigSnapshotFrom(normalizeConfigSnapshot({ scheduler: { interval_minutes: 30 } }), deps);
    expect(settings.schedulerIntervalMinutes).toBe(30);
    expect(settings.schedulerIntervalMinutesDraft).toBe(30);
    settings.schedulerIntervalMinutesDraft = -1;
    applyConfigSnapshotFrom(normalizeConfigSnapshot({ scheduler: { interval_minutes: 0 } }), deps);
    expect(settings.schedulerIntervalMinutesDraft).toBe(60);
  });

  it("replaces sources and group ip mode, invokes dns hydration", () => {
    const { deps, sources, sourceGroupIPMode, applyDNSProvidersFromSnapshot } = makeDeps();
    const normalized = normalizeConfigSnapshot({
      source_ip_mode: "mcis",
      sources: [{ id: "s1", kind: "url", url: "https://a.example", enabled: true }],
    });
    applyConfigSnapshotFrom(normalized, deps);
    expect(sourceGroupIPMode.value).toBe("mcis");
    expect(sources.value).toHaveLength(1);
    expect(sources.value[0].id).toBe("s1");
    expect(applyDNSProvidersFromSnapshot).toHaveBeenCalledWith(normalized);
  });
});

describe("download speed capture gating", () => {
  it("captures when idle or not yet captured", () => {
    const { deps, captureDownloadSpeedTaskConfig } = makeDeps();
    applyConfigSnapshotFrom(normalizeConfigSnapshot({}), deps);
    expect(captureDownloadSpeedTaskConfig).toHaveBeenCalledTimes(1);
  });

  it("skips capture while task active and already captured", () => {
    const { deps, captureDownloadSpeedTaskConfig } = makeDeps({
      isTaskActive: () => true,
      isDownloadSpeedTaskConfigCaptured: () => true,
    });
    applyConfigSnapshotFrom(normalizeConfigSnapshot({}), deps);
    expect(captureDownloadSpeedTaskConfig).not.toHaveBeenCalled();
  });

  it("recaptures when task active but never captured", () => {
    const { deps, captureDownloadSpeedTaskConfig } = makeDeps({
      isTaskActive: () => true,
      isDownloadSpeedTaskConfigCaptured: () => false,
    });
    applyConfigSnapshotFrom(normalizeConfigSnapshot({}), deps);
    expect(captureDownloadSpeedTaskConfig).toHaveBeenCalledTimes(1);
  });
});
