import { commandResult, normalizeCommandResult } from "../../bridge/command";
import { normalizeProbeEvent } from "../../bridge/normalize";
import { isObject, toStringValue } from "../../bridgeValues";
import type { PlatformAdapter, PlatformMethodName, ProbeEventSubscriptionOptions } from "../types";
import { downloadBase64File, downloadBlobFile, openWebUIPath, selectWebUIPath } from "./files";
import { ensureWebUIToken, webUIApp, webUIFetch, webUIJSON, webUITokenQuery } from "./transport";

export const webUIAdapter: PlatformAdapter = {
  mode: "webui",

  async invokeCore<T>(command: string, payloadJSON: string) {
    const response = await webUIFetch(`/api/command/${encodeURIComponent(command)}`, {
      body: payloadJSON,
      method: "POST",
    });
    return normalizeCommandResult<T>(await webUIJSON(response));
  },

  async invokePlatform<T>(method: PlatformMethodName, payload?: Record<string, unknown>) {
    return normalizeCommandResult<T>(await webUIApp(method, payload ?? {}));
  },

  webui: {
    async getEventSubscriptionKey() {
      return ensureWebUIToken();
    },

    downloadExportedFile(fileName: string, content: string, encoding: "utf8" | "base64", mimeType: string) {
      if (encoding === "base64") {
        downloadBase64File(fileName, content, mimeType);
        return;
      }
      downloadBlobFile(fileName, content, mimeType);
    },
  },

  async subscribeProbeEvents({ onEvent, onTransportDrift }: ProbeEventSubscriptionOptions) {
    const token = await ensureWebUIToken();
    const source = new EventSource(`/api/events/probe${webUITokenQuery(token)}`);
    source.onmessage = (message) => {
      try {
        const event = normalizeProbeEvent(JSON.parse(message.data));
        if (event) {
          onEvent(event);
        }
      } catch {
        // Ignore malformed frames; the next valid event or snapshot reconciliation repairs state.
      }
    };
    source.onerror = () => {
      onTransportDrift?.();
    };
    return () => source.close();
  },

  selectPath(payload: Record<string, unknown>) {
    return selectWebUIPath(payload);
  },

  openPath(targetPath: string) {
    return openWebUIPath(targetPath);
  },

  async openLogDirectory(payload: Record<string, unknown>) {
    return normalizeCommandResult(await webUIApp("OpenLogDirectory", payload));
  },

  async openReleasePage() {
    const result = normalizeCommandResult(await webUIApp("OpenReleasePage"));
    const releaseUrl = toStringValue(isObject(result.data) ? result.data.release_url : "");
    if (releaseUrl) {
      window.open(releaseUrl, "_blank", "noopener,noreferrer");
    }
    return result;
  },

  async consumeDroppedSources() {
    return commandResult<{ paths: string[] }>("SOURCES_DROPPED_EMPTY", { paths: [] });
  },
};
