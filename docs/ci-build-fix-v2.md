# CFST-GUI CI 构建故障 修正版实施方案（v2）

> 本方案在逐条核对仓库实际代码后修订，取代原「CFST-GUI 完整修改方案」中与现状冲突的部分。
> 核查基准：`main` 分支当前工作树 + `docs/android-mobile.md` 文档约定。

---

## 0. 与原方案的差异总览

| 原方案条目 | 修正判定 | 处理方式 |
| --- | --- | --- |
| 1.1 新增 `wails-runtime-mock.ts` 垫片 | 需修正 | 垫片 API 面不足（缺 Application/Call/Create/CancellablePromise），改为可选优化项并补全 API，不作为故障修复手段 |
| 1.2 Vite alias 按 mode 切换 | 需修正 | 机制可用但非本故障解药；`@wailsio/runtime` 是 npm 依赖本就不缺，真实风险在 `frontend/bindings` 生成与缓存 |
| 1.3 拆分 build 脚本 | 需修正 | 需同步改造 `scripts/build-release.sh` 与各 workflow，且必须保留 gomobile bind/签名/校验步骤 |
| 1.4 统一 TS 版本 | 暂缓 | 与两个 CI 故障无因果关系；`vue-tsc ^3.4.0` 兼容 TS7 未验证，单独处理 |
| 2.1 gitignore 掉 `file_paths.xml` / `keep.xml` | **不可行，撤销** | `file_paths.xml` 是 APK 更新安全边界；原 `values/keep.xml` 不是删除，而是迁移为不会重复声明资源的 `raw/keep.xml` |
| 2.2 `capacitor.config.ts` 注入 `customConfig.filePaths` | **不可行，撤销** | CapacitorConfig 无此字段，虚构 API |
| 2.3 AGP 9.3.0 / `warning.mode` | 已实施 | AGP 固定为 9.3.0；Gradle 9.5.1 高于其最低 9.5.0，并保留 parallel/build/configuration cache |
| 3.1 新建 `build-android.yml` | **不可行，重写** | 继续复用现有 workflow 和 `build-release.sh android`，保留 SDK、gomobile、签名与 APK 校验步骤 |
| 3.2 缓存策略 | 可行，保留 | bindings 不缓存；Android 先做一次 `--no-build-cache` 资源验证，再验证正常缓存构建 |
| 4. `build-release.sh` 重写 | **不可行，撤销** | 现 Windows 是 `go build -tags tray` + 签名/NSIS，非 `wails build`；重写版会丢 gomobile/签名/校验并删 FileProvider 配置 |
| 5.1 强制禁用代理 | 已实现 | `internal/app/update.go:40` 已 `DisableProxy: true` + 测试；原方案路径 `internal/updater/` 不存在 |
| 5.2 gomobile 边界 | 已满足 | `mobileapi.Service` 只公开 `Init` / `SetEventSink` / `Invoke(string,string)string`（包在根 `mobileapi/`） |
| 5.3 硬编码 manifest 哈希 | **不可行，撤销** | 资产级 SHA256 校验已存在（`verifySHA256`）；manifest 每次发布内容变化，硬编码会阻塞更新 |
| 6. 验证步骤 | 需修正 | `git clean -fdx` 会误删必要文件，改为定向校验 |

---

## 1. 事实基线（本次核查确认）

- 版本矩阵：Wails **v3.0.0-beta.16**（CLI 二进制为 **`wails3`**）· Capacitor **8.5.1** · AGP **9.3.0** · Gradle **9.5.1** · KGP **2.4.10** · JDK **24** · Go **1.27.0** · Node.js **26.7.0** · pnpm **10.34.5** · compileSdk/targetSdk **37** · NDK **29.0.14206865**。
- 前端已有三层分流（`frontend/src/lib/bridge.ts`）：Wails bridge 存在 → 桌面；Capacitor native 且无 Wails bridge → Android `Cfst` plugin；否则 → WebUI fetch。`isWailsRuntimeAvailable()` 运行时判断。
- `frontend/bindings/` 由 `.gitignore` 忽略，每次由 `wails3 generate bindings` 生成；生成的 JS 从 npm 依赖 `@wailsio/runtime` 引入 `Call/Create/CancellablePromise`。
- `mobile/android/app/src/main/res/xml/file_paths.xml` 是 FileProvider 的唯一 `@xml/file_paths` 定义，仅暴露私有 `update_downloads/`；资源收缩保留规则位于 `res/raw/keep.xml`，通过根级 `tools:keep="@xml/file_paths"` 引用目标，不再声明第二个同名 XML 资源。
- `cap sync/copy` 不生成这两个定制文件；共享门禁 `scripts/check-android-fileprovider-resources.sh` 会在同步后检查资源唯一性、保留规则和路径安全边界。
- updater 已 `DisableProxy: true`（`internal/app/update.go`）并已实现下载资产 SHA256 校验（`verifySHA256`）。

