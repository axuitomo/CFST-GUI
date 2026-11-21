# 更新日志

本文件记录 CFST-GUI 的用户可见变更，格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循语义化版本。每个版本更完整的验证命令和发行资产见 [`docs/release-notes/`](docs/release-notes/README.md)。

## [Unreleased]

### 修复

- 修复第二阶段追踪在 UDP/443 被封或被代理截走的网络上几乎全部淘汰：追踪请求此前沿用 `auto`，先做一次 QUIC 握手，失败耗时可达 1.5 秒以上，吃满 `stage2_timeout_ms`（默认 1000 ms）后 TCP 回退已无预算，全部 IP 按 `context deadline exceeded` 超时淘汰；现在与来源抓取一致固定走 TCP，并新增用例锁定该行为。

## [2.1.4] - 2026-11-21

本版以工程结构收敛和质量门禁为主。原计划单独发布的 `2.1.3` 未打标签发版，其多平台 DNS 读取、分流推送与平台自检等变更随本版首次发布，完整内容见本文件 2.1.3 段。

### 新增

- 新增 `pnpm check:fast`（`scripts/checks/fast.ps1`，bash twin `fast.sh`）：按变更区域只跑受影响的 Go 包 `-short` 与前端检查（oxlint/vitest/vue-tsc 并行），无改动时回退为最小全量扫描；pre-commit、`check.ps1`/`check.sh`、CI 与 `pnpm` 均已接线。
- 新增 `pnpm check:stringcode`（`scripts/checks/no-string-code.mjs`）：词法级扫描前端 TS 与 `.vue` script 块、Go 生产代码和构建脚本，禁止 `innerHTML`/`outerHTML`/`cssText`/`eval`/`new Function`、raw string 含 HTML 与 `Sprintf` 拼 JSON 等字符串拼装写法；当前前端 154 个文件、Go 178 个文件、5 个构建脚本全部通过。
- Go 依赖方向用 depguard 固化为四层（入口/应用/领域/基础），补齐根入口与 `cmd` 的分层盲区，契约测试豁免显式化。

### 变更

- 前端 `App.vue` 按职责拆出主题、导航、视口、Toast/活动流、下载速率、Android 运行时与选择器、配置持久化、输入源与档案、自动保存、DNS 平台/记录/自检、结果工作区与导出、WebDAV、应用更新、存储健康、COLO 词典、调度、任务同步、启动同步和探测任务等 composables；`bridge` 类型与配置按域拆成子模块并保留门面导出，对外导出集合逐一比对一致。
- `probeEventApply`、`useResultExport`、`useStartupSync`、`useProbeTask` 按域二次拆分到 `lib/probeEventApply/`、`lib/resultExport/`、`lib/startup/` 与 launch/control/deps 子模块，并补齐 characterization 测试。
- 运行时代码不再拼接 HTML/CSS/JS/SQL/JSON：安装器 UI 拆为 `index.html` + `installer.css` + `installer.js`，以 `go:embed all:ui` 提供静态路由；WebUI bundle 的 Docker/Compose/README 模板外置到 `scripts/build/templates/webui-bundle/`；`sqliteprobe` 四条 SQL 外置到 `queries/*.sql` 并用 `go:embed` 注入；动画预览样式按 16 个分区拆分。
- `vue-tsc` 改为增量编译（稳态 7.7s → 5.8s）；Go 定时器与退避类测试加 `-short` 守卫，单包重负载测试 ≤3s。
- `docs/plan` 维护记录按年月归档并新增索引；Android WebView 性能文档迁入 `docs/mobile/performance/`；README 与命令文档同步 `scripts/checks/` 路径。

### 修复

- 修复 Android gomobile 桥接边界未捕获 panic 直接终止应用进程：命令处理的 panic 现在收回为 `COMMAND_PANIC` 失败信封并记录 `bridge.panic` 事件，前端可显示原因并保留任务现场。
- 修复 Kotlin `normalizeJSON` 对字符串内控制字符的处理：字符串内 0x20 以下字符转义为 `\uXXXX`，字符串外只保留 JSON 允许的空白，避免 Go `encoding/json` 拒绝整条响应，也避免错误信息里的换行被静默丢弃。
- `mobileapi` 的 JSON 编码失败兜底不再手工拼接 JSON 文本，全部经 `json.Marshal` 生成，避免引号或反斜杠漏转义。
- 修复 `fast.ps1` 在 WSL 桩环境下 `bash -n` 假失败：优先解析 Git Bash。
- 同步 DNS 多平台读取与自检的 e2e aria 基线，测试不再因可访问名称变化而失败。
- `LICENSE` 恢复为 GPL-3.0 逐字原文：移除文件头的项目名、版权行、SPDX 标识和多余空行。GitHub 等平台的许可证识别按整段文本比对，这些附加内容会让整份文件被判为“无法识别”；项目版权与 SPDX 标识保留在 `README.md`，第三方归属仍在 `THIRD-PARTY-NOTICES.md`。
- 新增 `pnpm check:license`（`scripts/build/check-canonical-license.mjs`）：按整文件哈希校验 `LICENSE` 未被追加注释、版权附加文字或第三方声明，也未合并其它协议；quality 工作流与法律文件测试同时守住这条约束。

