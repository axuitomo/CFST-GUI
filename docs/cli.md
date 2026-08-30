# CLI 指令

本文档整理当前仓库可直接使用的 GUI、CLI、验证和 Release 命令。命令示例假设工作目录为仓库根目录。

## 运行模式

根目录 `main.go` 是薄入口，只负责注入嵌入资源并调用 `internal/app.Run`。运行模式判定在 `internal/app/run.go`：无参数时进入 Wails 桌面 GUI；第一个参数不是 `--gui` 时进入 CLI；第一个参数为 `--cli` 时会先移除该标记再解析 CFST 参数。

CLI 只负责把兼容参数转成共享探测 payload，再调用 `internal/appcore.Service.RunProbe`。TCP、追踪、下载、导出和任务快照与桌面、WebUI、Android 走同一条编排；CLI 仍保留原参数、控制台摘要和 `-o` 相对路径写出到当前工作目录，不走测速后自动 DNS/GitHub 推送。

| 命令 | 行为 |
| --- | --- |
| `go run .` | 启动桌面 GUI |
| `go run . --gui` | 显式启动桌面 GUI |
| `go run . --cli ...` | 进入 CLI，解析后续 CFST 参数 |
| `go run . -f ip.txt -o result.csv` | 兼容旧用法，直接进入 CLI |
| `./cfst-gui --cli ...` | 构建后二进制运行 CLI |

## 桌面 GUI

首次开发建议先安装 Wails CLI，并安装前端依赖：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
pnpm --dir frontend install
```

开发模式启动：

```bash
wails dev
```

构建当前嵌入式前端后直接运行 Go 程序：

```bash
go run .
```

如果单独执行前端命令时提示缺少 `frontend/wailsjs`，先在仓库根目录执行一次 `wails dev` 或 `wails build` 生成 Wails bridge。

## CLI 示例

使用默认 `ip.txt` 输入并写出 `result.csv`：

```bash
go run . --cli -f ip.txt -o result.csv
```

直接通过参数指定 IP/CIDR，限制 TCP 平均延迟和丢包率：

```bash
go run . --cli -ip 1.1.1.1,2.2.2.0/24 -tl 200 -tlr 0.15 -o result.csv
```

只做延迟和追踪探测，不做文件测速：

```bash
go run . --cli -f ip.txt -dd -p 20
```

自定义测速 URL、Host、SNI 和 User-Agent：

```bash
go run . --cli -url https://speedtest.xyz9923.dpdns.org/500m -host cf.example.com -sni cf.example.com -ua "Mozilla/5.0 ..."
```

指定下载协议；Linux ARM 和 Android 上 `auto` 会回退到 `tcp`：

```bash
go run . --cli -f ip.txt -http-protocol auto
go run . --cli -f ip.txt -http-protocol h3
```

## CFST 兼容参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-n` | `200` | 延迟测速线程数，最大会归一化到 `1000`。 |
| `-t` | `4` | 单个 IP 延迟测速次数，最少 `2`。 |
| `-dn` | `10` | 保留参数；当前不再限制下载测速数量。 |
| `-dt` | `4` | 单个 IP 下载测速最长时间，单位秒。 |
| `-tp` | `443` | 延迟测速和下载测速端口。 |
| `-url` | `https://speedtest.xyz9923.dpdns.org/500m` | 文件测速地址；CLI 会从该 URL 推导 `/cdn-cgi/trace` 追踪地址。 |
| `-ua` | 内置 Firefox UA | 自定义请求 User-Agent。 |
| `-host` | 空 | 强制覆盖请求 HTTP Host；CLI 的追踪 URL 从文件测速 URL 派生，因此两个阶段共用该值。 |
| `-sni` | 空 | 强制覆盖请求 TLS SNI；CLI 的追踪 URL 从文件测速 URL 派生，因此两个阶段共用该值。 |
| `-debug-capture` | 空 | 调试模式下把实际拨号目标改到指定地址。 |
| `-tls-insecure` | `false` | 忽略 TLS 证书校验；仅在明确需要跳过校验时传 `-tls-insecure=true`。 |
| `-http-protocol` | `auto` | 下载测速 HTTP 协议，可用 `auto`、`tcp`、`h1`、`h2`、`h3`。`auto` 在 Linux ARM 和 Android 上回退到 `tcp`。 |
| `-httping` | `false` | 使用 HTTPing 模式做延迟测速；该开关只对 CLI 生效，GUI/WebUI/Android 仍固定走 TCP + 追踪。 |
| `-httping-code` | `200` | HTTPing 有效状态码；默认只接受 `200`，显式设置 `0` 可关闭状态码筛选。 |
| `-cfcolo` | 空 | HTTPing 模式下按 IATA 机场码或地区码过滤，英文逗号分隔。 |
| `-tl` | `9999` | 平均延迟上限，单位 ms。 |
| `-tll` | `0` | 平均延迟下限，单位 ms。 |
| `-tlr` | `0.15` | 丢包率上限，范围 `0.00` 到 `1.00`。 |
| `-sl` | `0` | 下载速度下限，单位 MB/s。 |
| `-p` | `10` | 终端显示结果数量；为 `0` 时不显示结果直接退出。该值只影响控制台，不裁剪写出的 CSV。 |
| `-f` | `ip.txt` | IP 段数据文件路径；与 GUI 本地输入源一样按 32MiB 上限读取。 |
| `-ip` | 空 | 直接指定 IP/CIDR，英文逗号分隔。 |
| `-o` | `result.csv` | CSV 输出文件；传空字符串可不写文件。 |
| `-dd` | `false` | 禁用下载测速，结果按延迟排序。 |
| `-allip` | `false` | IPv4 段内测速全部 IP，而不是每个 `/24` 随机一个。CLI 会把 CIDR 原样交给共享探测引擎展开。 |
| `-debug` | `false` | 输出更多调试日志，并写入 `cfip-log.txt`。 |
| `-v` | `false` | 打印版本并检查 GitHub Releases 更新。 |
| `-h` | `false` | 打印帮助。 |

