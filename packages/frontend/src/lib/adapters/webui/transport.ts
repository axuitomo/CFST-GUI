import { isObject, toStringValue } from "../../bridge/values";

// WEBUI_HEALTH_TIMEOUT_MS 是 /api/health 探测的上限：探测只在首个 WebUI 请求前发生，
// 但没有上限时，「接受连接却不回包」的代理或负载均衡会让它永久挂起，界面停在加载中且
// 没有任何提示（fetch 默认不带超时）。
const WEBUI_HEALTH_TIMEOUT_MS = 3000;
const WEBUI_TOKEN_STORAGE_KEY = "cfst-webui-token";

let webUIAuthRequiredPromise: Promise<boolean> | null = null;
let authRefreshedHandler: (() => Promise<void>) | null = null;

export function setWebUIAuthRefreshedHandler(handler: (() => Promise<void>) | null) {
  authRefreshedHandler = handler;
}

// webUIAuthRequired 判断「这个页面是不是 CFST WebUI 服务托管的，并且要求访问令牌」：只有
// /api/health 返回 JSON 且带服务自述字段 service=cfst-webui 才算。静态兜底页不能算：只看
// HTTP 状态会在普通静态托管或 dev 代理下给出错误的令牌结论。
function webUIAuthRequired() {
  if (!webUIAuthRequiredPromise) {
    webUIAuthRequiredPromise = fetch("/api/health", {
      cache: "no-store",
      signal: AbortSignal.timeout(WEBUI_HEALTH_TIMEOUT_MS),
    })
      .then(async (response) => {
        if (!response.ok || !response.headers.get("content-type")?.includes("application/json")) {
          return false;
        }
        const payload = await response.json();
        return isObject(payload) && payload.service === "cfst-webui" && Boolean(payload.auth_required);
      })
      .catch(() => false);
  }
  return webUIAuthRequiredPromise;
}

export async function ensureWebUIToken() {
  if (!(await webUIAuthRequired())) {
    return "";
  }
  let token = localStorage.getItem(WEBUI_TOKEN_STORAGE_KEY) || "";
  while (!token.trim()) {
    token = window.prompt("请输入 CFST WebUI 访问令牌") || "";
    if (!token.trim()) {
      throw new Error("缺少 WebUI 访问令牌。");
    }
  }
  localStorage.setItem(WEBUI_TOKEN_STORAGE_KEY, token.trim());
  return token.trim();
}

export async function webUIFetch(path: string, init: RequestInit = {}, retry = true) {
  const headers = new Headers(init.headers || {});
  if (!headers.has("Content-Type") && init.body) {
    headers.set("Content-Type", "application/json");
  }
  const token = await ensureWebUIToken();
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  const response = await fetch(path, {
    ...init,
    headers,
  });
  if (response.status === 401 && retry) {
    localStorage.removeItem(WEBUI_TOKEN_STORAGE_KEY);
    // 令牌被拒时重探一次：首次探测可能失败过，或服务端刚开启令牌要求。
    webUIAuthRequiredPromise = null;
    await ensureWebUIToken();
    if (authRefreshedHandler) {
      await authRefreshedHandler();
    }
    return webUIFetch(path, init, false);
  }
  if (!response.ok) {
    let message = `WebUI 请求失败 (${response.status})`;
    try {
      const body = await response.json();
      if (isObject(body) && body.message) {
        message = toStringValue(body.message);
      }
    } catch {
      // Keep the status-based message when the response is not JSON.
    }
    throw new Error(message);
  }
  return response;
}

export async function webUIJSON<T>(response: Response): Promise<T> {
  const contentType = response.headers.get("content-type") || "";
  if (!contentType.includes("application/json")) {
    throw new Error("WebUI 返回了非 JSON 响应，请检查 API 地址和服务版本。");
  }
  return (await response.json()) as T;
}

export async function webUIApp<T = unknown>(method: string, payload: Record<string, unknown> = {}) {
  const response = await webUIFetch(`/api/platform/${encodeURIComponent(method)}`, {
    body: JSON.stringify(payload),
    method: "POST",
  });
  return await webUIJSON<T>(response);
}

export function webUITokenQuery(token: string) {
  return token ? `?token=${encodeURIComponent(token)}` : "";
}
