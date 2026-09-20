import { setAndroidSurfaceTheme } from "./bridge";

/**
 * 把解析后的主题同步到 Vue 之外的两层界面：
 * 1. `localStorage`：`index.html` 内联的启动画面在打包代码加载前读它，避免深色主题先闪一帧浅色；
 * 2. Android 原生窗口与 WebView 底色（`SetSurfaceTheme`）：覆盖 WebView 重建、冷启动和 relaunch 的空档。
 * 只记录解析结果，不参与主题判断；浏览器 / WebUI / 桌面端第 2 步静默跳过。
 */
const STORAGE_KEY = "cfst.surface-theme";
export type SurfaceThemeMode = "light" | "dark";

let appliedMode: SurfaceThemeMode | null = null;

export function applySurfaceTheme(mode: SurfaceThemeMode): void {
  if (mode === appliedMode) {
    return;
  }
  appliedMode = mode;
  try {
    window.localStorage.setItem(STORAGE_KEY, mode);
  } catch {
    // 隐私模式等场景下存储不可用：只影响下次冷启动的首帧底色。
  }
  void setAndroidSurfaceTheme(mode === "dark").catch(() => undefined);
}
