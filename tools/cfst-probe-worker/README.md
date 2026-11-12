# CFST-GUI 公开探针（Cloudflare Worker）

一个可自建的「回声机」：客户端把 TCP 目的地钉在候选 IP、把 TLS SNI 与 HTTP Host 固定成本域名，请求就会落到候选 IP 所在的 Cloudflare 机房，Worker 把 `request.cf` 里的机房信息和你的出口身份回给对方。思路来自 [CFData-WEB](https://github.com/PoemMisty/CFData-WEB) 的公开探针，响应形状改成 CFST-GUI 能直接消费的两种格式。

## 它解决什么，不解决什么

先排除一个误解：**光识别 COLO 不需要这个 Worker**。Cloudflare 对任意前置域名都自己回答 `/cdn-cgi/trace`，所以把「追踪 URL」指向本域名的 `/cdn-cgi/trace` 就能拿到 `colo=XXX`，哪怕 `worker.js` 一行都没部署（实测把 `https://probe.609909.xyz/cdn-cgi/trace` 换成任意 CF 站点同样有效）。

它真正多给的是三件事：

1. **身份可校验**：响应带 `probe=cfst` 标记，任何 Cloudflare 前置站点都能冒充 `/cdn-cgi/trace`，但冒充不了这个标记，可作为链路复验类逻辑的判据。
2. **出口与归属信息**：一次响应给出出口 IP、ASN、AS 组织、国家、地区、城市、经纬度；Cloudflare 自带的 trace 没有 ASN 与城市。
3. **可用的追踪端点**：追踪阶段不必再依赖 `speed.cloudflare.com` 是否可达。

## 端点

| 路径 | 方法 | 用途 |
| --- | --- | --- |
| `/trace` | GET / HEAD | 纯文本 `key=value`，含可被 CFST-GUI 提取的 `colo=` 行 |
| `/`、`/json` | GET / HEAD | 结构化 JSON，带 `probe` 标记 |
| 其它路径 | 任意 | 404 |

`/trace` 响应样例：

```text
probe=cfst
version=1
ip=110.252.12.126
ip_type=ipv4
colo=LAX
loc=38.0414,114.4786
country=CN
region=Hebei
city=Shijiazhuang
continent=AS
asn=4837
as_org=China Unicom Hebei province network
http=HTTP/3
tls=TLSv1.3
ts=2026-11-01T05:07:36.000Z
```

`/` 响应样例：

```json
{"probe":"cfst","version":1,"ip":"110.252.12.126","ipType":"ipv4","colo":"LAX",
 "country":"CN","region":"Hebei","regionCode":"HE","city":"Shijiazhuang","continent":"AS","asn":4837,
 "asOrganization":"China Unicom Hebei province network","loc":"38.0414,114.4786",
 "httpProtocol":"HTTP/3","tlsVersion":"TLSv1.3","ts":"2026-11-01T05:07:36.000Z"}
```

两个格式契约不要改：

- `colo=` 必须自成一行的三个字符，CFST-GUI 用 `(?im)^colo=([a-z0-9]{3})\s*$` 提取；`worker.test.mjs` 里有对应断言。
- 所有响应必须保留 `Cache-Control: no-store`，否则边缘缓存会把 A 机房的 colo 发给 B 候选，导致地区码错乱。

## 部署

```bash
cd tools/cfst-probe-worker
npx wrangler login
npx wrangler deploy
```

部署后先用 `https://cfst-probe.<你的账号>.workers.dev` 验证，再在 Cloudflare 控制台给 Worker 绑一个专用自定义域（如 `probe.example.com`），或取消 `wrangler.toml` 里 `routes` 的注释改用路由模式。

验证：

```bash
curl -s https://probe.example.com/trace
curl -s https://probe.example.com/ | head -c 200
```

## 接进 CFST-GUI

1. 打开「设置 → 探测策略 → 第二阶段」。
2. 「追踪 URL（可选）」填 `https://probe.example.com/trace`。
3. 「第二阶段 COLO 获取模式」选「追踪URL」。
4. 保存后跑一次任务，结果的地区码来自 `/trace` 的 `colo=` 行。

CLI 模式注意：CLI 从 `-url` 派生 `/cdn-cgi/trace`，没有单独的追踪 URL 参数；不过同一域名的 `/cdn-cgi/trace` 由 Cloudflare 直接回答，所以把 `-url` 指向本域名同样能拿到 colo，只是用不上 JSON 里的出口信息。

## 把第二阶段整体换成探针

可以，且不需要改代码。阶段 2 的拨号目标永远钉在候选 IP 上（`internal/task/head.go` 的 `traceProbeIP` 用 `DirectDialContext` 覆盖目的地），URL 只决定 SNI / Host 和路径。所以把追踪 URL 指向本例的 `/trace`，阶段 2 发出的就是「目的地=候选 IP、SNI/Host=探针域名、路径=/trace」的请求，和 CFData-WEB 的 relink 等价：

| 配置组合 | 阶段 2 实际行为 |
| --- | --- |
| `trace_colo_mode=standard`，`trace_url` 留空（默认） | 请求 `<候选IP>/cdn-cgi/trace`，Cloudflare 自己回答，不消耗 Worker 额度 |
| `standard` + `trace_url=https://探针域名/trace` | 先走 IP 直连 trace，失败才回退到探针 |
| `trace_colo_mode=trace_url` + `trace_url=https://探针域名/trace` | 只用探针，`colo` 来自 `/trace` 的 `colo=` 行 |

换成探针前必须知道：

- 只能填 https。追踪请求不跟随跳转（`CheckRedirect` 返回 `ErrUseLastResponse`），http→https 的 301 会被判 `status_mismatch` 而淘汰候选。
- 每次候选探测消耗 1 次 Worker 请求，失败重试会翻倍；免费额度 10 万/日。
- `trace_url` 模式下没有 IP 直连 trace 兜底：探针挂了、被限流了，阶段 2 会全量淘汰，结果为空。要稳妥就用 `standard` 模式把探针当兜底。
- 探针域名不能开「Cache Everything」之类的缓存规则，否则 colo 会串。
- 如果要给探针加限流，阈值必须高于单次扫描的请求速率（同一出口 IP 会在几十秒内打几千次），否则会把自己限死。
- CFST 目前只从响应体里取 `colo=`，`probe=`、`asn`、`city` 等字段全部忽略，所以换成探针得到的 colo 与用 `/cdn-cgi/trace` 一致；想真正利用探针身份（拒绝冒充的 trace 响应），需要在客户端侧校验 `probe=cfst`，属于 Go 改动，不在本工具范围内。

## 验证探针确实「按 IP 落机房」

SNI 钉死测试：域名不变、目的地换成指定 Cloudflare IP，返回的 `colo` 应该是那个 IP 所在机房，而不是你本地最近的机房。

```bash
# 104.16.0.1 的机房
curl -s --resolve probe.example.com:443:104.16.0.1 https://probe.example.com/trace | grep -E '^(colo|ip)='
# 与响应头 cf-ray 末尾三位核对
curl -sI --resolve probe.example.com:443:104.16.0.1 https://probe.example.com/trace | grep -i cf-ray
```

同一请求两次结果应当一致；若 `/trace` 的 `colo` 与 `cf-ray` 尾码不符，先检查是否被缓存规则或第三方 CDN 套了一层。

## 自检

```bash
node --test worker.test.mjs
```

覆盖：`colo=` 正则契约、JSON 字段、IPv6 出口、CF 字段缺失时的降级、HEAD、404/405/OPTIONS。无需安装依赖。

## 额度与安全

- 免费额度 10 万请求/日，每次追踪探测消耗 1 次；大任务前估算一下。
- Worker 只回显 `request.cf` 与 `CF-Connecting-IP`，不读 Cookie、不写日志、不落盘。
- 公开端点建议加 Cloudflare Rate Limiting 或 WAF 规则，但阈值要高于单次扫描速率（见上一节），并避免在该域名上挂别的业务（探针结论依赖「这个域名只做探针」）。
- 需要更强身份校验时改 `PROBE_MARKER`，并让客户端校验对应值。

## 与 CFData-WEB 探针的差异

| | CFData-WEB | 本 Worker |
| --- | --- | --- |
| 标记 | `probe:"cfdata"` | `probe=cfst` / `"probe":"cfst"` |
| 文本端点 | 无 | `/trace`，可直接作为 CFST 追踪 URL |
| 提供字段 | ip、ipType、colo、asn、asOrganization、country、city、loc | 同上，另加 region、regionCode、continent、httpProtocol、tlsVersion、ts |
| 缓存 | 未声明 | 强制 `no-store` |
| 用途 | 非标结果链路复验 | COLO 追踪 + 出口审计 + 可校验身份 |
