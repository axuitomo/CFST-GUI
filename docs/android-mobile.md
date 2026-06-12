# Android Mobile Architecture

桌面端继续使用 Wails。Android 端使用 Vue + Capacitor `8.4.0` + Cordova Android `15.0.0` + AGP 9 内置 Kotlin（顶层 KGP classpath 固定 `2.4.0`）+ gomobile AAR，并通过 `mobileapi` 包复用 Go 探测核心。

Android 原生层已从单体 Java plugin 迁移为 Kotlin。Capacitor 入口仍是 `CfstPlugin.kt`，但 SAF、导入导出、存储迁移、更新下载、通知权限、前台任务和调度等职责拆分到同目录下的 `Android*` Kotlin 文件，并配套迁移为 Kotlin 单元测试。

## Build

```bash
go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20260410095206-2cfb76559b7b
gomobile init
bash scripts/build-android-mobile.sh
```

调试构建脚本会执行：

1. `frontend` 生产构建。
2. `pnpm exec cap sync android` 同步 Web assets 和 Capacitor 生成文件到 `mobile/android`。
3. `gomobile bind -target=android/arm64,android/arm -ldflags '-linkmode external -extldflags "-Wl,-z,max-page-size=16384 -Wl,-z,common-page-size=16384"'` 生成 `mobile/android/app/libs/mobileapi.aar`。
4. `mobile/android/gradlew assembleDebug` 输出 ABI split APK。

Release 发行包由仓库根目录的统一脚本生成：

```bash
export CFST_ANDROID_KEYSTORE=/absolute/path/release.jks
export CFST_ANDROID_KEYSTORE_PASSWORD=...
export CFST_ANDROID_KEY_ALIAS=...
export CFST_ANDROID_KEY_PASSWORD=...
bash scripts/build-release.sh
```

也可以只构建 Android 资产：`bash scripts/build-release.sh android`。完整 GitHub Release 由 `.github/workflows/release.yml` 分平台构建桌面和 Android 资产，再集中生成 `cfst-gui-update-manifest.json`。

Release 签名只从环境变量读取，不把 keystore 或密码写入仓库。

Android 原生库要求同时满足两件事：

- `libgojni.so` 的 ELF `LOAD` 段按 16KB (`0x4000`) 对齐，保证 Android 15/16 的 16KB 页设备不落入兼容模式。
- APK 继续保持标准 `zipalign` 对齐，这样 16KB 原生库依然兼容 4KB 页设备。

当前仓库通过 `-Wl,-z,max-page-size=16384` 与 `-Wl,-z,common-page-size=16384` 一起实现这一点；`common-page-size=16384` 不能删除，否则 16KB / 4KB 双兼容会退化。每次 Debug / Release 构建结束后，脚本还会自动检查所有 split APK：

```bash
bash scripts/check-android.sh \
  mobile/android/app/libs/mobileapi.aar \
  mobile/android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk \
  mobile/android/app/build/outputs/apk/debug/app-armeabi-v7a-debug.apk \
  mobile/android/app/build/outputs/apk/debug/app-universal-debug.apk
```

验收重点是：

- `llvm-readelf -l` 看到 `libgojni.so` 的 `LOAD` 段 `Align` 为 `0x4000`
- `zipalign -c -P 16 -v 4` 校验 APK 通过
- `aapt` 校验 APK 内最终 manifest 中的 SDK、通知权限、dataSync 前台服务、FileProvider 和更新清理 receiver 声明

## Outputs

Debug APK 输出在：

- `mobile/android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk`
- `mobile/android/app/build/outputs/apk/debug/app-armeabi-v7a-debug.apk`
- `mobile/android/app/build/outputs/apk/debug/app-universal-debug.apk`

发行版会保留三个 Android APK，并参与统一更新 manifest：

- `build/release/android/cfst-gui-android-release.apk`
- `build/release/android/cfst-gui-android-arm64-v8a-release.apk`
- `build/release/android/cfst-gui-android-armeabi-v7a-release.apk`
- `build/release/cfst-gui-update-manifest.json`

`arm64-v8a` 是 Android 发布必选 ABI，`armeabi-v7a` 用于兼容旧设备。

Android 在线更新会直连检查 GitHub Releases latest，并读取 `cfst-gui-update-manifest.json` 选择最匹配当前 ABI 的 APK；旧版客户端仍会回退到 `cfst-gui-android-release.apk`。读取 manifest 和下载更新 APK 时会直连并发尝试 GitHub 加速候选链（`ghproxy.vip`、`gh.3w.pm`、`gh.ddlc.top` 和原始 GitHub Release 地址），全程不读取环境代理，优先使用最先完整下载且通过 SHA256 校验的结果。下载到 app 私有 `updates/` 目录后使用 `FileProvider` 拉起系统安装确认，`file_paths.xml` 仅暴露 `files/updates/`，不暴露 external/cache 根目录。新旧 APK 必须使用同一签名证书。

## Validation

Android 原生层迁移或工具链升级后，至少运行：

