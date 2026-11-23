interface SourceNameRule {
  name: string;
  tokens: string[];
}

// 名称取自 https://bestcf.pages.dev/ 的「优选IP BestIP」与「优选网站」清单。
// 数组顺序即优先级：先匹配更具体的链接特征，通用短 token 放最后，避免抢占更精确的规则。
// 同一链接只留一条规则（例如 stock.hostmonit.com 归 HostMonit，不再叫 CFYes）。
const SOURCE_NAME_RULES: SourceNameRule[] = [
  { name: "CM API", tokens: ["addressesapi.090227"] },
  { name: "CM 3", tokens: ["workervless2sub"] },
  { name: "CM 1", tokens: ["/cmliu/"] },
  { name: "CM", tokens: ["090227"] },
  { name: "天诚 4", tokens: ["gshtwy"] },
  { name: "WeTest", tokens: ["wetest"] },
  { name: "UOUIN", tokens: ["uouin"] },
  { name: "Mia", tokens: ["xinyitang"] },
  { name: "LZJ", tokens: ["/lzj/"] },
  { name: "CFYes", tokens: ["cfyes"] },
  { name: "HostMonit", tokens: ["hostmonit"] },
  { name: "天诚 5", tokens: ["soso.sryze"] },
  { name: "天诚 3", tokens: ["tiancheng3"] },
  { name: "天诚 2", tokens: ["tiancheng2"] },
  { name: "天诚 1", tokens: ["tiancheng"] },
  { name: "Gslege", tokens: ["gslege"] },
  { name: "MJZ", tokens: ["junzhen"] },
  { name: "SS", tokens: ["/svip-s/"] },
  { name: "LZ", tokens: ["love-ztm"] },
  { name: "ZhiXuan", tokens: ["zhixuanwang", "zhixuan"] },
  { name: "vvHan", tokens: ["vvhan"] },
  { name: "MingYu", tokens: ["ymyuuu", "mingyu"] },
  { name: "Yuanxiawan", tokens: ["yuanxiawan"] },
  { name: "LancelotRar", tokens: ["lancelotrar"] },
  { name: "Joname", tokens: ["joname"] },
  { name: "Ahang", tokens: ["ahang"] },
  { name: "Einsitang", tokens: ["einsitang"] },
  { name: "Hubbylei", tokens: ["hubbylei"] },
  { name: "Senflare", tokens: ["senflare"] },
  { name: "JieChaoCC", tokens: ["jiechaocc"] },
  { name: "IRCF", tokens: ["ircf"] },
  { name: "NiREvil", tokens: ["nirevil"] },
  { name: "LuoLi", tokens: ["luoli"] },
  { name: "Kristi", tokens: ["kristi"] },
  { name: "MoistR", tokens: ["moistr"] },
  { name: "YuTian", tokens: ["yutian"] },
  { name: "Fiatnorm", tokens: ["fiatnorm"] },
  { name: "Endpoint IP", tokens: ["/entryip/"] },
  { name: "VPS789", tokens: ["vps789"] },
  { name: "IPDB", tokens: ["ipdb.api", "030101"] },
  { name: "NB", tokens: ["byoip"] },
  { name: "HandsomeMJZ", tokens: ["handsomemjz"] },
  { name: "JZ", tokens: ["jz"] },
];

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

function decodeUrlPart(value: string) {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

function normalizedUrlParts(value: string) {
  const parsed = parseSourceUrl(value);
  const raw = value.trim().toLowerCase();
  if (!parsed) {
    return {
      haystack: `${raw} ${raw.replace(/[^a-z0-9]+/g, "")}`,
      segments: raw.split(/[^a-z0-9]+/).filter(Boolean),
    };
  }

  const hostname = parsed.hostname.toLowerCase();
  const pathname = decodeUrlPart(parsed.pathname || "").toLowerCase();
  const segments = [...hostname.split("."), ...pathname.split(/[^a-z0-9]+/)].filter(Boolean);

  return {
    haystack: `${hostname}${pathname} ${`${hostname}${pathname}`.replace(/[^a-z0-9]+/g, "")}`.toLowerCase(),
    segments,
  };
}

function titleFromToken(value: string) {
  const cleaned = value
    .replace(/\.[a-z0-9]{1,8}$/i, "")
    .replace(/[_-]+/g, " ")
    .trim();
  if (!cleaned) {
    return "";
  }
  return cleaned
    .split(/\s+/)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function genericSourceName(value: string) {
  const parsed = parseSourceUrl(value);
  if (!parsed) {
    return titleFromToken(value);
  }

  const pathSegments = decodeUrlPart(parsed.pathname || "")
    .split("/")
    .map((part) => part.trim())
    .filter(Boolean);
  // 跳过排序与格式类片段，让 /ircf/ipv4.txt 这类镜像链接取到来源名而不是“Ipv4”。
  const usefulPathSegment = [...pathSegments].reverse().find((part) => !/^(?:top\d*|ipv[46]|all|mini|best|ips?|txt|csv|list)(?:\.[a-z0-9]{1,8})?$/i.test(part));
  const fromPath = usefulPathSegment ? titleFromToken(usefulPathSegment) : "";
  if (fromPath) {
    return fromPath;
  }

  const hostParts = parsed.hostname.split(".").filter(Boolean);
  const hostRoot = hostParts.length > 2 ? hostParts[hostParts.length - 3] : hostParts[0];
  return titleFromToken(hostRoot || parsed.hostname);
}

export function detectSourceNameFromUrl(value: string) {
  const parts = normalizedUrlParts(value);
  if (!parts.haystack) {
    return "";
  }

  for (const rule of SOURCE_NAME_RULES) {
    if (rule.tokens.some((token) => parts.segments.includes(token) || parts.haystack.includes(token))) {
      return rule.name;
    }
  }

  return genericSourceName(value);
}

export function isDefaultSourceName(value: string) {
  return value.trim() === "" || /^输入源\s*\d+$/.test(value.trim());
}
