import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const nativeSurface = vi.hoisted(() => ({ setAndroidSurfaceTheme: vi.fn(async () => undefined) }));

vi.mock("./bridge", () => ({
  setAndroidSurfaceTheme: nativeSurface.setAndroidSurfaceTheme,
}));

function stubBrowserStorage() {
  const store = new Map<string, string>();
  vi.stubGlobal("window", {
    localStorage: {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => {
        store.set(key, value);
      },
    },
  });
  return store;
}

describe("applySurfaceTheme", () => {
  beforeEach(() => {
    nativeSurface.setAndroidSurfaceTheme.mockClear();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.resetModules();
  });

  it("caches the resolved mode and notifies the Android surface only on change", async () => {
    const store = stubBrowserStorage();
    const { applySurfaceTheme } = await import("./surfaceTheme");

    applySurfaceTheme("light");
    applySurfaceTheme("light");

    expect(nativeSurface.setAndroidSurfaceTheme).toHaveBeenCalledTimes(1);
    expect(nativeSurface.setAndroidSurfaceTheme).toHaveBeenCalledWith(false);
    expect(store.get("cfst.surface-theme")).toBe("light");

    applySurfaceTheme("dark");

    expect(nativeSurface.setAndroidSurfaceTheme).toHaveBeenCalledTimes(2);
    expect(nativeSurface.setAndroidSurfaceTheme).toHaveBeenLastCalledWith(true);
    expect(store.get("cfst.surface-theme")).toBe("dark");
  });

  it("keeps working when the page has no web storage", async () => {
    vi.stubGlobal("window", {});
    const { applySurfaceTheme } = await import("./surfaceTheme");

    applySurfaceTheme("dark");

    expect(nativeSurface.setAndroidSurfaceTheme).toHaveBeenCalledWith(true);
  });
});