### 版本与构建

- 默认版本统一为 `2.1.4`，Android `versionName` 为 `2.1.4`、`versionCode` 提升为 `20104`。
- 发布脚本、安装器默认版本与 Release/Container/Android 工作流的版本输入同步到 `2.1.4`。

## [2.1.3] - 2026-11-20

### 新增

- DNS 平台新增「允许修改 CNAME」开关（Cloudflare、DNSHE、阿里云 DNS、DNSPod 通用，默认关闭）。关闭时目标记录名存在 CNAME 会拒绝本次推送并返回 `DNS_CNAME_PROTECTED`，不会删除别名；开启后才按覆盖语义先删除 CNAME 再写入 A/AAAA。该开关随平台配置保存，自检会按当前值验证“拒绝并保留 CNAME”或“删除后替换”。
- 设置页新增 DNSHE「读取子域名」：可拉取账号下的子域名并在输入框中联想选择，显示完整域名与状态；「子域 ID（可选）」改为「子域名（可选）」，支持填 `test` 这类标签或完整域名，留空时仍按完整域名自动匹配。

### 变更

- DNSHE「子域名」填裸标签时的解释改为 `<标签>.<完整域名>`：账号下没有同名子域时（例如 `test` + `axuitomo.l.cd`）解析为 `test.axuitomo.l.cd` 并写入该名称，不再报「找不到子域名 test」；账号下存在同名子域或填写完整域名时仍按子域选择处理。推送、DNS 读取页与设置页「当前目标」预览使用同一套规则；DNSHE 平台自检固定落在主域名（子域 zone 顶点）下的 `cfst-selftest.<主域名>`，不跟随推送记录名。
- DNS 推送默认不再替换同名 CNAME：四个平台的既有配置升级后 `allow_cname` 均为 `false`，目标记录名存在 CNAME 时推送会失败而不是删除别名；需要替换请在设置页对应平台勾选「允许修改 CNAME」。
- DNS 分流设置对所有平台开放：Cloudflare、DNSHE、阿里云 DNS、DNSPod 都能在共享筛选后按规则拆分记录名、记录类型和规则 Top N；结果页按当前选中平台显示分流状态。
- DNS 平台备注名默认跟随平台名称：新增 Cloudflare / DNSHE / 阿里云 DNS / DNSPod 平台时自动填入对应平台名，读取旧配置时备注名为空的条目也回填平台名；仍可手动改成「主域名」「备用」等备注。
- DNS 页面新增「DNS 平台自检」：选择已启用的 Cloudflare / DNSHE / 阿里云 DNS / DNSPod 平台，在专用测试记录名（默认 `cfst-selftest.<推送记录名>`）上依次验证读取、上传 A/AAAA、幂等重推、换血、TTL 属性更新与 CNAME 拦截/替换（按平台「允许修改 CNAME」设置），跑完自动清理；不跑公网解析与测速。测试记录名必须带 `cfst-selftest` 前缀且不能等于推送记录名，TXT/MX 等非托管记录在终态按指纹复核。平台不允许创建 CNAME 时标记为跳过，不判整体失败。
- DNS 记录读取新增「读取平台」选择：可切换到设置页已保存的任一平台（Cloudflare、DNSHE、阿里云 DNS、DNSPod）读取记录，不再只能读 Cloudflare。未补全凭据或记录名的平台会给出配置错误提示，读取仍为只读、不写记录。
- 界面术语统一：输入组 IP 模式的「随机抽查」改为「MICS 抽样」，并同步到输入组摘要、任务阶段标签、Dashboard 抽样进度标题和入门文档。配置键与 `mcis` 模式值、后端行为均未变。
- 依赖与工具链同步到最新稳定版：前端 `@vitejs/plugin-vue` 6.0.11、Vitest 5.1.1、Playwright 1.65、oxlint 1.94、oxfmt 0.79、Stylelint 18（配 `stylelint-config-standard` 41）、Capacitor 8.6.2、motion-v 2.13；Go 侧 quic-go 0.65、modernc sqlite 1.62、`golang.org/x/*` 系列；Android 升到 AGP 9.5.1、Gradle 9.9.1、顶层 KGP 2.4.21。Vue 3.6 与 Wails 3.2 只有 RC/夜版，没有可切的稳定版，保持原有钉住版本。
- 「实时测速详情」在并发测速 IP 数大于 1 时改为每个在用 IP 一行，展示 IP、COLO、实时速率与平均速率；采样停止后自动移除该行。单并发仍保留原来的四格卡片布局。并发数、采样间隔、是否启用表格与行 TTL 都按任务启动时的配置快照固定，运行中修改设置不会改变当前任务的展示口径。
- 极速策略会跳过下载测速，此时不再显示空的并发表；实时测试进程按 IP 各保留一条测速日志，测速表改为语义化 `table`，读屏软件可以把速率与表头关联起来。

### 修复