---

## 2. 故障一：间歇性 Wails 运行库缺失 —— 修正方案

### 2.1 根因（修正）

- **不是**“generate bindings 不输出 runtime 库”：runtime 是 npm 依赖 `@wailsio/runtime`，`pnpm install` 后即存在。
- 真实风险集中在 **`frontend/bindings/` 缺失或与 node_modules 状态不一致**时，`vite build` 无法解析 `bridge.ts` 中静态导入的 `../../bindings/.../app.js` → “运行库缺失”。
- 当前 CI 各流（release / resubmit / container / quality）都先执行 `wails3 generate bindings` 或走 `build-release.sh`，但**缺少“生成成功”的显式断言**，一旦 wails3 偶发失败或目录被恢复出旧状态，就会带病继续构建。

### 2.2 修复动作

**A. 在 `scripts/build-release.sh` 的 `build_frontend()` 增加 bindings 生成断言**（fail-fast，避免带病构建）：

```bash
build_frontend() {
  cd "$ROOT_DIR"
  wails3 generate bindings
  local bindings_js="frontend/bindings/github.com/axuitomo/CFST-GUI/internal/app/app.js"
  if [[ ! -f "$bindings_js" ]]; then
    echo "ERROR: wails3 generate bindings did not produce $bindings_js" >&2
    exit 1
  fi
  cd "$FRONTEND_DIR"
  pnpm install --frozen-lockfile
  pnpm --dir "$FRONTEND_DIR" run build
}
```

**B. 缓存策略声明**（维持现状 + 显式禁止）：

- 允许缓存：`frontend/node_modules`、pnpm store、Go module cache、Gradle cache。
- **禁止缓存：`frontend/bindings/`**。当前 CI 用 `setup-node cache: pnpm`（缓存 pnpm store，不含项目内 bindings），已满足；不要在 `actions/cache` 中新增 bindings 路径。每次构建强制重新 `wails3 generate bindings`。

**C. 步骤顺序冻结**：`wails3 generate bindings` → `pnpm install --frozen-lockfile` → `pnpm run build`（`build_frontend` 已保证），不要在 android/webui 分支跳过 generate 或执行 `wails build`。

**D.（可选优化，独立 PR，不作为故障修复）**：若想缩减 Android/WebUI 产物中 Wails runtime 体积，可做完整 API 垫片 + `--mode` 别名：

- 垫片需对齐实际导出面：`Application`、`Events`、`Window`、`Call`、`Create`、`CancellablePromise`（由 `wailsRuntime.ts` 与生成 bindings 的 import 确定）。
- `vite.config.ts` 按 `mode` 用 `resolve.alias` 将 `@wailsio/runtime` 指向垫片；`package.json` 增加 `build:wails` / `build:android` / `build:webui`。
- 落地前提：在本地 `pnpm typecheck && pnpm build` 通过后再接入 CI；`vue-tsc --noEmit` 必须能解析垫片类型。

---

## 3. 故障二：Duplicate resources —— 修正方案

### 3.1 根因（已定位）

- `res/xml/file_paths.xml` 已通过文件名定义 `@xml/file_paths`。
- 原 `res/values/keep.xml` 又使用 `<item name="file_paths" type="xml">` 声明同一个资源键，因此 AAPT2 在 `packageReleaseResources` 阶段报告 Duplicate resources。
- 该冲突与 Gradle 缓存和 `cap sync` 无关；删除真正的 FileProvider XML 会破坏在线更新安全边界。

### 3.2 修复动作

