import { Application, Events, Window } from "@wailsio/runtime";

let mainWindow: ReturnType<typeof Window.Get> | undefined;

// 惰性获取主窗口：窗口 API 一旦被调用就会向 Wails 宿主发起 IPC 请求，而
// WebUI/浏览器没有 Wails 宿主（见 isWailsRuntimeAvailable）。这里延迟到真正
// 调用窗口 API 时才初始化，WebUI 下这些函数不会被调用，也就不会产生请求。
function getMainWindow() {
  if (!mainWindow) {
    mainWindow = Window.Get("main");
  }
  return mainWindow;
}

export const WindowCenter = () => getMainWindow().Center();
export const WindowGetSize = async () => {
  const w = await getMainWindow().Width();
  const h = await getMainWindow().Height();
  return { w, h };
};
export const WindowIsMaximised = () => getMainWindow().IsMaximised();
export const WindowMaximise = () => getMainWindow().Maximise();
export const WindowSetSize = (width: number, height: number) => getMainWindow().SetSize(width, height);
export const WindowUnfullscreen = () => getMainWindow().Restore();
export const WindowUnmaximise = () => getMainWindow().Restore();
export const WindowMinimise = () => getMainWindow().Minimise();
export const WindowToggleMaximise = async () => {
  if (await getMainWindow().IsMaximised()) {
    return getMainWindow().Restore();
  }
  return getMainWindow().Maximise();
};
export const Quit = () => Application.Quit();

export function EventsOn(eventName: string, callback: (payload: unknown) => void) {
  return Events.On(eventName, (event) => callback(event.data));
}

export function isWailsRuntimeAvailable() {
  if (typeof window === "undefined") {
    return false;
  }
  // 只有 Wails 桌面宿主会注入 _wails.environment（wails v3 internal/runtime/runtime.go），
  // 而 @wailsio/runtime 模块自身在任何 DOM 环境都会创建空对象 window._wails，
  // 因此 `"_wails" in window` 在 WebUI/浏览器里恒为真：那会让 WebUI 误走桌面 IPC 通道，
  // 把命令发往不存在的 /wails/runtime（405）并在 WebUI 下渲染桌面窗口按钮。
  const host = (window as Window & { _wails?: { environment?: { OS?: string } } })._wails;
  return Boolean(host?.environment);
}
