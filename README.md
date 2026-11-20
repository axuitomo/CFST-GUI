# CFST-GUI

[![Go Version](https://img.shields.io/github/go-mod/go-version/axuitomo/CFST-GUI?style=flat-square&label=Go&color=00ADD8&logo=go)](go.mod)
[![Wails](https://img.shields.io/badge/Wails-v3.2.0--nightly.20261103-f36f45?style=flat-square)](https://wails.io/)
[![License](https://img.shields.io/github/license/axuitomo/CFST-GUI?style=flat-square&label=License)](LICENSE)

CFST-GUI 是一个基于 Wails + Vue + Capacitor 的 Cloudflare/CDN IP 测速工具，提供可视化任务面板、输入源管理、结果导出、配置同步、DNS 记录读取、DNS 平台自检和自动推送能力。

同一套 Vue 前端与 Go 核心（`internal/appcore`、`internal/task`）服务三种形态：Wails 桌面 GUI、Linux WebUI（HTTP 服务）和 Android 应用。

<img width="2560" height="1599" alt="CFST-GUI 主界面" src="https://github.com/user-attachments/assets/64bfa564-391a-47de-aca8-62d3afa57e3e" />

## 当前状态

- 桌面端 Wails v3.2.0-nightly.20261103 + Go 1.27.4，默认原生桌面 GUI
- 前端 Vue 3 + Vite 8 + Tailwind CSS 4 + TypeScript（`vue-tsc` + 独立 `tsc`）+ Phosphor Icons
- Android Capacitor 8.6.2 WebView + Kotlin Plugin（前台服务、WorkManager、SAF、权限、安装更新）+ gomobile AAR；发布基线 JDK 25、AGP 9.5.1、Gradle 9.9.1、SDK/target 37、Build Tools 37.0.0、NDK 30.0.16248370
- 发行产物只覆盖 Windows 和 Android，统一输出到 `build/artifacts/release/`；GitHub Release 不发布 Linux、Docker、macOS 或 iOS 资产

## 功能概览

| 能力 | 说明 |
| --- | --- |
| 桌面测速 | 固定 4 阶段：IP 池 → TCP 延迟 → 追踪探测 → 文件测速。极速模式只跑前三个阶段，完整模式追加文件测速。支持丢包率上限（默认 15%，最高 100%）、地区码和下载速度阈值过滤，结果页可排序、分页并按 IPv4/IPv6 筛选。 |
| 任务控制 | 运行中或暂停中的任务可显式终止，终止会中断输入源加载、网络探测、重试等待和测速后推送，并以「已终止」状态结束，不计为失败或完成。暂停覆盖输入源准备、探测、重试/冷却、CSV 导出、推送和结果持久化；不可回滚的外部写操作会先完成当前安全步骤。 |
| 输入源 | 来自远程 URL、本地文件或手动输入。按行清洗，跳过空行与 `#` 注释，解析 IP/CIDR 与域名（域名用系统本地 DNS 解析）。候选处理支持 `traverse` 与 MICS 抽样，候选总量由 `probe.source_pool_limit`（默认 65536）统一约束。桌面用系统文件对话框，Android 用 SAF 选择器（导入文件复制到 app 私有目录，持久授权仅用于导出目录）。 |
| 探测策略 | 设置页把三个阶段与 MICS 抽样折叠成四行摘要，展开后编辑并发、超时、有效状态码、地区码白黑名单、测速时长与速率依据等；单位内联在输入框右侧，`0` 显示为「不限制」，区间冲突给红框提示，阶段 1 超时小于延迟上限时给黄色警告并可一键同步。完整口径见[配置详解](docs/guide/configuration.md)和[行为基线](docs/dev/behavior-baseline.md)。 |
| 结果与导出 | CSV 导出默认文件名 `result.csv`；桌面/WebUI 写入导出目录，Android 写入持久化授权的 SAF 导出目录。文件测速默认用 4 个 HTTP Range GET 分片，服务端不支持 Range 时回退完整流式 GET。 |
| DNS | DNS 读取页只经 Cloudflare 官方 API 只读记录，可按 Zone 全量、配置中的记录名或指定子域名读取，并筛选 A/AAAA。推送在定时任务与「测速后自动推送」中执行，支持 Cloudflare（API Token + Zone ID + 分流规则）、DNSHE、阿里云 DNS 和 DNSPod 并列配置；IPv4 写 A，IPv6 写 AAAA。 |
| 配置与同步 | 以当前 `config_snapshot` schema 为准。ZIP 配置包固定包含 `cfst-gui-config.json`，另支持 WebDAV 测试/备份/还原、`source-profiles.json` 输入组、GitHub 结果 CSV 导出；旧配置读取时补齐默认值、迁移旧字段别名，落盘时写成当前规范格式。 |
| 在线更新 | 设置页直连 GitHub Releases，按 `cfst-gui-update-manifest.json` 匹配平台资产，下载走加速候选链、全程不读环境代理，并用 SHA256 校验。 |

配置导出、ZIP 归档、WebDAV 备份和本地备份文件可能包含 Cloudflare Token、WebDAV 凭据、导出路径和输入源路径，请只保存到可信位置。

## 文档入口

更完整的使用、部署和接口说明在 [`docs/`](docs/index.md)：

| 场景 | 文档 |
| --- | --- |
| 新手入门：从添加输入源、处理 COLO 与设置 trace，到启动测速 | [docs/guide/CFST-GUI入门教程.md](docs/guide/CFST-GUI入门教程.md) |
| 首次使用：导出目录、COLO 词典、输入源和测速 | [docs/guide/quick-start.md](docs/guide/quick-start.md) |
| 产品定位、发行资产和安装建议 | [docs/guide/介绍产品.md](docs/guide/介绍产品.md) |
| 运行、部署、构建与签名 | [docs/guide/deployment.md](docs/guide/deployment.md) |
| 配置目录、字段默认值和旧配置兼容 | [docs/guide/configuration.md](docs/guide/configuration.md) |
| 全部开发、检查与发版命令 | [docs/dev/commands.md](docs/dev/commands.md) |
| 跨端命令、事件、配置与任务行为基线 | [docs/dev/behavior-baseline.md](docs/dev/behavior-baseline.md) |
| 功能接口契约、配置结构和代码定位 | [docs/reference/README.md](docs/reference/README.md) |
| 浏览器组件测试与截图基线 | [docs/dev/browser-tests.md](docs/dev/browser-tests.md) |
| WebUI、Docker、Android 和 Actions 环境变量 | [docs/guide/docker-env.md](docs/guide/docker-env.md) |
| Android 架构、SAF 文件访问和移动端桥接 | [docs/mobile/android-mobile.md](docs/mobile/android-mobile.md) |
| Cloudflare DNS Token 最小权限 | [docs/integration/cloudflare-api-token.md](docs/integration/cloudflare-api-token.md) |
| GitHub 结果导出 PAT 最小权限 | [docs/integration/github-pat.md](docs/integration/github-pat.md) |
| Telegram Bot 上传通知配置和排错 | [docs/integration/telegram-bot.md](docs/integration/telegram-bot.md) |
| 上传筛选和自动推送口径 | [docs/integration/upload-design.md](docs/integration/upload-design.md) |
| 版本更新日志 | [CHANGELOG.md](CHANGELOG.md) |
| 全部历史版本发布说明 | [docs/release-notes/README.md](docs/release-notes/README.md) |

## 快速开始

### 环境要求

Go 1.27.4、Node.js 26.12.1 + pnpm 12.16.1、Wails v3 CLI。日常主环境是 PowerShell 7；跨平台 `.sh` 脚本通过 Git for Windows 提供的 `bash.exe` 调用，不需要 WSL。Linux 和 macOS 目标仍需对应平台工具链。

```powershell
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.2.0-nightly.20261103
pnpm --dir frontend install
```

### 开发调试

```powershell
wails3 dev                                  # 拉起 Vite 并打开 Wails 原生窗口，前端热更新
wails3 generate bindings                    # 缺 frontend/bindings 时生成 Wails 桥接代码
pnpm --dir frontend build; go run .         # 跑嵌有当前前端产物的独立程序
bash scripts/dev/open-dev.sh webui          # 浏览器形态，http://127.0.0.1:34115
bash scripts/build/build-android-mobile.sh  # 生成 Web assets 与 mobileapi.aar
```

`frontend/dist` 是构建产物、不入库（只保留 `.gitkeep` 让 `//go:embed` 可编译），任何要运行或出货的二进制都必须先构建前端；漏构建会直接报错要求先 `pnpm --dir frontend build`，不会回退到旧界面。启动日志会打印当前前端来源：`proxying live Vite dev server` 表示走 Vite 实时源码，`serving embedded frontend/dist snapshot` 表示用的是二进制内嵌快照。

WebView2 用户数据目录固定为 `%LOCALAPPDATA%\CFST-GUI\webview2`（不放漫游目录，避免同步浏览器缓存）。开发端口占用、孤儿 Vite 进程、单实例锁与托盘的排查步骤见[部署与构建](docs/guide/deployment.md#本地开发)，Android 真机调试见[移动端文档](docs/mobile/android-mobile.md#android-studio-真机调试)。

### 构建发行版

```powershell
$env:CFST_ANDROID_KEYSTORE = 'C:\path\to\release.jks'   # 另需 KEYSTORE_PASSWORD
$env:CFST_ANDROID_KEY_ALIAS = '...'                     # 与 KEY_PASSWORD
bash scripts/build/build-release.sh                     # 默认 all
bash scripts/build/build-release.sh windows             # 或按目标单独构建
```

可用 target：`windows`、`linux`、`linux-amd64`、`linux-arm64`、`darwin-amd64`、`darwin-arm64`、`android`、`manifest`。默认 `all` 强制完成 Windows amd64、Linux amd64/arm64 和 Android arm64，缺任一产物即失败；用逗号分隔的 `CFST_RELEASE_TARGETS`（如 `windows,android`）可收缩本地构建集合。macOS 仅在对应主机构建，不进 GitHub Release。

发行产物（`build/artifacts/release/`）：Windows 安装器 `cfst-gui-windows-amd64.exe`（界面简体中文，安装前检查 WebView2 Runtime，底部可查看 GPL-3.0 与第三方声明）与便携版 `-portable.exe`；Linux WebUI bundle `cfst-gui-linux-{amd64,arm64}.tar.gz`；Android `cfst-gui-android-arm64-v8a-release.apk`；`cfst-gui-update-manifest.json`。GitHub Actions 流水线位于 `.github/workflows/release.yml`，由 `v*` tag、推送 `test` 分支或手动触发，只发布 Windows、Android 和 manifest。签名证书、Windows SDK SignTool、Docker/GHCR 及其他发行细节见[部署与构建](docs/guide/deployment.md)。

## 仓库结构

```text
frontend/          Vue 前端，桌面、WebUI 和 Android 共用（src/lib/bridge.ts 为三端桥接层）
internal/          Go 核心：appcore 业务服务、probecore 探测领域逻辑、task 测速、app 桌面/WebUI 生命周期
mobileapi/         gomobile 暴露给 Android Kotlin 层的服务
mobile/android/    Android 原生工程、Kotlin Plugin、资源和 Gradle 配置
build/             应用图标、平台资源（config/ 构建配置、windows/ 安装器清单与图标）与本地产物（artifacts/，不提交）
scripts/           构建（build/）、质量门禁（checks/）、开发辅助（dev/）和共享函数（lib/）
docs/              guide/ 用户指南、dev/ 开发者、integration/ 集成、mobile/ 移动端、reference/ 接口参考、release-notes/ 版本记录
devtools/          CDP 调试脚本与运行时工具库（不提交）
tools/             开发辅助工具
```

Go module 路径为 `github.com/axuitomo/CFST-GUI`。构建产物统一放在 `build/artifacts/`；`cfst-results/` 存放按日期归档的结果 CSV，不属于构建产物；`devtools/profiles/` 含敏感登录数据，已被忽略。

## 常用开发命令

```powershell
& .\scripts\ci-local.ps1                  # 一键本地质量门禁
& .\scripts\checks\check.ps1              # Go 测试 + 前端单测/typecheck/build
& .\scripts\checks\lint.ps1               # go vet、golangci-lint、shellcheck、actionlint、Oxlint、stylelint、markdownlint、ktlint/detekt
bash scripts/dev/format.sh                # 格式化
& .\scripts\audit.ps1                     # 依赖校验与安全审计
& .\scripts\verify-generated.ps1          # Wails/前端生成物一致性
bash scripts/checks/changed-check.sh      # 只检查当前变更
bash scripts/checks/doctor.sh             # 诊断开发环境
bash scripts/checks/check-android.sh      # Android debug 构建、16KB 页对齐与 APK manifest 检查
bash scripts/checks/release-preflight.sh <version>   # 发版前检查
```

帮助脚本默认只读或 dry-run，会改动文件或环境的脚本要求显式参数（如 `--apply`、`--force`）。完整命令清单、`check.ps1` 与 `verify-generated.ps1` 的前置条件见 [docs/dev/commands.md](docs/dev/commands.md) 和 [`scripts/`](scripts/)。

## 配置与数据

配置根目录由 Go 的 `os.UserConfigDir()` 决定并追加 `CFST-GUI`；桌面端固定使用应用数据目录，Android 使用 `/storage/emulated/0/Android/data/<包名>/files`。设置页不再提供自定义储存目录，仍可用 `CFST_GUI_PORTABLE_ROOT` / `portable.json` 启用便携数据目录。

主要文件：

- `storage.json`：储存 bootstrap，旧版自定义 `storage_dir` 只用于一次性迁移
- `desktop-config.json` / `mobile-config.json`：桌面 GUI / WebUI 与 Android 的当前配置快照，结构同构；`config.json` 兼容旧桥接结构
- `source-profiles.json`：输入组（`items[].sources`）
- `cfip-log.txt`：调试日志，默认 JSONL，可切换自由格式和记录粒度
- `exports/`、`imports/`、`backups/`：结果导出、导入文件和导入前备份

旧配置缺少的字段在读取时补当前默认值，未知字段不会导致读取失败，但保存、导入、WebDAV 写回或切换档案后会被清理为规范格式。字段级说明见[配置详解](docs/guide/configuration.md)。

## 注意事项

- 默认文件测速 URL 为 `https://speed.cloudflare.com/__down?bytes=99999999`（该端点拒绝 ≥ `100000000`）；旧版本的默认地址在配置归一化时自动跟随，自定义地址不受影响。
- HTTP 出口统一使用共享客户端，默认优先 HTTP/3，失败回退 TCP 上的 HTTP/1.1/2；测速 GET 带 `no-store`/`no-cache` 并校验长度、Range 与 Digest/MD5/SHA256 响应头。TLS 证书校验默认关闭，可在「网络与任务 → TLS 证书校验」开启；远端来源抓取和更新包始终校验。
- 测速结果会受代理、运营商、路由器策略和本地网络状态影响；追踪探测、文件测速与大规模扫描可能触发远端或网络侧限制。
- DNS 读取页不会修改线上记录；定时任务和测速后自动推送会真实创建、更新或删除记录，启用前请先读取并确认记录名与分流规则。
- 配置和归档文件可能包含敏感凭据，不要提交到公开仓库或公开分享。

## 致谢与参考

- [XIU2/CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest)：最初的核心测速逻辑基础（上游 GPLv3 归属见 [第三方许可清单](THIRD-PARTY-NOTICES.md)）
- [Leo-Mu/montecarlo-ip-searcher](https://github.com/Leo-Mu/montecarlo-ip-searcher)：MICS 抽样搜索思路
- [cmliu/CF-Workers-SpeedTestURL](https://github.com/cmliu/CF-Workers-SpeedTestURL)：测速 URL 相关参考
- [Netrvin/cloudflare-colo-list](https://github.com/Netrvin/cloudflare-colo-list)：Cloudflare COLO 数据来源参考
- [baipiao.eu.org cloudflare locations](https://www.baipiao.eu.org/cloudflare/locations)：COLO 词典备用城市命名源（参考自 [PoemMisty/CFData-WEB](https://github.com/PoemMisty/CFData-WEB)）
- [Cloudflare local IP ranges](https://api.cloudflare.com/local-ip-ranges.csv)：Cloudflare 本地 IP 段数据来源
- [xiaolin-007/CloudFlareScan](https://github.com/xiaolin-007/CloudFlareScan)：Cloudflare 扫描工具相关参考

## License

Copyright (C) 2026 axuitomo.

本项目采用 GNU GPL 第 3 版（SPDX：GPL-3.0-only），完整许可文本见 [LICENSE](LICENSE)。

随发行物分发的第三方组件、原始许可证文本和归属清单见 [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md)。