- 修复 DNS 读取页在 DNSHE 平台选择「当前域名全部记录」却只返回子域顶点的问题：空记录名现在按整个子域返回全部记录，配置的目标记录名与裸子域标签仍按解析后的记录名过滤。
- 修复 DNSHE 只填子域名、不填记录名时自检在预检阶段报「没有填写推送记录名」的问题：自检入口先按 `subdomain > subdomain_id > record_name` 解析完整域名作为测试基准，DNSHE 适配器也按同一优先级选择子域，避免测到与推送不同的目标。
- DNSHE 客户端遇到 429 会按 `Retry-After` 或指数退避重试（最多 3 次）；自检客户端固定 2 秒请求间隔并把子域解析结果缓存到阶段内，避免单次自检超过 DNSHE 默认 30 请求/分钟而中途失败或残留测试记录。
- DNS 平台自检增加进程内单飞锁，重复执行返回 `DNS_SELFTEST_BUSY`，避免两个窗口同时清理同一测试记录名；`wrote_records` 只在本次实际创建或更新记录后为真，不再把清理旧残留计为“已写入”。
- 修复 DNSHE 主域名记录识别：`dnspush.AbsoluteName` 在记录名恰好等于子域名的顶点场景会把完整域名再拼一次（`axuitomo.l.cd.axuitomo.l.cd`），导致 DNS 页按配置名过滤后主域名记录消失。顶点现在原样返回，相对记录名按 `@` 处理。
- 修复 DNSHE 分流推送在只填子域名、不填记录名时跳过主目标的问题；分流目标可覆盖记录名，但必须属于所选子域名，跨子域写入会被拒绝。
- 前端配置归一化把 `probe.concurrency.stage3` 固定写回 1，导致设置页保存或重新加载后「并发测速 IP 数」总是变回 1、实际任务始终串行下载。现在按保存值归一化，并保持 1-16 的既有范围。此前被自动写回 1 的配置需要重新设置一次。
- 安卓端的 200ms 进度节流此前会连 `probe.speed` 一起拦掉：同一秒内并发采样的多颗 IP 只有第一颗能进 UI，实时测速表看起来仍只有一行。现在节流只限制状态文案与进程日志，每个 IP 的实时行逐采样更新。
- DNS 推送的 Cloudflare 入口在缺少探测结果、只带 `ipsRaw` 时会绕过记录类型过滤直接写 A/AAAA。现在该入口直接返回 `DNS_INPUT_EMPTY` 且不发起任何请求；core 仍按输入 IP 池管理 A 与 AAAA，类型筛选由推送调用方负责，并已用注释与测试固定这一契约。

## [2.1.2] - 2026-11-18

### 变更

- Windows 安装器从 NSIS 改为自绘的 Go 安装器（Wails v3 + WebView2）：560×380 无边框窗口、深色单色版式、等宽字体展示路径与版本号，安装进度在同一窗口内以日志控制台 + 2px 进度条就地切换。安装与卸载界面均为简体中文。安装器不再依赖 NSIS 工具链，发布脚本改为编译 `cmd/installer` 并签名。
- 安装器保留 `/S`、`/D=` 静默参数与 winget/choco 批量部署能力，新增 `--no-desktop`、`--no-launch`；卸载器改为安装器自身副本，卸载界面与安装器一致。
- Windows 发布签名默认允许自签证书：本地缺少证书时生成临时自签 PFX，CI 在缺少签名 Secret 时按 `windows_self_signed` 输入生成临时证书；`SignTool` 警告只记录日志、不再中断构建。设置 `CFST_WINDOWS_ALLOW_SELF_SIGNED=0` 可恢复强制受信任证书链。
- 许可明确为 GPL-3.0-only 并补齐版权声明：新增 `THIRD-PARTY-NOTICES.md` 与可复现的第三方清单生成、校验脚本；前端产物、安装目录、Linux/macOS 发行包、GitHub Release 与 APK 均携带 `LICENSE` 与第三方声明，Android Release 另校验签名身份与法律资产。
- 安装器新增“查看许可证”入口：安装前即可在窗内阅读 GPL-3.0 原文与第三方声明，文本直接读取安装包内嵌的 `LICENSE` / `THIRD-PARTY-NOTICES.md`（只读 `/api/legal` 接口）。
- 安装成功后安装器自动关闭：完成日志与进度条停留约 1 秒后退出；失败或取消仍保留窗口供排查。
- 安装器界面、状态栏与安装日志全部为简体中文；安装或卸载前检测目标目录里的 `cfst-gui.exe` 是否正在运行，界面先询问是否关闭应用并继续，确认后再结束进程，静默安装则提示先退出应用。

### 修复

