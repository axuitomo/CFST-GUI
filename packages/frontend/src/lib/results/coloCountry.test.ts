import { afterEach, describe, expect, it, vi } from "vitest";
import { coloCountryCode, formatColoCountry, readColoCountryFormat, saveColoCountryFormat } from "./coloCountry";

describe("coloCountry", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("已知机场码返回国家码与中文名", () => {
    expect(coloCountryCode("hkg")).toBe("HK");
    expect(formatColoCountry("HKG", "name")).toBe("中国香港");
    expect(formatColoCountry("HKG", "code")).toBe("HK");
    expect(formatColoCountry("TPE", "name")).toBe("中国台湾");
    expect(formatColoCountry("NRT", "name")).toBe("日本");
  });

  it("未知或空机场码统一显示 -", () => {
    expect(coloCountryCode("ZZZ")).toBe("");
    expect(formatColoCountry("ZZZ", "name")).toBe("-");
    expect(formatColoCountry(undefined, "code")).toBe("-");
  });

  it("显示口径持久化在 localStorage，默认中文名称", () => {
    const store: Record<string, string> = {};
    vi.stubGlobal("window", {
      localStorage: {
        getItem: (key: string) => store[key] ?? null,
        setItem: (key: string, value: string) => {
          store[key] = value;
        },
      },
    });
    expect(readColoCountryFormat()).toBe("name");
    saveColoCountryFormat("code");
    expect(readColoCountryFormat()).toBe("code");
  });
});
