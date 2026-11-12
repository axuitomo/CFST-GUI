import { describe, expect, it } from "vitest";
import { normalizeConfigSnapshot, normalizeSourceConfig, normalizeSourceProfileStore } from "./bridge/config";
import { resolveSourceGroupIPMode, sourceInputIssue, sourcePreviewKey } from "./sourceGroups";

const source = (input = {}) => normalizeSourceConfig(input, 0);

describe("source groups", () => {
  it("inherits a group mode and retains it for an empty group", () => {
    const snapshot = normalizeConfigSnapshot({ source_ip_mode: "mcis", sources: [{ ip_mode: "traverse" }] });
    expect(snapshot.source_ip_mode).toBe("mcis");
    expect(snapshot.sources[0].ip_mode).toBe("mcis");
    expect(normalizeConfigSnapshot({ source_ip_mode: "mcis", sources: [] }).source_ip_mode).toBe("mcis");
  });

  it("infers uniform legacy modes without silently changing mixed sources", () => {
    expect(resolveSourceGroupIPMode(undefined, [])).toBe("traverse");
    expect(resolveSourceGroupIPMode(undefined, [source({ ip_mode: "mcis" })])).toBe("mcis");
    const mixed = normalizeConfigSnapshot({ sources: [{ ip_mode: "mcis" }, { ip_mode: "traverse" }] });
    expect(mixed.source_ip_mode).toBe("");
    expect(mixed.sources.map((entry) => entry.ip_mode)).toEqual(["mcis", "traverse"]);
    const profile = normalizeSourceProfileStore({ items: [{ ip_mode: "mcis", sources: [{ ip_mode: "traverse" }] }] }).items[0];
    expect(profile.ip_mode).toBe("mcis");
    expect(profile.sources[0].ip_mode).toBe("mcis");
  });

  it("reports missing content for the chosen type and rejects unusable URLs", () => {
    for (const kind of ["inline", "file", "url"]) expect(sourceInputIssue(source({ kind }))).not.toBe("");
    expect(sourceInputIssue(source({ kind: "inline", content: "1.1.1.1" }))).toBe("");
    expect(sourceInputIssue(source({ kind: "file", path: "ip.txt" }))).toBe("");
    for (const url of ["example.com/ips.txt", "//example.com/ips.txt", "https://[::1]/ips"]) {
      expect(sourceInputIssue(source({ url }))).toBe("");
    }
    for (const url of ["ftp://example.com/ips", "https://", "bad address"]) {
      expect(sourceInputIssue(source({ url }))).not.toBe("");
    }
  });

  it("invalidates previews on content, mode and filter changes", () => {
    const original = source({ kind: "inline", content: "1.1.1.1" });
    for (const change of [{ content: "1.0.0.1" }, { ip_mode: "mcis" }, { colo_filter: "JP" }, { colo_filter_mode: "deny" }]) {
      expect(sourcePreviewKey(source({ ...original, ...change }))).not.toBe(sourcePreviewKey(original));
    }
    expect(sourcePreviewKey({ ...original, name: "renamed" })).toBe(sourcePreviewKey(original));
  });

  it("normalizes uniform legacy COLO filters to the group and preserves mixed legacy filters", () => {
    const uniform = normalizeSourceProfileStore({
      items: [
        {
          id: "group",
          sources: [
            { colo_filter: "JP,HKG", colo_filter_mode: "deny" },
            { colo_filter: "JP,HKG", colo_filter_mode: "deny" },
          ],
        },
      ],
    }).items[0];
    expect(uniform.colo_filter).toBe("JP,HKG");
    expect(uniform.colo_filter_mode).toBe("deny");

    const mixed = normalizeSourceProfileStore({
      items: [
        {
          id: "legacy",
          sources: [
            { colo_filter: "JP", colo_filter_mode: "allow" },
            { colo_filter: "US", colo_filter_mode: "deny" },
          ],
        },
      ],
    }).items[0];
    expect(mixed.colo_filter_mode).toBe("");
    expect(mixed.sources.map((entry) => entry.colo_filter)).toEqual(["JP", "US"]);
  });

  it("invalidates previews when group COLO settings change", () => {
    const original = source({ kind: "inline", content: "1.1.1.1" });
    expect(sourcePreviewKey(original, "JP", "allow")).not.toBe(sourcePreviewKey(original, "US", "allow"));
    expect(sourcePreviewKey(original, "JP", "allow")).not.toBe(sourcePreviewKey(original, "JP", "deny"));
  });
});