- 修复选择旧 NSIS 安装目录时被误判为“非空且非已有安装”的问题：卸载项缺少 `InstallLocation` 时从 `UninstallString` / `DisplayIcon` 还原目录，并识别 `icon.ico`、`LICENSE` 与 `THIRD-PARTY-NOTICES.md` 等应用自有文件。
- 修复安装器可卸载任意目录、升级中断损坏已有程序等风险：校验注册安装位置，仅删除自有文件，使用原子替换和失败回滚，并在安装期间安全取消。
- 修复诊断 ZIP 泄露通知密码、Webhook 密钥与鉴权信息；导出副本脱敏，不改写持久化配置。
- WebUI 写入统一校验同源请求与 JSON Content-Type，无令牌 API 拒绝非回环 Host；修复首次事件订阅失败后的启动恢复。
- 修复 Linux 构建误包含 Windows 安装器；恢复发布脚本 ShellCheck，并将 Windows 安装版与便携版签名验收改为哈希、时间戳和签名身份一致性校验。
- 修复邮件通知忽略 TLS 配置与取消信号的问题，增加发送总超时；修复 DNS 替换 CNAME 失败时残留地址记录，并在响应丢失后重新读取服务端状态进行回滚。
- Android 更新元数据仅接受官方 HTTPS 来源及受控重定向，限制读取时长与大小；APK 必须通过可信 SHA256 校验，缺少 manifest 时兼容官方资产 digest，避免选择不兼容架构。
- 修复新 Windows 安装器在含空格路径下卸载后留下 `uninstall.exe` 的问题；兼容清理旧 NSIS 版在公共桌面、公共开始菜单创建的快捷方式。
- 回退 2.1.1 引入的「启动同步链让到首帧之后」。它把 `loadConfig` 推到首帧之后，启动同步因此可能在用户已经点过界面之后才落地，并覆盖刚选中的 COLO 过滤模式（`tests/sources-workflow.spec.ts` 的「applies COLO filtering from the input group to source previews」抓到的就是竞态）。冷启动收益保留首包裁剪与启动屏淡出两处，这一处让位撤销。

## [2.1.1] - 2026-11-15

### 优化

- 冷启动收尾更快：升级到 Vue 3.6.0-rc.15 并改用裸 `vue` 的默认入口（该版本起 `runtime.esm-bundler` 同时导出 VDOM 与 Vapor 运行时），打包器可以按实际引用裁剪，首屏 `app.js` 从 502 KB 降到 486 KB（gzip 159 → 153 KB）。同时把启动画面淡出从 0.18s/240ms 压到 0.12s/160ms，并让启动同步链等首帧画完再跑：慢机上用户更早看到用缓存回填的看板，而不是在骨架屏后面等配置与任务快照读完。
- 文件测速的「最低下载速度」现在同时是提前收工线：预热结束再留两个采样窗口后，累计峰值仍低于下限的 IP 会立刻结束测量，不再跑满整个测量窗口。这类 IP 本来就会被下限淘汰，结果不受影响；下限为 0（默认）时行为不变。
- 阶段 1 的 TCP 采样改为并发发出（`internal/task/tcping.go`）。此前每颗 IP 要串行做完跳首包 + `ping_times` 次建连，一次真实跑法里 13863 颗候选有 11948 颗全超时，光这一阶段就花了 122 秒；并发后每颗 IP 的墙钟成本从「N × 超时」回到「1 × 超时」，同一跑法的阶段 1 预期压到 30~45 秒。丢包率与平均延迟的口径不变（仍是 `ping_times` 个独立样本）；代价是同时在飞连接数从 `routines` 变成 `routines × ping_times`，暂停与终止只在一批采样的边界生效。
- 追踪阶段并发上限从 30 提升到 100（`MaxTraceRoutines`）。同一跑法里 1915 颗候选在 30 并发下被 2 秒超时拖成 100 秒，抬到 100 后预期约 30 秒；每颗 IP 打的是不同边缘节点，429 仍按既有退避处理。已在配置里显式写过阶段 2 并发的用户保持自己的值，不受此改动影响。
- 连续失败冷却不再把「候选本身不通」算进去（`internal/task/retry.go`）。此前每颗超时的死 IP 都推进一次连续失败计数，实测一次 13863 颗候选的跑法触发了 1051 次 2500ms 冷却，而每次只让一个 worker 停一下、另外 199 个照常探测，等于纯损失约 11% 的阶段 1 墙钟时间。现在只有耗时不到该阶段超时预算 1/10 的失败（路由不可达、端口耗尽这类链路异常）和 worker 崩溃才计入冷却；文件测速阶段的失败全部按候选侧处理。`cooldown_policy` 的字段与默认值不变，配置为 0 仍然等于关闭。

### 修复

- Android 设置页点开输入框时页面像卡死：软键盘以 `adjustResize` 压缩窗口后，固定标题栏与底部导航占满整条视口，横屏下内容区整片空白、竖屏下聚焦的字段被标题栏盖住。键盘态此前用 `innerHeight - visualViewport.height` 判定，而这两个高度随窗口同步缩小、差值恒为 0，标志位永远停在「关闭」；现在改为与「无键盘时的视口高度」基线比较（基线只在焦点不在文本控件时刷新，`frontend/src/lib/androidViewport.ts`），键盘打开时收起标题栏与底部导航，并把 `.mobile-main` 为标题栏预留的 `padding-top` 归零。

## [2.1.0] - 2026-11-15

### 变更

