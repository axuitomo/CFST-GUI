// 配置归一化的默认值与上下限常量。
export const MIN_PROBE_PING_TIMES = 2;
export const DEFAULT_MAX_LOSS_RATE = 0.15;
export const MAX_LOSS_RATE = 1;
export const DEFAULT_HTTPING_STATUS_CODE = 200;
export const DEFAULT_FILE_TEST_URL = "https://speed.cloudflare.com/__down?bytes=99999999";
// 历史版本内置的默认测速地址：命中时按未配置处理，跟随当前默认值。
export const LEGACY_FILE_TEST_URLS = new Set(["https://speedtest.xyz9923.dpdns.org/500m", "https://speed.cloudflare.com/__down?bytes=25000000"]);
export const DEFAULT_DOH_URL = "https://dns.alidns.com/dns-query";
export const DEFAULT_ECH_DOMAIN = "cloudflare-ech.com";
export const DEFAULT_DNSHE_TTL = 600;
export const DEFAULT_ALIYUN_TTL = 600;
export const DEFAULT_DNSPOD_TTL = 600;
export const DEFAULT_CLOUDFLARE_UPLOAD_TOP_N = 5;
export const DEFAULT_GITHUB_UPLOAD_TOP_N = 20;
export const DEFAULT_SOURCE_POOL_LIMIT = 65536;
export const MAX_SOURCE_POOL_LIMIT = 1000000;
export const DEFAULT_CLOUDFLARE_TTL = 300;
export const DEFAULT_UTC_OFFSET_MINUTES = 8 * 60;
export const DEFAULT_TELEGRAM_NOTIFICATION_TOP_N = 5;
export const MAX_TELEGRAM_NOTIFICATION_TOP_N = 50;
