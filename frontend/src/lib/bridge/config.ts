// 配置归一化门面：实现按域拆到 `config/*`，对外导出集合与 `./bridge/config` 导入路径保持不变。
export { DEFAULT_DOH_URL, DEFAULT_ECH_DOMAIN, DEFAULT_FILE_TEST_URL, DEFAULT_HTTPING_STATUS_CODE, DEFAULT_MAX_LOSS_RATE, DEFAULT_SOURCE_POOL_LIMIT, DEFAULT_ALIYUN_TTL, DEFAULT_DNSHE_TTL, DEFAULT_DNSPOD_TTL, MAX_LOSS_RATE, MAX_SOURCE_POOL_LIMIT, MIN_PROBE_PING_TIMES } from "./config/defaults";
export { isMaskedTokenValue, normalizeColoFilterMode, normalizeSourceColoFilterPhase, normalizeTraceColoMode } from "./config/scalars";
export { normalizeSourceConfig, normalizeSourceProfileStore } from "./config/sources";
export { normalizeConfigSnapshot, normalizeSourceProfileUpdatePayload } from "./config/snapshot";