- 移除 CLI 模式与 CFST 兼容参数：`internal/app` 只保留 GUI 和 WebUI 健康检查两条启动路径，传入 `--cli` 或旧参数会打印提示并以状态码 2 退出；Windows 发布产物只保留桌面 `cfst-gui.exe`，不再构建命令行版。
- 移除 HTTPing 延迟测速模式：探测固定走 TCP 延迟 + 追踪 + 文件测速，`probe.httping` 与 `strategy=http-colo` 不再生效；追踪阶段的有效状态码与地区码过滤保留在 `trace_status_code`、`trace_cf_colo`、`trace_cf_colo_mode`。
- 桌面端支持把输入文件拖到 exe 上：文件作为本地输入源加入列表（不自动写盘），保存配置后生效；应用已运行时拖放会转发给现有窗口。
- 移除「屏蔽 403」「屏蔽 1034」两个开关：403 / 1034 一律直接淘汰该 IP，不重试、不计入连续失败冷却（等同原先两个开关都打开）。`probe.ignore_403`、`probe.ignore_1034` 不再读取，旧配置里的这两个键会被忽略；`stage.reject` 与 `probe.edge_rejected` 的 `ignored` 字段保留，值恒为 `true`。
- 阶段 2 追踪也识别 1034：追踪响应是 403 且正文含 `error code: 1034` 时，`internal/task/head.go` 当场淘汰该 IP，不再浪费一次文件测速；`probe.edge_rejected` 事件的 `stage` 因此可能是 `stage2_trace`。1034 是 zone 级响应，默认追踪地址所在的 Cloudflare 官方 zone（`speed.cloudflare.com`）不会返回它，所以这条拦截只有把 `trace_url` 换成自己站点的 `/cdn-cgi/trace` 才可能命中；设置页在追踪域名为 Cloudflare 官方 zone 时给出醒目提示，说明默认端点筛不出 1034。
- 设置页「探测策略」三个阶段的桌面排版收口，不再留半行空白：第一阶段把 TCP 延迟区间从独占整行压到两列，与丢包率同排，于是 8 个字段整齐占满三行（原来丢包率一行只占 1/3）；第三阶段的「输入上限」从「底层调优」上移到「核心目标」，与结果展示数量并排，底层调优因此从 7 项变成 6 项、正好两行，末尾不再空两格；单 IP 测量窗口的「预热 / 正式测速」两个输入改为等分撑满整行（原来右侧空出一大块）；同时「阶段1 TCP 超时」改称「第一阶段 TCP 超时」，与阶段标题一致。字段、配置键与后端行为均未变。
- 「DNS 推送平台」新增阿里云 DNS 与 DNSPod（腾讯云）两个平台，与 Cloudflare、DNSHE 并列：阿里云走云解析 RPC 接口（AccessKey ID/Secret，`SignatureVersion 1.0` + HMAC-SHA1），DNSPod 走腾讯云 API 3.0（SecretId/SecretKey，`TC3-HMAC-SHA256`），两者都先用完整域名在账号下按最长后缀匹配所属域名，再按 A/AAAA + TTL 覆盖同步；域名不在账号下时直接报错、不写入任何记录。`dns_providers[].provider` 增加 `aliyun`、`dnspod`，`alidns` / `dndps` / `tencentcloud` 作为读取别名，旧配置与旧键不受影响。
- 「DNS 推送平台」的每张平台卡片默认折叠：标题行显示平台名、记录名和「已就绪 / 待补全 / 已停用」状态，点标题展开字段，新添加的平台自动展开；添加按钮从 2 个扩到 4 个（Cloudflare / DNSHE / 阿里云 DNS / DNSPod）。

### 修复

- 403 / 1034 的屏蔽与放行边界：放行的 403 / 1034 不会产生结果行，屏蔽的 403 / 1034 不再计入连续失败冷却。两种情况的 `stage.reject` 与 `probe.edge_rejected` 事件都带 `note` 和 `ignored` 字段，放行的也能在界面里认出来。
- 403 / 1034 的正文嗅探从 64 字节放宽到 512 字节（`Range: bytes=0-511`），避免 1034 标记靠后时被误判成普通 403、落回 403 开关的行为。

## [2.0.7] - 2026-11-14

### 变更

- 设置页「探测策略」的三个阶段与 MICS 抽样改为可折叠：折叠时摘要行直接显示关键取值（第一阶段 端口/并发/延迟区间/丢包上限，第二阶段 并发/有效状态码/地区码白黑名单，第三阶段 并发/单 IP 时长/速率依据/结果条数，MICS 抽样 预算/并发），展开后才显示该块字段；展开区内的字段改为按「测量方式 → 阈值 → 并发与规模」成对排列，长说明字段（TCP 并发线程、追踪 URL、最终地区码筛选）独占整行。MICS 标题写明「仅 MICS 来源」，原来的「独立参数」徽章随之去掉。极速模式下第三阶段整块收起并标注「已跳过」，不再渲染 14 个不生效的下载参数。字段、配置键与后端行为均未变。

