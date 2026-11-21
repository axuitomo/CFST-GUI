#!/usr/bin/env node
// 前后端 DNS 平台 kind 集合守卫。
// 一个平台在三处出现：Go 常量表、前端 DNSProviderKind 联合类型、前端 DNS 平台 registry。
// 漏改一处的表现是「UI 少一个平台」或「配置里的平台被后端丢弃」，都不该等用户发现。
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT_DIR = join(dirname(fileURLToPath(import.meta.url)), "..", "..");

const SOURCES = {
  go: "internal/appcore/dns_providers.go",
  registry: "frontend/src/lib/dnsProviderRegistry.ts",
  union: "frontend/src/lib/bridge/types/enums.ts",
};

function read(relativePath) {
  return readFileSync(join(ROOT_DIR, relativePath), "utf8");
}

function quoted(block) {
  return [...block.matchAll(/"([^"]+)"/g)].map((match) => match[1]);
}

/** Go 侧 kind 常量：DNSProviderXxx = "kind"。 */
export function goProviderKinds(source) {
  return [...source.matchAll(/DNSProvider[A-Za-z]*\s*=\s*"([a-z0-9-]+)"/g)].map((match) => match[1]);
}

/** 前端联合类型：export type DNSProviderKind = "a" | "b"。 */
export function unionProviderKinds(source) {
  const declaration = /export type DNSProviderKind\s*=\s*([^;]+);/.exec(source);
  return declaration ? quoted(declaration[1]) : [];
}

/** 前端注册表：DNS_PROVIDER_KINDS: DNSProviderKind[] = ["a", "b"]。 */
export function registryProviderKinds(source) {
  const declaration = /DNS_PROVIDER_KINDS:\s*DNSProviderKind\[\]\s*=\s*\[([^\]]*)\]/.exec(source);
  return declaration ? quoted(declaration[1]) : [];
}

function describe(kinds) {
  return kinds.length > 0 ? kinds.join(", ") : "<空>";
}

function compare(label, leftName, left, rightName, right) {
  const missing = left.filter((kind) => !right.includes(kind));
  const extra = right.filter((kind) => !left.includes(kind));
  if (missing.length === 0 && extra.length === 0) {
    return true;
  }
  console.error(`dns-provider-kinds 不一致：${leftName} vs ${rightName}（${label}）`);
  console.error(`  ${leftName}: ${describe(left)}`);
  console.error(`  ${rightName}: ${describe(right)}`);
  if (missing.length > 0) {
    console.error(`  ${rightName} 缺少: ${missing.join(", ")}`);
  }
  if (extra.length > 0) {
    console.error(`  ${rightName} 多出: ${extra.join(", ")}`);
  }
  return false;
}

function main() {
  const goKinds = goProviderKinds(read(SOURCES.go));
  const registryKinds = registryProviderKinds(read(SOURCES.registry));
  const unionKinds = unionProviderKinds(read(SOURCES.union));

  // 解析失败等于守卫失效，必须报错而不是静默通过。
  const parseFailures = [];
  if (goKinds.length === 0) parseFailures.push(`${SOURCES.go}: 没有解析到 DNSProvider* 常量`);
  if (registryKinds.length === 0) parseFailures.push(`${SOURCES.registry}: 没有解析到 DNS_PROVIDER_KINDS`);
  if (unionKinds.length === 0) parseFailures.push(`${SOURCES.union}: 没有解析到 DNSProviderKind 联合类型`);
  if (parseFailures.length > 0) {
    console.error("dns-provider-kinds 守卫无法解析：");
    for (const failure of parseFailures) console.error(`  - ${failure}`);
    return 1;
  }

  let ok = true;
  ok = compare("集合", "Go", goKinds, "registry", registryKinds) && ok;
  ok = compare("集合", "registry", registryKinds, "DNSProviderKind", unionKinds) && ok;

  // 顺序即 UI 里「添加平台」按钮的顺序，必须两边一致。
  const orderMatches = goKinds.length === registryKinds.length && goKinds.every((kind, index) => kind === registryKinds[index]);
  if (!orderMatches) {
    console.error("dns-provider-kinds 顺序不一致：Go 表即 UI 顺序，registry 必须同序");
    console.error(`  Go:       ${describe(goKinds)}`);
    console.error(`  registry: ${describe(registryKinds)}`);
    ok = false;
  }

  if (!ok) {
    return 1;
  }
  console.log(`dns-provider-kinds ok: ${goKinds.length} platform(s) (${describe(goKinds)})`);
  return 0;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  process.exit(main());
}
