import { commandResult } from "../../bridge/command";
import type { CommandResult, PathSelectionPayload } from "../../bridge/types";
import { toStringValue } from "../../bridgeValues";
import { browseWebUIDirectory as pickWebUIDirectory, type WebUIFileList } from "./directoryPicker";
import { ensureWebUIToken, webUIFetch, webUIJSON } from "./transport";

function arrayBufferToBase64(buffer: ArrayBuffer) {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  const chunkSize = 0x8000;
  for (let index = 0; index < bytes.length; index += chunkSize) {
    const chunk = bytes.subarray(index, index + chunkSize);
    binary += String.fromCharCode(...chunk);
  }
  return btoa(binary);
}

export function downloadBase64File(fileName: string, contentBase64: string, mimeType = "application/octet-stream") {
  const link = document.createElement("a");
  link.href = `data:${mimeType};base64,${contentBase64}`;
  link.download = fileName || "download";
  document.body.appendChild(link);
  link.click();
  link.remove();
}

export function downloadBlobFile(fileName: string, content: string, mimeType = "application/octet-stream") {
  const link = document.createElement("a");
  link.href = URL.createObjectURL(new Blob([content], { type: mimeType }));
  link.download = fileName || "download";
  document.body.appendChild(link);
  link.click();
  URL.revokeObjectURL(link.href);
  link.remove();
}

async function selectBrowserFile(mode: string): Promise<CommandResult<PathSelectionPayload>> {
  const input = document.createElement("input");
  input.type = "file";
  input.accept = mode === "config_archive_import" ? ".zip,.json,application/zip,application/json" : ".txt,.csv,text/plain,text/csv,*/*";
  const file = await new Promise<File | null>((resolve) => {
    input.onchange = () => resolve(input.files?.[0] || null);
    input.click();
  });
  if (!file) {
    return commandResult("PATH_SELECTION_CANCELED", { canceled: true, mode }, { message: "已取消选择文件。" });
  }
  if (mode === "config_archive_import") {
    return commandResult(
      "PATH_SELECTED",
      {
        canceled: false,
        content_base64: arrayBufferToBase64(await file.arrayBuffer()),
        display_name: file.name,
        file_name: file.name,
        mode,
        path: `browser-upload:${file.name}`,
      },
      { message: "已选择配置压缩包。" },
    );
  }
  return commandResult(
    "PATH_SELECTED",
    {
      canceled: false,
      content: await file.text(),
      display_name: file.name,
      file_name: file.name,
      mode,
      path: `browser-upload:${file.name}`,
    },
    { message: "已选择输入源文件。" },
  );
}

async function fetchWebUIFileList(path: string): Promise<WebUIFileList> {
  const query = path ? `?path=${encodeURIComponent(path)}` : "";
  const response = await webUIFetch(`/api/files/list${query}`, { method: "GET" });
  return (await webUIJSON(response)) as WebUIFileList;
}

// 薄调用：实现与样式在 adapters/webui/directoryPicker（S2 字符串代码歼灭），
// 这里只注入带令牌与重试的目录列表读取器。
function browseWebUIDirectory(startPath: string, title: string) {
  return pickWebUIDirectory(startPath, title, fetchWebUIFileList);
}

export async function selectWebUIPath(payload: Record<string, unknown>) {
  const mode = toStringValue(payload.mode ?? payload.kind).trim() || "source_file";
  const title = toStringValue(payload.title);
  const defaultFileName = toStringValue(payload.default_file_name ?? payload.defaultFileName).trim() || "result.csv";
  if (mode === "source_file" || mode === "config_archive_import" || mode === "config_import" || mode === "import_config") {
    return selectBrowserFile(mode === "config_archive_import" || mode === "config_import" || mode === "import_config" ? "config_archive_import" : mode);
  }
  if (mode === "config_archive_export" || mode === "config_export" || mode === "save_file") {
    return commandResult<PathSelectionPayload>(
      "PATH_SELECTED",
      {
        canceled: false,
        file_name: defaultFileName,
        mode,
        target_uri: `browser-download:${defaultFileName}`,
      },
      { message: "已选择浏览器下载。" },
    );
  }
  const selected = await browseWebUIDirectory(toStringValue(payload.current_path ?? payload.currentPath), title || "选择服务端目录");
  if (!selected) {
    return commandResult<PathSelectionPayload>("PATH_SELECTION_CANCELED", { canceled: true, mode }, { message: "已取消选择目录。" });
  }
  return commandResult<PathSelectionPayload>(
    "PATH_SELECTED",
    {
      canceled: false,
      directory: selected,
      mode,
      path: selected,
    },
    { message: "已选择服务端目录。" },
  );
}

export async function openWebUIPath(targetPath: string) {
  if (/^https?:\/\//i.test(targetPath)) {
    window.open(targetPath, "_blank", "noopener,noreferrer");
    return;
  }
  try {
    await fetchWebUIFileList(targetPath);
    await browseWebUIDirectory(targetPath, "浏览服务端目录");
    return;
  } catch {
    const token = await ensureWebUIToken();
    window.open(`/api/files/download?path=${encodeURIComponent(targetPath)}${token ? `&token=${encodeURIComponent(token)}` : ""}`, "_blank", "noopener,noreferrer");
  }
}
