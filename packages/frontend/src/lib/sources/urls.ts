export interface SourceUrlCdnSwitch {
  label: string;
  nextUrl: string;
}

function parseSourceUrl(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return null;
  }

  try {
    return new URL(trimmed);
  } catch {
    try {
      return new URL(`https://${trimmed}`);
    } catch {
      return null;
    }
  }
}

function pathSegments(value: string) {
  return value
    .replace(/^\/+|\/+$/g, "")
    .split("/")
    .map((part) => part.trim())
    .filter(Boolean);
}

function buildURL(host: string, path: string) {
  const target = new URL(`https://${host}`);
  target.pathname = path;
  return target.toString();
}

export function githubRawToJsDelivrUrl(value: string) {
  const parsed = parseSourceUrl(value);
  if (!parsed || parsed.hostname.toLowerCase() !== "raw.githubusercontent.com") {
    return "";
  }

  const segments = pathSegments(parsed.pathname);
  if (segments.length < 4) {
    return "";
  }

  const owner = segments[0];
  const repo = segments[1];
  let branchIndex = 2;
  if (segments.length >= 6 && segments[2] === "refs" && segments[3] === "heads") {
    branchIndex = 4;
  }
  const branch = segments[branchIndex];
  const fileSegments = segments.slice(branchIndex + 1);
  if (!owner || !repo || !branch || fileSegments.length === 0) {
    return "";
  }

  return buildURL("cdn.jsdelivr.net", `/gh/${owner}/${repo}@${branch}/${fileSegments.join("/")}`);
}

export function jsDelivrToGithubRawUrl(value: string) {
  const parsed = parseSourceUrl(value);
  if (!parsed || parsed.hostname.toLowerCase() !== "cdn.jsdelivr.net") {
    return "";
  }

  const segments = pathSegments(parsed.pathname);
  if (segments.length < 4 || segments[0] !== "gh") {
    return "";
  }

  const [repo, branch] = segments[2].split("@");
  const owner = segments[1];
  const fileSegments = segments.slice(3);
  if (!owner || !repo || !branch || fileSegments.length === 0) {
    return "";
  }

  return buildURL("raw.githubusercontent.com", `/${owner}/${repo}/${branch}/${fileSegments.join("/")}`);
}

export function sourceUrlCdnSwitch(value: string): SourceUrlCdnSwitch | null {
  const cdnUrl = githubRawToJsDelivrUrl(value);
  if (cdnUrl) {
    return { label: "切到 jsDelivr CDN", nextUrl: cdnUrl };
  }

  const rawUrl = jsDelivrToGithubRawUrl(value);
  if (rawUrl) {
    return { label: "切回 GitHub Raw", nextUrl: rawUrl };
  }

  return null;
}