- 「探测策略」展开区按低端 WebView 约束重做：单位改为输入框内的静态后缀（ms、%、MB/s、KiB、秒），值为 0 表示不限制的字段显示「不限制」徽标；丢包率改为整数百分比输入（配置仍存 0~1）；TCP 延迟上下限合并为一行区间，下限不小于上限时标红；阶段1 超时小于延迟上限时给出警告条与一键同步；端口策略切到「输入源端口优先」后「测速端口」改称「回退测速端口」；TCP 并发线程提供 128/200/384 胶囊，平台建议收进字段级 ? 提示。第二阶段按「阶段准入与 COLO 策略 → 追踪请求配置 → 最终地区码过滤」三段重排，地区码改为标签容器（回车或逗号批量添加、点 × 删除、可套用亚太优选/美西），有效状态码提供 200/204/不限制 胶囊且选「不限制」时禁用输入，追踪 URL 在标准模式下降级为备用端点。第三阶段改为核心目标、异常过滤、底层调优三层，底层 7 项默认不渲染；最低下载速度旁显示约合 Mbps 并提供 5/15/50 胶囊，预热与正式测速合并为串联组。MICS 抽样面板在自动模式下隐藏输入框、改为显示推导结果（预算为第一阶段候选池上限×3，并发同步第一阶段 TCP 并发），切到手动时预填该值，摘要行同时显示「手动(值)」；底部大段说明与字段名一并移除。移除底部跨阶段脚注，说明改由字段级 ? 提示承载；折叠与提示均不使用高度过渡，靠原生 details 与 v-show 瞬时切换。
- 桌面端 WebView2 profile 从漫游的 `%APPDATA%\CFST-GUI\webview2` 迁到本机缓存目录 `%LOCALAPPDATA%\CFST-GUI\webview2`：域账号或开启配置漫游的部署不再在登录/注销时同步一份浏览器缓存。首次启动自动搬迁旧目录，localStorage 与缓存保留。
- `probe.edge_rejected` 界面事件改为按 250 ms 窗口批量下发（明细在 `items`、条数在 `count`）：一条几千 IP 源被 403/1034 拒绝时不再逐颗过一次桥。旧形态的单条字段仍然照最后一条保留。同时把该事件的快照落盘并入已有的每秒节流，去掉每次拒绝一次带 fsync 的重写。
- Android 端 WebView renderer 改为 `waivedWhenNotVisible`：切到后台后把回收优先级让给系统内存压力，减少被 low-memory kill 的概率；JS 与探测进度不受影响。
- 桌面端、WebUI 与 Android 每次启动检查 COLO 词典：三份本地 COLO 文件（综合 / IPv4 / IPv6）有任何一份缺失或读不出来时自动补一份（本地已有 GEOFEED 就只重建、不联网），后台异步执行不阻塞启动。失败只写 `colo.dictionary.warmup` 调试事件，并在下一次测速结果的提示里说明，不影响测速本身。运行中切换数据目录（Android 旧版 self-hosted 存储的迁移路径）会立刻对新目录再检查一次；手动「更新 / 处理词典」成功后旧提示立即失效，切换目录后旧目录迟到的失败结论也不会覆盖新目录。CLI 不参与启动自检。

### 修复

- 追踪阶段的诊断统计（`trace_diagnostics` 的 `reason_counts`、`status_counts`、`samples`）此前由多个追踪 worker 并发写入同一批 map 与切片且未加锁：竞态下日志、失败提示和结果里的诊断数字会静默失真，极端情况还能触发 Go 的 `concurrent map write` 直接终止进程。现在写入与读取共用一把互斥锁。
- 文件测速的并发 range 请求共享同一个 HTTP 客户端，重定向回调会并发写“最后一次重定向目标”；`go test -race` 可复现，调试日志里的重定向目标可能与实际请求对不上。改为加锁记录。
- 探测 worker 异常退出（panic）此前只写调试日志，关掉 debug 时表现为“某个 IP 莫名其妙没有结果”。现在引擎会计数，并在本次探测结果的 `warnings` 里说明异常退出数量；对应 IP 仍按失败处理，其余 IP 照常跑完。

## [2.0.5] - 2026-11-12

### 变更

- 输入组统一管理 IP 模式和 COLO 筛选并保存在组档案中；新增可自托管的 Cloudflare Worker 追踪探针，并改进 COLO 地点词典的名称匹配。

- 事件节流 `probe.event_throttle_ms` 现在真正生效：运行前按该值下发进度节流，`probe.progress` 事件与阶段日志共用同一个节流窗口。此前该设置只写进调试摘要，运行时固定 100 ms。
- 追踪阶段字段改名：`probe.httping_status_code` → `trace_status_code`、`probe.httping_cf_colo` → `trace_cf_colo`、`probe.httping_cf_colo_mode` → `trace_cf_colo_mode`（对应 Go 字段 `TraceStatusCode` / `TraceCFColo` / `TraceCFColoMode`）。旧键继续兼容读取，保存时只写新键；`httping` 仍是只对 CLI `-httping` 生效的 HTTPing 模式开关。
- `stage_limits.stage3 = 0` 现在按文档默认值 `100` 处理，不再回落到旧隐藏值 `10`；`-1` 仍表示不限制（CLI 使用）。
- 自定义 Host/SNI 取消总开关 `probe.custom_host_sni`：四个字段非空即生效，留空时 Host 与 SNI 跟随 URL。旧配置显式写 `false` 时迁移会清空这四个字段，保持"关闭即不生效"的既有行为。

### 移除

- 移除无效字段 `probe.download_count` / `testCount`（Go `TestCount`）与 `probe.download_speed_sample_interval_seconds`：它们从未被运行时读取。旧 `download_count` / `testCount` 仅在迁移时作为 `stage_limits.stage3` 的兼容来源。
- 移除 `probe.thresholds.max_http_latency_ms`（`HeadMaxDelayMS`）：归一化阶段本来就会丢弃该值，运行时固定不限制。
- 移除 `probe.timeouts.stage3_ms`：文件测速超时一直由 `download_time_seconds` 决定。旧配置只有 `stage3_ms` 时按秒迁移到 `download_time_seconds`。

