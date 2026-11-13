import type { Page } from "@playwright/test";
import { normalizeConfigSnapshot } from "../../frontend/src/lib/bridge/config";
import type { SourceConfig, SourceProfileStore } from "../../frontend/src/lib/bridge/types";
export async function sourceBackend(page: Page, initialSources: SourceConfig[] = []) {
  let snapshot = normalizeConfigSnapshot({ sources: initialSources });
  const profiles: SourceProfileStore = {
    active_profile_id: "original",
    items: [
      {
        colo_filter: "",
        colo_filter_mode: "allow",
        id: "original",
        name: "原输入组",
        ip_mode: snapshot.source_ip_mode,
        sources: initialSources,
        created_at: "",
        updated_at: "",
      },
    ],
    schema_version: "cfst-gui-source-profiles-v1",
    updated_at: "",
  };
  let nextID = 0;
  let previewSource: SourceConfig | null = null;
  await page.route("**/api/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/api/health") {
      await route.fulfill({ json: { service: "cfst-webui", auth_required: false, ok: true } });
      return;
    }
    if (path === "/api/events/probe") {
      await route.fulfill({ body: ": connected\n\n", contentType: "text/event-stream" });
      return;
    }
    const command = decodeURIComponent(path.split("/").pop() || "");
    const payload = (route.request().postDataJSON() || {}) as Record<string, unknown>;
    let data: unknown = {};
    if (command === "GetAppInfo")
      data = { current_version: "test", platform: "webui", install_mode: "webui" };
    if (command === "config.load") data = { config_snapshot: snapshot, source_profiles: profiles };
    if (command === "config.save") {
      snapshot = normalizeConfigSnapshot(payload.config_snapshot);
      data = { config_snapshot: snapshot, source_profiles: profiles };
    }
    if (command === "source_profiles.update_current" || command === "source_profiles.save") {
      const id = String(
        payload.profile_id ||
          (command === "source_profiles.save" ? `copy-${++nextID}` : profiles.active_profile_id),
      );
      const sources = normalizeConfigSnapshot({
        source_ip_mode: payload.ip_mode,
        sources: payload.sources,
      }).sources;
      const mode = normalizeConfigSnapshot({
        source_ip_mode: payload.ip_mode,
        sources,
      }).source_ip_mode;
      const item = {
        id,
        name: String(payload.name || "当前输入源"),
        colo_filter: String(payload.colo_filter || ""),
        colo_filter_mode: String(payload.colo_filter_mode || "allow") as "allow" | "deny",
        ip_mode: mode,
        sources,
        created_at: "",
        updated_at: "",
      };
      const index = profiles.items.findIndex((entry) => entry.id === id);
      if (index < 0) profiles.items.push(item);
      else profiles.items[index] = item;
      if (payload.set_active !== false) {
        profiles.active_profile_id = id;
        snapshot = normalizeConfigSnapshot({ ...snapshot, sources, source_ip_mode: mode });
      }
      data =
        command === "source_profiles.save"
          ? profiles
          : { source_profiles: profiles, sources, config_snapshot: snapshot };
    }
    if (command === "source.preview") {
      const source = payload.source as SourceConfig;
      previewSource = source;
      if (source.url.includes("unavailable")) {
        await route.fulfill({
          json: { ok: false, code: "SOURCE_READ_FAILED", message: "来源服务器返回 503" },
        });
        return;
      }
      data = {
        summary: { total_count: 1, invalid_count: 1, mode: source.ip_mode },
        preview_entries: ["1.1.1.1"],
      };
    }
    await route.fulfill({
      json: {
        ok: true,
        code: "TEST_OK",
        data,
        message: "",
        warnings: [],
        schema_version: "cfst-gui-command-v2",
      },
    });
  });
  return { profiles, snapshot: () => snapshot, preview: () => previewSource };
}