1. 保留 `res/xml/file_paths.xml`，且只允许 `<files-path name="update_downloads" path="update_downloads/" />`。
2. 将保留规则移到 `res/raw/keep.xml`，内容使用根级 `tools:keep="@xml/file_paths"`，不再用 `<item type="xml">` 重复声明资源。
3. 新增 `scripts/check-android-fileprovider-resources.sh`，并在 bootstrap、Debug/Release 构建、Android doctor 和 release preflight 中调用。
4. 用 `:app:processDebugResources --no-build-cache` 验证冷构建，再用正常缓存构建验证重复构建和配置缓存兼容性。

---

## 4. CI 工作流修正（如需新建/调整 workflow）

- **CLI 命令**：一律 `wails3 generate bindings`；安装用 `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.16`。
- **JDK**：`setup-java` 用 `java-version: "24"`（项目 `build.gradle` 强制，JVM 版本不符直接抛异常）。
- **复用现有脚本**：Android 资产构建走 `bash scripts/build-release.sh android`（已含 wails3 generate、cap sync、gomobile bind、签名、aapt/页面对齐校验），不要新造重复步骤。
- **SDK 组件**：`platforms;android-37.0`、`build-tools;37.0.0`、`ndk;29.0.14206865`、`cmdline-tools;latest`（对齐现有 workflow）。
- **缓存**：仅 node_modules / pnpm store / gradle / go mod；显式排除 `frontend/bindings` 与 `mobile/android` 动态生成产物。

参考（与现有 `release.yml` android job 对齐的最小修改点）：

```yaml
- name: Set up Java
  uses: actions/setup-java@v4
  with:
    distribution: temurin
    java-version: "24"          # 原方案写 23，必须改 24
- name: Install Wails
  run: go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.16   # 不是 cmd/wails
- name: Build Android asset
  run: bash scripts/build-release.sh android    # 复用，而非新写一整套 gradle 步骤
```

---

## 5. 明确不做的改动（防回退）

1. 不删除 / 不 gitignore `res/xml/file_paths.xml`；不恢复会重复声明资源的 `res/values/keep.xml`，保留规则固定在 `res/raw/keep.xml`。
2. 不使用 `capacitor.config.ts` 的 `android.customConfig.filePaths`（CapacitorConfig 无此字段）。
3. 不把 manifest 内容哈希硬编码进二进制（会阻塞每次更新发布；资产级 SHA256 已有）。
4. 不用 `wails build` 重写 Windows 构建（现为 `go build -tags tray` + NSIS/签名）。
5. TS 双版本统一**暂缓**，在可运行 pnpm 的环境验证 `vue-tsc` 与 TS7 兼容性后单独提交。

---

## 6. 验证步骤

```bash
# 1) 前端类型与构建（含 bindings 断言路径）
cd frontend
pnpm install --frozen-lockfile
pnpm typecheck
pnpm build

# 2) bindings 生成断言
cd ..
bash -n scripts/build-release.sh          # 语法校验

# 3) Android 资源唯一性和无缓存 AAPT2 校验
bash scripts/check-android-fileprovider-resources.sh
cd mobile/android
./gradlew :app:processDebugResources --no-build-cache
cd ../..

# 4) 完整 Android 发布链路（本地有签名环境时）
$env:CFST_ANDROID_KEYSTORE = 'C:\path\release.jks'
$env:CFST_ANDROID_KEYSTORE_PASSWORD = '...'
$env:CFST_ANDROID_KEY_ALIAS = '...'
$env:CFST_ANDROID_KEY_PASSWORD = '...'
bash scripts/build-release.sh android

# 5) CI 稳定性：连续触发 android job（含一次 gradle 缓存命中）确认稳定通过
```

验收口径：
- `wails3 generate bindings` 后 `frontend/bindings/github.com/axuitomo/CFST-GUI/internal/app/app.js` 必须存在，否则构建在 2.2-A 处 fail-fast。
- `res/xml/file_paths.xml` 与 `res/raw/keep.xml` 各仅一份，且任何 source set 中都不存在 `res/values/keep.xml`。
- APK 内 `AndroidManifest` 的 FileProvider 仍指向 `@xml/file_paths`，且 `update_downloads` 路径保留。