### 修复

- 展开输入组、输入源卡片或设置页分区时右侧会突然探出滚动条：这条滚动条占掉 15px 宽度，居中的内容整体左移、卡片里的文字重新换行，看起来整页抖一下。桌面外壳的内容滚动区改为常驻滚动条槽（`scrollbar-gutter: stable`），宽度不再随滚动条出现或消失而变化。

## [2.0.4] - 2026-10-29

2.0.3 的 GitHub Release 已撤回（原 Release 转为 draft），代码与 `v2.0.3` 标签保留在仓库中。撤回原因是文件测速实时速率读数异常（0 与尖峰交替、收尾显示 `-`），修复见下文「修复」一节，随后以 2.0.4 重新发布。

### 新增

- DNS 设置从「系统配置 → 高级 → 网络与任务 → 第一阶段」的普通字段抽成独立卡片「网络与任务 → DNS 与 ECH」，集中放 DoH 地址（含三个预设与自定义）、ECH 开关、ECH 域名和 ECH 配置；折叠标题显示当前 DoH 预设，修改沿用设置页的自动保存。
- 新增 `doh.test` 命令与「测试 DoH」按钮：用当前配置的端点解析测试主机并报告 IPv4/IPv6 数量，再按手填配置、来源主机 HTTPS RR、ECH 域名的顺序报告 ECHConfigList 来源与字节数。测试路径不回退系统 DNS，端点故障会明确报错；合法 NXDOMAIN 视为端点可用；非法 ECH 域名或 base64 配置降级为自动获取并给出警告。
- 文件测速 URL 从「探测策略 → 第三阶段」的高级专属区域提到「探测策略」顶部，普通设置里也能直接改；字段只有一份，高级设置看到的是同一处。追踪 URL 留空时仍由该地址派生 `/cdn-cgi/trace`。
- 403 与 1034 拆成两个独立开关：「屏蔽 403」（`probe.ignore_403` / `-ignore-403`，默认开启）与「屏蔽 1034」（`probe.ignore_1034` / `-ignore-1034`）分别决定两类响应的处理。屏蔽即跳过：不重试、不计入连续失败冷却；不屏蔽则按普通失败处理：按 `retry_policy` 重试并计入冷却。1034 判定优先于 403，屏蔽 403 不会连带屏蔽 1034。阶段 1 HTTPing 遇到 403 会补一次小范围 GET 读正文来区分 1034 和普通 403。
- 两种开关设置下，没测到速率的 IP 都不会进入结果：403 / 1034 只是被拒绝的原因，不是保留理由。日志里用 `reason=edge_ip_restricted` / `forbidden` 加 `note` 字段标出是哪一类，`cfip-log.txt` 打开 debug 后可按 reason 筛出这些 IP。
- 新增 `probe.edge_rejected` 事件：403 / 1034 淘汰会同时发一条不受调试开关约束的界面事件，进程面板直接显示「边缘 IP 受限 1034」或「HTTP 403 拒绝」。此前只能打开调试日志写 `cfip-log.txt` 才能在 `stage.reject` 里看到 1034 标识。

### 变更

- `probe.ignore_1034` 默认值从 `true` 改为 `false`：边缘 IP 受限不再默认跳过，而是按普通失败处理（重试并计入冷却）。旧配置里显式写入的值继续保留；升级后如需维持「不重试」的旧行为，请在设置页勾选「屏蔽 1034」或传 `-ignore-1034`。
- 结果 CSV 统一为 10 列：`IP 地址、已发送、已接收、丢包率、TCP延迟(ms)、平均速率(MB/s)、最高速率(MB/s)、地区码、追踪延迟(ms)、备注`。任务自写的 `result.csv` 与结果页「导出 CSV」从此是同一个形状（此前前者 8 列、后者 9 列且末列不同）。旧文件仍可按表头读取；追加模式下新写入的行会比旧表头多列。
- 阶段 1 HTTPing 的 403 淘汰改用 `reason=forbidden`（此前与其它状态码不匹配共用 `status_mismatch`），这样两类边缘拒绝在 `stage.reject`、`probe.edge_rejected` 和文档里用同一套名词；非 403 的状态码不匹配仍是 `status_mismatch`。
- 自定义 Host / SNI 改为默认关闭：新增 `probe.custom_host_sni` 开关，关闭时 `host_header`、`sni`、`download_host_header`、`download_sni` 一律不生效，Host 与 SNI 自动跟随 URL（追踪阶段跟随 `trace_url`），行为与只填 URL 完全一致；设置页四个输入框在开关关闭时置灰。旧配置没有该字段时，只要填过任一字段就按开启处理，升级前后请求身份不变；CLI 的 `-host` / `-sni` 传入即自动开启。

### 修复

