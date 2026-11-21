// configSnapshotBuild 特征测试（重构计划 §4.2：先锁输入→输出再动刀）。
// 锁定默认值映射、github 启用门控、masked token 拒绝、webhook 拆分、
// scheduler interval 的 draft 回退、签名键序无关性。
import { describe, expect, it } from "vitest";
import { buildConfigSnapshot, schedulerIntervalMinutesForSnapshot, snapshotSignature, stableSnapshotValue, type ConfigSnapshotInput } from "./configSnapshotBuild";
import { createDefaultSettingsForm } from "./settingsForm";

function makeInput<S extends { status_text: string } = { status_text: string }>(overrides: Partial<ConfigSnapshotInput<S>> = {}): ConfigSnapshotInput<S> {
  return {
    settings: createDefaultSettingsForm(),
    dnsProviderSnapshots: [],
    primaryCloudflareSnapshot: null,
    sourceIPMode: "",
    sources: [] as S[],
    ...overrides,
  };
}

describe("stableSnapshotValue / snapshotSignature", () => {
  it("sorts object keys recursively, keeps array order", () => {
    expect(stableSnapshotValue({ b: 1, a: { d: 2, c: 3 } })).toEqual({ a: { c: 3, d: 2 }, b: 1 });
    expect(snapshotSignature({ a: 1, b: 2 })).toBe(snapshotSignature({ b: 2, a: 1 }));
    expect(snapshotSignature([1, 2])).not.toBe(snapshotSignature([2, 1]));
    expect(snapshotSignature({ a: 1 })).not.toBe(snapshotSignature({ a: 2 }));
  });
});

describe("schedulerIntervalMinutesForSnapshot", () => {
  it("falls back to draft when committed value invalid; disabled uses nonNegative", () => {
    const settings = createDefaultSettingsForm();
    // defaults: schedulerIntervalMinutes 0, draft 60
    expect(schedulerIntervalMinutesForSnapshot(settings, true)).toBe(60);
    expect(schedulerIntervalMinutesForSnapshot(settings, false)).toBe(60);
    settings.schedulerIntervalMinutes = 30;
    expect(schedulerIntervalMinutesForSnapshot(settings, true)).toBe(30);
    settings.schedulerIntervalMinutes = -5;
    expect(schedulerIntervalMinutesForSnapshot(settings, true)).toBe(60);
  });
});

describe("buildConfigSnapshot defaults", () => {
  it("locks probe/cloudflare/github/ui/scheduler defaults", () => {
    const snap = buildConfigSnapshot(makeInput());
    expect(snap.probe.strategy).toBe("fast");
    expect(snap.probe.disable_download).toBe(true);
    expect(snap.probe.tcp_port).toBe(443);
    expect(snap.probe.ping_times).toBe(4);
    expect(snap.probe.source_pool_limit).toBe(65536);
    expect(snap.probe.user_agent.length).toBeGreaterThan(0);
    expect(snap.probe.skip_first_latency_sample).toBe(true);
    expect(snap.probe.test_all).toBe(false);
    expect(snap.cloudflare.enabled).toBe(false);
    expect(snap.cloudflare.ttl).toBe(300);
    expect(snap.cloudflare.proxied).toBe(false);
    expect(snap.github.enabled).toBe(false);
    expect(snap.github.branch).toBe("main");
    expect(snap.github.path_template).toBe("cfst-results/{date}/{time}-{task_id}.csv");
    expect(snap.github.commit_message_template).toBe("CFST results {date} {time}");
    expect(snap.ui.settings_mode).toBe("advanced");
    expect(snap.ui.theme_dark_start).toBe("19:00");
    expect(snap.ui.theme_light_start).toBe("07:00");
    expect(snap.scheduler.config_source).toBe("draft_preferred");
    expect(snap.scheduler.interval_minutes).toBe(60);
    expect(snap.scheduler.daily_times).toEqual([]);
    expect(snap.backup.webdav.remote_path).toBe("cfst-gui-config.zip");
    expect(snap.backup.webdav.timeout_seconds).toBe(30);
  });

  it("trims source status_text and passes ip mode through", () => {
    const snap = buildConfigSnapshot(makeInput({
      sourceIPMode: "ipv4",
      sources: [{ id: "s1", status_text: "  就绪  " }],
    }));
    expect(snap.source_ip_mode).toBe("ipv4");
    expect(snap.sources).toEqual([{ id: "s1", status_text: "就绪" }]);
  });

  it("splits webhook urls / wecom urls / mobiles / email to on separators", () => {
    const settings = createDefaultSettingsForm();
    settings.webhookURLs = "https://a.example\nhttps://b.example,;https://c.example";
    settings.webhookWeComURLs = "https://w1\nhttps://w2";
    settings.webhookWeComMobiles = "138x;139y,";
    settings.emailTo = "a@x.com\nb@y.com";
    const snap = buildConfigSnapshot(makeInput({ settings }));
    expect(snap.notifications.webhook.urls).toEqual(["https://a.example", "https://b.example", "https://c.example"]);
    expect(snap.notifications.webhook.wecom_urls).toEqual(["https://w1", "https://w2"]);
    expect(snap.notifications.webhook.wecom_mobiles).toEqual(["138x", "139y"]);
    expect(snap.notifications.email.to).toEqual(["a@x.com", "b@y.com"]);
  });
});

describe("github enable gating", () => {
  it("requires owner/repo/branch/pathTemplate and unmasked token", () => {
    const settings = createDefaultSettingsForm();
    settings.githubOwner = "me";
    settings.githubRepo = "repo";
    settings.githubBranch = "main";
    settings.githubPathTemplate = "p/{date}.csv";
    settings.githubToken = "ghp_realtoken";
    expect(buildConfigSnapshot(makeInput({ settings })).github.enabled).toBe(true);
    settings.githubToken = "ghp_***";
    expect(buildConfigSnapshot(makeInput({ settings })).github.enabled).toBe(false);
    settings.githubToken = "****";
    expect(buildConfigSnapshot(makeInput({ settings })).github.enabled).toBe(false);
    settings.githubToken = "ghp_realtoken";
    settings.githubOwner = "  ";
    expect(buildConfigSnapshot(makeInput({ settings })).github.enabled).toBe(false);
  });
});

describe("strategy / scheduler mode mapping", () => {
  it("full strategy enables download; daily mode emits daily_times and zero interval", () => {
    const settings = createDefaultSettingsForm();
    settings.probeStrategy = "full";
    settings.schedulerTriggerMode = "daily";
    settings.schedulerDailyTimes = "07:30\n19:00";
    const snap = buildConfigSnapshot(makeInput({ settings }));
    expect(snap.probe.strategy).toBe("full");
    expect(snap.probe.disable_download).toBe(false);
    expect(snap.scheduler.interval_minutes).toBe(0);
    expect(snap.scheduler.daily_times).toEqual(["07:30", "19:00"]);
  });
});