```bash
cd mobile/android
./gradlew buildEnvironment
./gradlew testDebugUnitTest
./gradlew lintDebug
./gradlew assembleDebug
cd ../..
bash scripts/android-doctor.sh
bash scripts/check-android.sh \
  mobile/android/app/libs/mobileapi.aar \
  mobile/android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk \
  mobile/android/app/build/outputs/apk/debug/app-armeabi-v7a-debug.apk \
  mobile/android/app/build/outputs/apk/debug/app-universal-debug.apk
bash scripts/release-preflight.sh 1.8.5 --allow-dirty
```

`scripts/check-android.sh` 对显式传入的 AAR/APK 同时检查 16KB ELF/zipalign 和 APK 内最终 manifest：SDK 版本、Android 13 通知权限、Android 14 dataSync 前台服务、FileProvider authority、更新清理 receiver、私有 `files/updates/` 路径、APK 安装权限、WorkManager 合并组件和敏感组件导出状态。

`scripts/build-android-mobile.sh` 和 `scripts/build-release.sh android` 会重新构建前端并执行 `pnpm exec cap sync android`；当工作树存在无关前端改动时，优先使用上面的 Gradle 与显式 AAR/APK 检查，避免把前端状态同步进 Android 产物。

`scripts/android-doctor.sh` 还会检查 AGP 内置 Kotlin 使用的 KGP buildscript classpath、Android 13 通知权限、Android 14 dataSync 前台服务声明、WorkManager/安装权限、FileProvider authority、私有更新目录路径和更新清理 receiver。

连接真机或可用 AVD 后，先运行设备 smoke：

```bash
bash scripts/android-doctor.sh --device-smoke \
  --device-smoke-apk mobile/android/app/build/outputs/apk/debug/app-universal-debug.apk
```

设备 smoke 会安装 APK、读取设备侧 `dumpsys package`、验证通知/前台服务/WorkManager/FileProvider/receiver 信号，并启动 launcher Activity。随后仍需手测 SAF 目录授权、输入源/配置导入复制、CSV/日志/配置导出、Android 13+ 通知权限弹窗、前台服务任务、WorkManager 定时任务、GitHub 更新下载、SHA256 校验、FileProvider APK 安装确认，以及安装确认页返回后输入框聚焦不闪烁、状态栏仍可见。

## Bridge

前端统一调用 `frontend/src/lib/bridge.ts`：

- Wails bridge 存在时优先走桌面端 `window.go.app.App`，并兼容旧生成物的 `window.go.main.App`。
- Android native 环境且无 Wails bridge 时走 Capacitor `Cfst` plugin。

Android plugin 位于 `mobile/android/app/src/main/java/io/github/axuitomo/cfstgui/CfstPlugin.kt`，通过 gomobile 生成的 `mobileapi.Service` 调 Go，并把 probe 事件通过 `desktop:probe` 回传给前端。

除单任务 probe 外，Android bridge 现在也支持策略管道相关方法：`LoadPipelineProfiles`、`SavePipelineProfiles`、`SavePipelineProfile`、`DeletePipelineProfile`、`RunPipeline`、`StartPipeline`、`CancelPipeline`、`GetPipelineSnapshot`、`ListPipelineResults`。前端仍统一走 `frontend/src/lib/bridge.ts` 做三端归一化。

当前 Android 长任务执行链路已经调整为：

1. 前端提交 `RunProbe` 或 `StartPipeline` 后，Capacitor plugin 会先返回 accepted 响应。
2. `ProbeForegroundService` 在前台服务中继续执行真实长任务；单次 probe 调用同步 `RunProbe`，策略管道调用同步 `RunPipeline`，并共用同一条事件流来细粒度更新系统通知。
3. Go 侧 `mobileapi` 在任务运行过程中持续写入任务快照，任务完成后额外持久化结果行；快照会区分 `active_runtime`、`paused_runtime` 和 `persisted_only`，避免把失联旧会话误判成仍在运行。
4. 前端启动后会先查询 Android 原生运行时状态：若探测任务仍附着在前台服务/Go runtime 上，则自动重新接入当前任务；若只剩快照与已落盘结果，则恢复结果视图并明确提示“当前不可无缝重连”。
5. 结果页在移动端优先使用窗口化列表渲染，并结合分页读取结果，而不是一次性把全量结果灌进 WebView。
6. 设置页的“异常保护”区块会展示 Android 电池优化状态，并提供“申请豁免 / 系统电池设置 / 应用详情”入口；“系统电池设置”会先尝试打开常见厂商自启动/后台白名单页面，失败后回退到 Android 标准电池优化设置。
7. Android 自动调度由 `SchedulerWorker` 基于 WorkManager 注册下一次运行；触发后会拉起 `ProbeForegroundService.startScheduledIntent()`，再由 Go 侧 `runScheduledProbe` 读取保存配置并执行单次测速。Android 调度不会执行工作流；测速完成后仍会按 `auto_dns_push` 和 `auto_github_export` 配置继续 DNS 推送和 GitHub 导出。受系统省电、厂商后台策略和 Doze 影响，实际触发时间可能晚于配置时间。

## Mobile WebView UX

