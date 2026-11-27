import { afterEach, describe, expect, it, vi } from "vitest";
import { buildCopyQueue, normalizeCopyCount, readResultCopySettings, resolveCopyLimit, RESULT_COPY_SETTINGS_KEY, saveResultCopySettings } from "./copy";

describe("resultCopy", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("按档位截取并按展示顺序去重同一 IP", () => {
    const rows = [{ address: "1.1.1.1" }, { address: "2.2.2.2" }, { address: "1.1.1.1" }, { address: "3.3.3.3" }];

    expect(buildCopyQueue(rows, { customCount: 10, mode: "all" })).toEqual(["1.1.1.1", "2.2.2.2", "3.3.3.3"]);
    expect(buildCopyQueue(rows, { customCount: 10, mode: "5" })).toEqual(["1.1.1.1", "2.2.2.2", "3.3.3.3"]);
    expect(buildCopyQueue(rows, { customCount: 2, mode: "custom" })).toEqual(["1.1.1.1", "2.2.2.2"]);
  });

  it("自定义数量限制在 1 到 9999", () => {
    expect(normalizeCopyCount(0)).toBe(10);
    expect(normalizeCopyCount("abc")).toBe(10);
    expect(normalizeCopyCount(1)).toBe(1);
    expect(normalizeCopyCount(50000)).toBe(9999);
    expect(resolveCopyLimit({ customCount: 9999, mode: "custom" })).toBe(9999);
    expect(resolveCopyLimit({ customCount: 10, mode: "all" })).toBe(0);
  });

  it("档位与自定义数量写入 localStorage 并回显", () => {
    const store: Record<string, string> = {};
    vi.stubGlobal("window", {
      localStorage: {
        getItem: (key: string) => store[key] ?? null,
        setItem: (key: string, value: string) => {
          store[key] = value;
        },
      },
    });

    expect(readResultCopySettings()).toEqual({ customCount: 10, mode: "all" });
    saveResultCopySettings({ customCount: 321, mode: "custom" });
    expect(JSON.parse(store[RESULT_COPY_SETTINGS_KEY])).toEqual({ customCount: 321, mode: "custom" });
    expect(readResultCopySettings()).toEqual({ customCount: 321, mode: "custom" });

    store[RESULT_COPY_SETTINGS_KEY] = "not json";
    expect(readResultCopySettings()).toEqual({ customCount: 10, mode: "all" });
  });
});
