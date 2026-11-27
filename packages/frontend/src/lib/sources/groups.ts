import type { SourceConfig, SourceIPMode } from "../bridge/types";

export type SourceGroupIPMode = SourceIPMode | "";

// 空模式只用于保留旧配置里混用的模式；用户明确选择后，全组统一。
export function resolveSourceGroupIPMode(value: unknown, sources: Pick<SourceConfig, "ip_mode">[]): SourceGroupIPMode {
  const explicit = String(value ?? "")
    .trim()
    .toLowerCase();
  if (explicit === "traverse" || explicit === "mcis") return explicit;
  const modes = new Set(sources.map((source) => (source.ip_mode === "mcis" ? "mcis" : "traverse")));
  return modes.size > 1 ? "" : modes.has("mcis") ? "mcis" : "traverse";
}

export function sourceInputIssue(source: Pick<SourceConfig, "kind" | "content" | "path" | "url">): string {
  if (source.kind === "inline") return source.content.trim() ? "" : "请粘贴 IP、CIDR 或域名，每行一条。";
  if (source.kind === "file") return source.path.trim() ? "" : "请选择文件或填写文件路径。";
  const value = source.url.trim();
  if (!value) return "请填写来源链接，例如 https://example.com/ips.txt。";
  try {
    const url = new URL(value.startsWith("//") ? `https:${value}` : value.includes("://") ? value : `https://${value}`);
    if (!["http:", "https:"].includes(url.protocol) || !url.hostname || /\s/.test(value)) {
      return "链接必须是有效的 HTTP 或 HTTPS 地址。";
    }
    return "";
  } catch {
    return "链接格式不正确，请检查主机名和地址。";
  }
}

export const duplicateSourceUrlIssue = "该链接与其它输入源重复，同一链接只保留一个，请删除或修改其中一个。";

// 同链接判定只做最常见的等价归一：忽略协议前缀、末尾斜杠与大小写差异。
function sourceUrlKey(value: string): string {
  return value
    .trim()
    .replace(/^https?:\/\//i, "")
    .replace(/\/+$/, "")
    .toLowerCase();
}

// 返回“与前面某个输入源链接相同”的来源 ID；第一个出现的来源保留。
export function duplicateSourceUrlIds(sources: Pick<SourceConfig, "id" | "kind" | "url">[]): Set<string> {
  const seen = new Set<string>();
  const duplicates = new Set<string>();
  for (const source of sources) {
    if (source.kind !== "url") continue;
    const key = sourceUrlKey(source.url);
    if (!key) continue;
    if (seen.has(key)) duplicates.add(source.id);
    else seen.add(key);
  }
  return duplicates;
}
export function sourcePreviewKey(source: SourceConfig, groupColoFilter = "", groupColoFilterMode: "allow" | "deny" | "" = ""): string {
  return JSON.stringify([source.kind, source.content.trim(), source.path.trim(), source.url.trim(), source.ip_mode, source.colo_filter.trim(), source.colo_filter_mode, groupColoFilter.trim(), groupColoFilterMode]);
}
