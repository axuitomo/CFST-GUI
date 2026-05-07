interface SourceNameRule {
  name: string;
  tokens: string[];
}

const SOURCE_NAME_RULES: SourceNameRule[] = [
  { name: "CM[090227.pages.dev]", tokens: ["090227.pages.dev", "090227"] },
  { name: "VPS789", tokens: ["vps789"] },
  { name: "Gslege", tokens: ["gslege"] },
  { name: "tiancheng", tokens: ["tiancheng"] },
  { name: "vvhan", tokens: ["vvhan"] },
  { name: "HandsomeMJZ", tokens: ["handsomemjz"] },
  { name: "JZ", tokens: ["jz"] },
  { name: "Xinyitang", tokens: ["xinyitang"] },
  { name: "NiREvil", tokens: ["nirevil"] },
  { name: "MingYu", tokens: ["mingyu"] },
  { name: "WeTest", tokens: ["wetest"] },
  { name: "ZhiXuan", tokens: ["zhixuan"] },
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
  const segments = [
    ...hostname.split("."),
    ...pathname.split(/[^a-z0-9]+/),
  ].filter(Boolean);

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
  const usefulPathSegment = [...pathSegments].reverse().find((part) => !/^top\d*$/i.test(part));
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