## 前端与验证

前端命令可在仓库根目录通过 pnpm 脚本执行。当前前端工具链基线为 Node.js 22、Vite 8.2、Tailwind CSS 4.3、TypeScript 6 API 和 `vue-tsc` 3；独立 `pnpm tsc:ts7` 使用 TypeScript 7.0.2 的 `tsc`。Tailwind 由 `@tailwindcss/vite` 接入，生产构建会刷新 `frontend/dist` 中的 hashed assets。

在 Windows PowerShell 的仓库根目录运行 pnpm 脚本：

```powershell
pnpm test
pnpm lint
pnpm typecheck
pnpm build
& .\scripts\check.ps1
& .\scripts\lint.ps1
& .\scripts\ci-local.ps1
```

`check.ps1` 执行过滤后的 Go 测试、前端单测、类型检查和生产构建；`lint.ps1` 执行 `go vet`、可选 shellcheck 和 ESLint；`ci-local.ps1` 组合格式、lint、功能、生成物和依赖审计。运行前先确认 `node --version` 和 `pnpm --version` 可用；仓库不要求 WSL，跨平台环境仍可使用同名 `.sh` 脚本。

Go 侧测试在仓库根目录执行：

```powershell
$goPackages = @(go list ./... | Where-Object { $_ -notmatch '/frontend/node_modules(?:/|$)' })
go test $goPackages
```

Android 相关验证在仓库根目录或 `mobile/android` 下执行：

```powershell
Push-Location mobile/android
.\gradlew.bat testDebugUnitTest
.\gradlew.bat lintDebug
.\gradlew.bat assembleDebug
Pop-Location
bash scripts/check-android.sh `
  mobile/android/app/libs/mobileapi.aar `
  mobile/android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk
bash scripts/android-doctor.sh
```

`scripts/android-doctor.sh` 还会阻塞 Android Activity 隐藏状态栏/系统栏、启用 WebView 自动暗化、输入框聚焦强制居中滚动，以及用 `visualViewport` 驱动 app 根高度的改动。输入框聚焦稳定性、按钮颜色/文字对比、安装确认页返回后的闪烁问题和刘海屏/打孔屏视觉避让仍应在真机或 AVD 上手测。

连接真机或 AVD 后，可追加设备侧 smoke：

```powershell
bash scripts/android-doctor.sh --device-smoke `
  --device-smoke-apk mobile/android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk
```

## Release 命令

统一构建脚本位于 `scripts/build-release.sh`，默认目标是 `all`：

```bash
bash scripts/build-release.sh
bash scripts/build-release.sh all
```

也可以按目标单独构建：

```bash
bash scripts/build-release.sh windows
bash scripts/build-release.sh linux
bash scripts/build-release.sh linux-amd64
bash scripts/build-release.sh linux-arm64
bash scripts/build-release.sh darwin-amd64
bash scripts/build-release.sh darwin-arm64
bash scripts/build-release.sh android
bash scripts/build-release.sh manifest
```

`linux` 会一次生成 `amd64` 和 `arm64` 两种 Linux WebUI bundle；两份 bundle 都同时支持 `docker compose up -d --build` 与 bundle 内 `./run-local.sh` 的本地运行入口。Android Release 目标需要先提供签名环境变量，详见 [Docker 与环境变量](./docker-env.md)。
