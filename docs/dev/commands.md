# 运行、验证与发布命令

本文档整理当前仓库的 GUI 运行、验证和 Release 命令。命令示例假设工作目录为仓库根目录。

## 运行模式

根目录 `main.go` 是薄入口，只负责注入嵌入资源并调用 `internal/app.Run`。`internal/app/run.go` 只保留两条启动路径：

| 命令 | 行为 |
| --- | --- |
| `go run .` | 启动桌面 GUI（Windows/macOS）或 Linux WebUI |
| `go run . --gui` | 显式启动 GUI，与无参数等价 |
| `cfst-webui --healthcheck` | 容器健康检查，不启动 GUI |

CLI 模式与 CFST 兼容参数已移除：传入 `--cli` 或其他 flag 会以状态码 2 退出，终端运行时可见提示；只传文件路径时按拖放导入处理。桌面端支持把文件拖到 exe 上，文件会作为本地输入源加入列表，保存配置后生效。Windows 发布产物只有一个桌面二进制 `cfst-gui.exe`，以 `-H windowsgui` 链接，双击不附加控制台窗口。

## 桌面 GUI

首次开发建议先安装 Wails CLI，并安装前端依赖：

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.2.0-nightly.20261103
pnpm --dir frontend install
wails3 generate bindings
pnpm --dir frontend dev
```

如果单独执行前端命令时提示缺少 `frontend/bindings`，先在仓库根目录执行一次 `wails3 generate bindings` 生成 Wails V3 bridge。

## 前端与验证

前端命令可在仓库根目录通过 pnpm 脚本执行。当前前端工具链基线为 Node.js 26.12.1、pnpm 12.16.1、Vite 8.3、Tailwind CSS 4.3、TypeScript 6 API 和 `vue-tsc` 3；独立 `pnpm tsc:ts7` 使用 TypeScript 7.0.2 的 `tsc`。Tailwind 由 `@tailwindcss/vite` 接入，生产构建会刷新 `frontend/dist` 中的 hashed assets。

在 Windows PowerShell 的仓库根目录运行 pnpm 脚本：

```powershell
pnpm test
pnpm lint
pnpm typecheck
pnpm build
pnpm check:notices
& .\scripts\check.ps1
& .\scripts\lint.ps1
& .\scripts\ci-local.ps1
```

`check.ps1` 执行过滤后的 Go 测试、前端单测、类型检查和生产构建；`lint.ps1` 执行 `go vet`、`golangci-lint`（errcheck/staticcheck/ineffassign/unused/revive/goimports）、可选 shellcheck、`actionlint`、前端 Oxlint、stylelint、markdownlint、根级 Oxlint 和 Android ktlint/detekt（`ktlintMainSourceSetCheck` + `detektDebug` + `detektDebugUnitTest`）；`ci-local.ps1` 组合格式、lint、功能、生成物和依赖审计。`pnpm check:notices` 需要先生成 `frontend/dist`（即先执行 `pnpm build`），用于依赖变化后校验 `THIRD-PARTY-NOTICES.md` 是否过期；CI 的 Windows 兼容任务会在前端构建后执行同一检查。运行前先确认 `node --version` 和 `pnpm --version` 可用；仓库不要求 WSL，跨平台环境仍可使用同名 `.sh` 脚本。

`scripts/dev/audit.sh` / `audit.ps1` 的 `pnpm audit` 默认指向 `https://registry.npmjs.org` 的审计端点，因为 npmmirror 等镜像不实现该接口，会让门禁直接报错而不是执行审计；需要换端点时设置 `CFST_NPM_AUDIT_REGISTRY`，需要调整阻断级别时设置 `CFST_PNPM_AUDIT_LEVEL`（默认 `moderate`），跳过整段依赖审计时设置 `CFST_SKIP_AUDIT=1`。

Android Kotlin 静态检查使用 detekt `2.0.0-alpha.6`（plugin id `dev.detekt`）。2.x 只在带类型解析的 variant 任务里运行需要绑定上下文的规则，因此 Kotlin 门禁固定跑 `detektDebug`（`src/main`）和 `detektDebugUnitTest`（`src/test`），两者各自维护 baseline：`mobile/android/app/detekt-baseline-debug.xml` 与 `mobile/android/app/detekt-baseline-debugUnitTest.xml`；规则覆盖 `mobile/android/config/detekt/detekt.yml` 与 detekt 内置默认配置叠加，用于把 `MagicNumber` 恢复为“只豁免常量声明”的严格行为。

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
bash scripts/checks/check-android.sh `
  mobile/android/app/libs/mobileapi.aar `
  mobile/android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk
bash scripts/checks/android-doctor.sh
```

`scripts/checks/android-doctor.sh` 还会阻塞 Android Activity 隐藏状态栏/系统栏、启用 WebView 自动暗化、输入框聚焦强制居中滚动，以及用 `visualViewport` 驱动 app 根高度的改动。输入框聚焦稳定性、按钮颜色/文字对比、安装确认页返回后的闪烁问题和刘海屏/打孔屏视觉避让仍应在真机或 AVD 上手测。

连接真机或 AVD 后，可追加设备侧 smoke：

```powershell
bash scripts/checks/android-doctor.sh --device-smoke `
  --device-smoke-apk mobile/android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk
```

## Release 命令

统一构建脚本位于 `scripts/build/build-release.sh`，默认目标是 `all`：

```bash
bash scripts/build/build-release.sh
bash scripts/build/build-release.sh all
```

也可以按目标单独构建：

```bash
bash scripts/build/build-release.sh windows
bash scripts/build/build-release.sh linux
bash scripts/build/build-release.sh linux-amd64
bash scripts/build/build-release.sh linux-arm64
bash scripts/build/build-release.sh darwin-amd64
bash scripts/build/build-release.sh darwin-arm64
bash scripts/build/build-release.sh android
bash scripts/build/build-release.sh manifest
```

`linux` 会一次生成 `amd64` 和 `arm64` 两种 Linux WebUI bundle；两份 bundle 都同时支持 `docker compose up -d --build` 与 bundle 内 `./run-local.sh` 的本地运行入口。Android Release 目标需要先提供签名环境变量，详见 [Docker 与环境变量](../guide/docker-env.md)。
