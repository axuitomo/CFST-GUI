/**
 * CFST-GUI 公开探针（Cloudflare Worker）
 *
 * 用途：把「连哪个 IP」和「用哪个域名」分开。客户端把 TCP 目的地钉在候选 IP、
 * 把 TLS SNI / HTTP Host 固定成这个 Worker 的域名，请求就会落到候选 IP 所在的
 * Cloudflare 机房，Worker 再把 request.cf 里的机房与出口信息回给对方。
 *
 * 端点：
 *   GET /trace   纯文本 key=value，含 colo=XXX，可直接填进 CFST-GUI 的「追踪 URL」
 *   GET /        等价信息的 JSON，带 probe 标记，供链路复验类客户端做身份校验
 *
 * 注意：/trace 的 colo 行必须保持「单独一行、恰好三个字符」，CFST-GUI 用
 * (?im)^colo=([a-z0-9]{3})\s*$ 提取；写成别的形状就只能靠 cf-ray 头兜底。
 */

const PROBE_MARKER = "cfst";
const PROBE_VERSION = 1;

const JSON_HEADERS = {
  "content-type": "application/json; charset=utf-8",
  "cache-control": "no-store",
  "access-control-allow-origin": "*",
};

const TEXT_HEADERS = {
  "content-type": "text/plain; charset=utf-8",
  "cache-control": "no-store",
  "access-control-allow-origin": "*",
};

export default {
  async fetch(request) {
    const { pathname } = new URL(request.url);
    if (pathname !== "/" && pathname !== "/trace" && pathname !== "/json") {
      return new Response("not found\n", { status: 404, headers: TEXT_HEADERS });
    }
    if (request.method === "OPTIONS") {
      return new Response(null, { status: 204, headers: JSON_HEADERS });
    }
    if (request.method !== "GET" && request.method !== "HEAD") {
      return new Response("method not allowed\n", { status: 405, headers: TEXT_HEADERS });
    }

    const data = probeData(request);
    const body = pathname === "/trace" ? renderTrace(data) : JSON.stringify(data) + "\n";
    const headers = pathname === "/trace" ? TEXT_HEADERS : JSON_HEADERS;
    return new Response(request.method === "HEAD" ? null : body, { headers });
  },
};

export function probeData(request) {
  const cf = request.cf || {};
  const ip = request.headers.get("CF-Connecting-IP") || "";
  return {
    probe: PROBE_MARKER,
    version: PROBE_VERSION,
    ip,
    ipType: ipKind(ip),
    colo: String(cf.colo || "").toUpperCase(),
    country: cf.country || "",
    region: cf.region || "",
    regionCode: cf.regionCode || "",
    city: cf.city || "",
    continent: cf.continent || "",
    asn: Number(cf.asn) || 0,
    asOrganization: cf.asOrganization || "",
    loc: location(cf.latitude, cf.longitude),
    httpProtocol: cf.httpProtocol || "",
    tlsVersion: cf.tlsVersion || "",
    ts: new Date().toISOString(),
  };
}

export function renderTrace(data) {
  const lines = [
    `probe=${data.probe}`,
    `version=${data.version}`,
    `ip=${data.ip}`,
    `ip_type=${data.ipType}`,
  ];
  if (data.colo) {
    lines.push(`colo=${data.colo}`);
  }
  if (data.loc) {
    lines.push(`loc=${data.loc}`);
  }
  if (data.country) {
    lines.push(`country=${data.country}`);
  }
  if (data.region) {
    lines.push(`region=${data.region}`);
  }
  if (data.city) {
    lines.push(`city=${data.city}`);
  }
  if (data.continent) {
    lines.push(`continent=${data.continent}`);
  }
  if (data.asn) {
    lines.push(`asn=${data.asn}`);
  }
  if (data.asOrganization) {
    lines.push(`as_org=${data.asOrganization}`);
  }
  if (data.httpProtocol) {
    lines.push(`http=${data.httpProtocol}`);
  }
  if (data.tlsVersion) {
    lines.push(`tls=${data.tlsVersion}`);
  }
  lines.push(`ts=${data.ts}`);
  return lines.join("\n") + "\n";
}

function ipKind(ip) {
  if (!ip) {
    return "";
  }
  return ip.includes(":") ? "ipv6" : "ipv4";
}

// Cloudflare 把 latitude/longitude 给成字符串（"30.27130"），缺失时是 null。
function location(latitude, longitude) {
  const lat = toNumber(latitude);
  const lon = toNumber(longitude);
  if (Number.isNaN(lat) || Number.isNaN(lon)) {
    return "";
  }
  return `${lat.toFixed(4)},${lon.toFixed(4)}`;
}

function toNumber(value) {
  if (value === null || value === undefined || value === "") {
    return Number.NaN;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : Number.NaN;
}