- 设置页第一次展开或收起卡片会整页抖一下：展开动效首次开启时给所有 `.settings-mode-enter` 元素（包含全部折叠卡片）挂了 `block-size: 0 → auto` 动画，卡片会一起塌到 0 再长回来；现在这张动画只作用于切换普通/高级时新出现的分区，卡片本身只用 `::details-content` 的高度过渡，并且该过渡常开，第一次 toggle 也能平滑推开而不再硬跳。内容淡入也改为只跟随被点击的那张卡片。
- 文件测速实时速率改为最近 1 秒滑动窗口，刷新节奏与窗口对齐（`download_speed_sample_interval_ms` 默认改为 `1000`，更小的值按 1 秒执行）；读数不再在 0 与尖峰之间跳动，收尾采样也不再显示 `-`，最高速率同样取窗口速率且不会低于平均速率。

## [2.0.3] - 2026-10-29（已撤回）

### 新增

- DoH 输入源解析：新增 `probe.doh_url`（RFC 8484），同时解析输入源域名与来源 URL 主机名，A/AAAA 并行查询、4 秒预算、IPv4 优先；端点失败回退系统 DNS，合法 NXDOMAIN 不回退。DoH 客户端直连并固定 TCP；端点必须是 `https://`，非法值清空并给出配置警告。设置页提供阿里 / DoH.pub / Cloudflare 预设，CLI 为 `-doh`（留空即默认阿里 DoH）。
- 来源请求 ECH：按主机维护 TCP 传输，从同一 DoH 端点查询 HTTPS RR 的 `ech` 参数并挂到 TLS 握手，来源主机自有记录优先；无记录或被拒时回退普通传输并仍校验真实证书。新增 `probe.ech`、`probe.ech_config`、`probe.ech_domain`，CLI 为 `-ech`、`-ech-config`、`-ech-domain`。
- 新增 `probe.source_pool_limit`（默认 `65536`，上限 `1000000`）与 CLI `-pool-limit`，负 `ip_limit` 改为“展开到上限为止”。
- 下载测速并发上限从 1 提升到 16（默认仍为 1），设置页新增“并发测速 IP 数”。

### 变更

- 任务快照写盘按任务合并为每秒至多一次；结果页缓存已解码行，文件未变时不重复读取与排序。
- COLO 词典改为索引查询；HTTP/3 失效路径在进程内暂停 30 秒，避免每次探测重付 QUIC 握手代价。
- 候选池有界展开并在 stage 0 直接汇总候选、逐个来源上报进度；域名解析按次数（`limit × 4`，钳制 `1024..20000`）和 30 秒累计时长设预算，超限告警并标出截断。
- 结果页精简并默认按平均速度排序；仪表盘启动回退限定在乐观窗口内；前端只挂载当前 shell，进程条目改用稳定 key；设置页分区展开状态持久化。

### 修复

- 远端来源只走 TCP，避免慢 QUIC 握手吃满 30 秒来源超时后用过期的请求上下文回退。
- 来源遇到 429/5xx 重试一次并继续切换候选 URL；失败只更新状态文案，不再清空上一次成功读取的时间与数量；全部来源失败时补充直连提示。
- 探测阶段在后置检查通过后才计入 `completed_stages`。
- 调度器 `skip_if_active` 窗口内的 `PROBE_ALREADY_RUNNING` 记为跳过，不再让历史与下游 DNS/GitHub 状态显示失败。
- 配置写入串行化；来源状态就地更新，未知字段不丢失，缺少 `enabled` 键的来源视为启用。
- 移除事件发射器遗留的调试输出；恢复展开或重新进入设置页不再触发整页淡入。
- 桌面端上传结论的系统通知恢复发送：前端此前调用 Wails v2 的 `window.runtime` 接口，在 v3 宿主下必然抛错并只留下 `notification.desktop_failed` 日志；现在改由 Go 侧在 `upload.notification` 事件上调用 Wails v3 通知服务，主窗口隐藏在托盘时同样能收到。通知初始化失败只写 `error-log.txt`，不影响启动与测速。
- Windows 发行构建与 `task build` 补齐 Wails v3 的 `production` 标签（macOS 的 `wails3 build` 原本就会自动追加）。缺少该标签时发行包会编入 Vite 反代路径与调试开关。

## 更早版本

| 版本 | 日期 | 摘要 |
| --- | --- | --- |
| [v2.0.2](docs/release-notes/v2.0.2.md) | 2026-10-09 | 多渠道上传通知（Telegram、Webhook、钉钉、企业微信、Email）、MICS 抽样预算与下载成功数上限、Windows 系统代理、HTTP/2 超时修复、oxlint + oxfmt。 |
| [v2.0.1](docs/release-notes/v2.0.1.md) | 2026-09-20 | 修复 Android WebView 渲染进程被回收导致的进程终止，消除冷启动白屏，清理 CodeQL 告警。 |
| [v2.0.0-fix](docs/release-notes/v2.0.0-fix.md) | 2026-09-19 | 修复 bridge 通道判定、Windows WebView2 数据目录与移动端帮助提示。 |
| [v2.0.0](docs/release-notes/v2.0.0.md) | 2026-09-19 | 设置页分区帮助、前端缓存策略、启动诊断、桌面/CLI 启动模式拆分、质量门禁与文档拆分。 |

全部历史版本见 [`docs/release-notes/README.md`](docs/release-notes/README.md)。
