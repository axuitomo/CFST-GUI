import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createDefaultSettingsForm } from "../lib/config/settingsForm";

const nativeSurface = vi.hoisted(() => ({ setAndroidSurfaceTheme: vi.fn(async () => undefined) }));

vi.mock("../lib/bridge", () => ({
  setAndroidSurfaceTheme: nativeSurface.setAndroidSurfaceTheme,
}));

function stubDom(matches: boolean) {
  const toggle = vi.fn();
  const documentStub = { documentElement: { classList: { toggle }, dataset: {} as Record<string, string> } };
  const mediaQuery = { addEventListener: vi.fn(), matches, removeEventListener: vi.fn() };
  const store = new Map<string, string>();
  const windowStub = {
    clearTimeout: (id: number) => clearTimeout(id),
    localStorage: {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => {
        store.set(key, value);
      },
    },
    matchMedia: vi.fn(() => mediaQuery),
    setTimeout: (fn: () => void, ms: number) => setTimeout(fn, ms),
  };
  vi.stubGlobal("document", documentStub);
  vi.stubGlobal("window", windowStub);
  return { mediaQuery, toggle, dataset: documentStub.documentElement.dataset };
}

async function freshTheme() {
  vi.resetModules();
  const { useTheme } = await import("./useTheme");
  return useTheme;
}

describe("useTheme", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    nativeSurface.setAndroidSurfaceTheme.mockClear();
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("light/dark 直用并同步 dataset 与 dark class", async () => {
    const { dataset, toggle } = stubDom(false);
    const useTheme = await freshTheme();
    const settings = createDefaultSettingsForm();
    const { applyThemeMode } = useTheme(settings);
    settings.themeMode = "light";
    applyThemeMode();
    expect(dataset.theme).toBe("light");
    expect(toggle).toHaveBeenLastCalledWith("dark", false);
    settings.themeMode = "dark";
    applyThemeMode();
    expect(dataset.theme).toBe("dark");
    expect(toggle).toHaveBeenLastCalledWith("dark", true);
  });

  it("auto_system_time 跟随系统偏好", async () => {
    const { dataset } = stubDom(true);
    const useTheme = await freshTheme();
    const settings = createDefaultSettingsForm();
    settings.themeMode = "auto_system_time";
    const { installThemeTracking } = useTheme(settings);
    installThemeTracking();
    expect(dataset.theme).toBe("dark");
  });

  it("auto_system_time 未安装监听时回退时钟窗口（原 App 行为）", async () => {
    const { dataset } = stubDom(true);
    const useTheme = await freshTheme();
    const settings = createDefaultSettingsForm();
    settings.themeMode = "auto_system_time";
    settings.utcOffsetMinutes = 0;
    const { applyThemeMode } = useTheme(settings);
    vi.setSystemTime(new Date("2026-01-15T12:00:00Z"));
    applyThemeMode();
    expect(dataset.theme).toBe("light");
  });

  it("auto_time 按时钟窗口判定，支持跨午夜窗口", async () => {
    const { dataset } = stubDom(false);
    const useTheme = await freshTheme();
    const settings = createDefaultSettingsForm();
    settings.themeMode = "auto_time";
    settings.utcOffsetMinutes = 0;
    settings.themeLightStart = "07:00";
    settings.themeDarkStart = "19:00";
    const { applyThemeMode } = useTheme(settings);
    vi.setSystemTime(new Date("2026-01-15T12:00:00Z"));
    applyThemeMode();
    expect(dataset.theme).toBe("light");
    vi.setSystemTime(new Date("2026-01-15T20:00:00Z"));
    applyThemeMode();
    expect(dataset.theme).toBe("dark");
    settings.themeLightStart = "22:00";
    settings.themeDarkStart = "06:00";
    vi.setSystemTime(new Date("2026-01-15T12:00:00Z"));
    applyThemeMode();
    expect(dataset.theme).toBe("dark");
  });

  it("install 注册偏好监听并启动 60s 轮询", async () => {
    const { dataset, mediaQuery } = stubDom(false);
    const useTheme = await freshTheme();
    const settings = createDefaultSettingsForm();
    settings.themeMode = "light";
    const { installThemeTracking } = useTheme(settings);
    installThemeTracking();
    expect(mediaQuery.addEventListener).toHaveBeenCalledWith("change", expect.any(Function));
    expect(dataset.theme).toBe("light");
    settings.themeMode = "dark";
    vi.advanceTimersByTime(60_000);
    expect(dataset.theme).toBe("dark");
  });

  it("detach 移除监听，stop 停掉轮询", async () => {
    const { dataset, mediaQuery } = stubDom(false);
    const useTheme = await freshTheme();
    const settings = createDefaultSettingsForm();
    settings.themeMode = "light";
    const { detachThemeMediaListener, installThemeTracking, stopThemeRefreshTimer } = useTheme(settings);
    installThemeTracking();
    detachThemeMediaListener();
    stopThemeRefreshTimer();
    expect(mediaQuery.removeEventListener).toHaveBeenCalledWith("change", expect.any(Function));
    settings.themeMode = "dark";
    vi.advanceTimersByTime(120_000);
    expect(dataset.theme).toBe("light");
  });
});
