import "./directoryPicker.css";

export interface WebUIFileEntry {
  is_dir: boolean;
  name: string;
  path: string;
  size: number;
}

export interface WebUIFileList {
  entries: WebUIFileEntry[];
  path: string;
  roots: string[];
}

export type WebUIFileListFetcher = (path: string) => Promise<WebUIFileList>;

// WebUI 服务端目录选择器：DOM 用标准 createElement 组装，样式全部走
// directoryPicker.css 的 class（原 style.cssText 内联字符串已歼灭）。
// 目录列表读取由调用方注入（webui adapter 的 fetchWebUIFileList 带令牌与重试），
// 本模块不触碰平台通道，行为与迁移前一致：上一级/双击进入/根快捷方式/
// 打开/选择当前目录/取消，读取失败时在列表区显示错误信息。
export async function browseWebUIDirectory(startPath: string, title: string, fetchList: WebUIFileListFetcher): Promise<string | null> {
  const overlay = document.createElement("div");
  overlay.className = "cfst-dir-picker";
  const panel = document.createElement("div");
  panel.className = "cfst-dir-picker__panel";

  const header = document.createElement("div");
  header.className = "cfst-dir-picker__header";
  header.textContent = title || "选择服务端目录";

  const pathRow = document.createElement("div");
  pathRow.className = "cfst-dir-picker__path-row";
  const input = document.createElement("input");
  input.className = "cfst-dir-picker__input";
  const goButton = document.createElement("button");
  goButton.className = "cfst-dir-picker__go";
  goButton.textContent = "打开";
  pathRow.append(input, goButton);

  const roots = document.createElement("div");
  roots.className = "cfst-dir-picker__roots";

  const list = document.createElement("div");
  list.className = "cfst-dir-picker__list";

  const actionRow = document.createElement("div");
  actionRow.className = "cfst-dir-picker__actions";
  const parentButton = document.createElement("button");
  parentButton.className = "cfst-dir-picker__secondary";
  parentButton.textContent = "上一级";
  const spacer = document.createElement("span");
  spacer.className = "cfst-dir-picker__spacer";
  const cancelButton = document.createElement("button");
  cancelButton.className = "cfst-dir-picker__secondary";
  cancelButton.textContent = "取消";
  const chooseButton = document.createElement("button");
  chooseButton.className = "cfst-dir-picker__choose";
  chooseButton.textContent = "选择当前目录";
  actionRow.append(parentButton, spacer, cancelButton, chooseButton);

  panel.append(header, pathRow, roots, list, actionRow);
  overlay.appendChild(panel);
  document.body.appendChild(overlay);

  let currentPath = startPath || "";

  const render = async (path: string) => {
    list.textContent = "正在读取目录...";
    const data = await fetchList(path);
    currentPath = data.path;
    input.value = currentPath;
    roots.replaceChildren();
    data.roots.forEach((root) => {
      const button = document.createElement("button");
      button.textContent = root;
      button.className = "cfst-dir-picker__root";
      button.onclick = () => void render(root);
      roots.appendChild(button);
    });
    list.replaceChildren();
    data.entries.forEach((entry) => {
      const row = document.createElement("button");
      row.type = "button";
      row.disabled = !entry.is_dir;
      row.textContent = `${entry.is_dir ? "[D]" : "[F]"} ${entry.name}`;
      row.className = entry.is_dir ? "cfst-dir-picker__entry cfst-dir-picker__entry--dir" : "cfst-dir-picker__entry cfst-dir-picker__entry--file";
      row.ondblclick = () => entry.is_dir && void render(entry.path);
      row.onclick = () => {
        if (entry.is_dir) {
          input.value = entry.path;
        }
      };
      list.appendChild(row);
    });
    if (data.entries.length === 0) {
      list.textContent = "目录为空。";
    }
  };

  return new Promise<string | null>((resolve) => {
    const close = (value: string | null) => {
      overlay.remove();
      resolve(value);
    };
    goButton.onclick = () => void render(input.value);
    parentButton.onclick = () => {
      const parent = currentPath.replace(/[\\/]+$/, "").replace(/[\\/][^\\/]*$/, "") || currentPath;
      void render(parent);
    };
    cancelButton.onclick = () => close(null);
    chooseButton.onclick = () => close(input.value || currentPath);
    void render(currentPath).catch((error) => {
      list.textContent = error instanceof Error ? error.message : "读取目录失败。";
    });
  });
}