Android WebView 使用 `viewport-fit=cover` 和 safe-area padding 适配状态栏、刘海屏/打孔屏等异形屏、底部安全区与移动端固定导航。Activity 保持 edge-to-edge WebView 布局，但不隐藏 Android 状态栏或导航栏；Android P+ 会启用短边 cutout 布局，系统栏保持可见并由前端 safe-area padding 避让。

Activity 使用 `adjustResize`；前端只通过 `visualViewport` 计算键盘 inset 和键盘开闭状态，并在软键盘打开时隐藏底部导航，避免输入框被遮挡。Android viewport 状态不再驱动 app 根容器高度，也不在输入框聚焦时强制居中滚动，避免键盘动画、浏览器自动滚动和前端布局更新互相拉扯导致画面抖动。

Android 原生层会关闭 theme force dark、WebView `FORCE_DARK_OFF` 和 Android 13+ algorithmic darkening，避免 WebView 或系统深色策略把按钮背景自动变淡、把按钮文字改成低对比颜色。

Android 原生 select 在部分 WebView 中会显示为系统白色大面板；前端会在 Android app 环境拦截 `select` 的 pointer/touch/click 事件，改用应用内底部 picker。该 picker 支持点外关闭、滚动区域选择、Esc 关闭、禁用项和基础 `role=listbox/option` ARIA 状态。

## Notes

- Android 配置文件实际由 app 私有运行时目录中的 `mobile-config.json` 读取；应用存储不再使用 SAF 存储镜像。
- `LoadConfig` / `SaveConfig`、配置归档导入导出和 WebDAV 备份现在都会一并携带 `pipeline_profiles`，对应文件为 app 私有运行时目录中的 `pipeline-profiles.json`。
- CSV、测速文件和调试日志通过已持久授权的 SAF 导出目录写入；未选择导出目录或权限失效时会明确失败并要求重新选择。
- Android 任务快照和分页结果缓存默认保存在 app 私有运行时目录下的 `tasks/`，用于进程重建后的恢复读取。
- 输入源文件和配置导入通过 SAF 文件选择器完成，输入源文件会复制到 app 私有 `imports/` 目录供 Go 侧读取。
- Android SAF 持久化权限只用于导出目录，不参与配置读取或应用数据持久化。
- `scripts/android-doctor.sh` 和 `scripts/release-preflight.sh` 会阻塞隐藏 Android 状态栏/系统栏、启用 WebView 自动暗化、输入框聚焦强制居中滚动，以及用 `visualViewport` 驱动 app 根高度的改动。
- `probe.failed` / `probe.completed` 事件会携带 `failure_stage` 与 `trace_diagnostics`，便于前端展示更接近真实原因的错误摘要；Android 原生 bridge / storage fallback 会额外写入 `Logcat`，默认 tag 为 `CfstPlugin`。
- 当前 Android 的 `StartPipeline` 已复用 `ProbeForegroundService` 承载策略管道执行与通知更新；`RunPipeline` 仍保留同步 bridge 语义，便于桌面 / WebUI / native 三端继续共用同一套接口。
- Android 调度当前固定为单次测速模式；前端会隐藏工作流定时模式，并把 `scheduler.run_mode` 归一为 `probe`。DNS 推送和 GitHub 导出仍可作为测速后的后续动作配置。桌面和 WebUI 仍支持 `pipeline` 定时工作流。
- 当前 `CancelProbe` 会在阶段边界生效，底层测速阶段运行中不会被强制中断。
- 结果页不再假定一次性加载全部结果；移动端在分页读取基础上进一步使用窗口化列表渲染，以降低大结果集导致的 WebView / JS 内存压力。
- 当前恢复能力仍以“恢复快照、结果、进度语义和暂停/运行状态”为主，还没有做到跨进程无缝重连到底层完整运行时对象；若原生 runtime 已丢失，前端会把该任务标记为 `persisted_only` 并提示重新启动。
- Android 构建要求 JDK 24（当前验证环境为 `24.0.2`）；`mobile/android/build.gradle` 会强制校验当前 Gradle JVM，并将 Android 子项目 compile options 统一覆盖为 Java 24 bytecode。
- Android 发布基线为 Capacitor `8.4.0`、Cordova Android `15.0.0`、AGP `9.2.1`、Gradle `9.5.1`、AGP 9 内置 Kotlin（顶层 KGP classpath 固定 `2.4.0`）、SDK platform `android-37.0`、Build Tools `37.0.0`、cmdline-tools `20.0` 和 NDK `29.0.14206865`。
- AndroidX 依赖按最新稳定更新；`androidx.core` 升到 `1.19.0`，因此 compile SDK 同步升到 `android-37.0`。
- `app/capacitor.build.gradle` 等带有 “DO NOT EDIT” 注释的文件由 `pnpm exec cap sync android` 生成；如果模板默认值写 Java 21，不手工编辑生成文件，以顶层 Gradle 的 Java 24 bytecode 覆盖保持一致。AGP 9 已内置 Kotlin 支持，`app/build.gradle` 不再显式应用 `org.jetbrains.kotlin.android`。
