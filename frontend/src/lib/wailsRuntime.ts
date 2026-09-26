import { Application, Events, Window } from "@wailsio/runtime";
import { InitializeNotifications, SendNotification } from "../../wailsjs/runtime/runtime";

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

export async function showDesktopNotification(title: string, body: string) {
  if (!isWailsRuntimeAvailable()) return;
  await InitializeNotifications();
  await SendNotification({ id: `cfst-${Date.now()}`, title, body });
}
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

// isWailsDesktopHost 判断页面是不是由 Wails 资源服务托管：Windows 是
// http://wails.localhost，Linux/macOS 是 wails://localhost（wails v3
// internal/assetserver/assetserver_{windows,linux,darwin}.go 的 baseURL）。它只看页面
// 地址、不看运行时是否注入，所以启动瞬间就可用，是「桌面页面不可能走 WebUI 通道」的
// 兜底依据：桌面端打到只有 webui 构建才有的 /api/command/{command} 会得到 404，这正是
// 右下角「WebUI 请求失败 (404)」的来源。
export function isWailsDesktopHost() {
  if (typeof window === "undefined" || !window.location) {
    return false;
  }
  const { hostname, protocol } = window.location;
  return protocol === "wails:" || hostname === "wails.localhost";
}

// waitForWailsRuntime 等宿主把 _wails.environment 注入进来。wails v3 的注入点是
// navigationCompleted 时的 execJS（internal/runtime/runtime.go 的 runtimeConfigReady），
// 晚于页面脚本，所以启动竞态下必须等一次；超时返回 false，让调用方退回兜底通道。
export function waitForWailsRuntime(timeoutMs = 3000): Promise<boolean> {
  if (isWailsRuntimeAvailable()) {
    return Promise.resolve(true);
  }
  return new Promise<boolean>((resolve) => {
    let settled = false;
    const finish = (available: boolean) => {
      if (settled) {
        return;
      }
      settled = true;
      window.clearInterval(poll);
      window.clearTimeout(timer);
      resolve(available);
    };
    const poll = window.setInterval(() => {
      if (isWailsRuntimeAvailable()) {
        finish(true);
      }
    }, 25);
    const timer = window.setTimeout(() => finish(false), timeoutMs);
  });
}
