import { Application, Events, Window } from "@wailsio/runtime";

const mainWindow = Window.Get("main");

export const WindowCenter = () => mainWindow.Center();
export const WindowGetSize = async () => ({ w: await mainWindow.Width(), h: await mainWindow.Height() });
export const WindowIsMaximised = () => mainWindow.IsMaximised();
export const WindowMaximise = () => mainWindow.Maximise();
export const WindowSetSize = (width: number, height: number) => mainWindow.SetSize(width, height);
export const WindowUnfullscreen = () => mainWindow.Restore();
export const WindowUnmaximise = () => mainWindow.Restore();
export const WindowMinimise = () => mainWindow.Minimise();
export const WindowToggleMaximise = async () => {
  if (await mainWindow.IsMaximised()) {
    return mainWindow.Restore();
  }
  return mainWindow.Maximise();
};
export const Quit = () => Application.Quit();

export function EventsOn(eventName: string, callback: (payload: unknown) => void) {
  return Events.On(eventName, (event) => callback(event.data));
}

export function isWailsRuntimeAvailable() {
  return typeof window !== "undefined" && "_wails" in window;
}
