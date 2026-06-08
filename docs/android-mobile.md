# Android Mobile Architecture

桌面端继续使用 Wails。Android 端使用 Vue + Capacitor + gomobile AAR，并通过 `mobileapi` 包复用 Go 探测核心。

## Build

```bash
go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20260410095206-2cfb76559b7b
gomobile init
bash scripts/build-android-mobile.sh
```

调试构建脚本会执行：

1. `frontend` 生产构建。
2. `npx cap sync android` 同步 Web assets 到 `mobile/android`。
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

当前仓库通过 `-Wl,-z,max-page-size=16384` 与 `-Wl,-z,common-page-size=16384` 一起实现这一点；`common-page-size=16384` 不能删除，否则 16KB / 4KB 双兼容会退化。每次 Debug / Release 构建结束后，脚本还会自动执行：

```bash
bash scripts/check-android-page-alignment.sh \
  mobile/android/app/libs/mobileapi.aar \
  mobile/android/app/build/outputs/apk/debug/app-universal-debug.apk
```

验收重点是：

- `llvm-readelf -l` 看到 `libgojni.so` 的 `LOAD` 段 `Align` 为 `0x4000`
- `zipalign -c -P 16 -v 4` 校验 APK 通过

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

Android 在线更新通过 GitHub Releases latest + `cfst-gui-update-manifest.json` 选择最匹配当前 ABI 的 APK；旧版客户端仍会回退到 `cfst-gui-android-release.apk`。下载到 app 私有 `updates/` 目录后使用 `FileProvider` 拉起系统安装确认。新旧 APK 必须使用同一签名证书。

## Bridge

前端统一调用 `frontend/src/lib/bridge.ts`：

- Wails bridge 存在时优先走桌面端 `window.go.app.App`，并兼容旧生成物的 `window.go.main.App`。
- Android native 环境且无 Wails bridge 时走 Capacitor `Cfst` plugin。

Android plugin 位于 `mobile/android/app/src/main/java/io/github/axuitomo/cfstgui/CfstPlugin.java`，通过 gomobile 生成的 `mobileapi.Service` 调 Go，并把 probe 事件通过 `desktop:probe` 回传给前端。

除单任务 probe 外，Android bridge 现在也支持策略管道相关方法：`LoadPipelineProfiles`、`SavePipelineProfiles`、`SavePipelineProfile`、`DeletePipelineProfile`、`RunPipeline`、`StartPipeline`、`CancelPipeline`、`GetPipelineSnapshot`、`ListPipelineResults`。前端仍统一走 `frontend/src/lib/bridge.ts` 做三端归一化。

当前 Android 长任务执行链路已经调整为：

1. 前端提交 `RunProbe` 或 `StartPipeline` 后，Capacitor plugin 会先返回 accepted 响应。
2. `ProbeForegroundService` 在前台服务中继续执行真实长任务；单次 probe 调用同步 `RunProbe`，策略管道调用同步 `RunPipeline`，并共用同一条事件流来细粒度更新系统通知。
3. Go 侧 `mobileapi` 在任务运行过程中持续写入任务快照，任务完成后额外持久化结果行；快照会区分 `active_runtime`、`paused_runtime` 和 `persisted_only`，避免把失联旧会话误判成仍在运行。
4. 前端启动后会先查询 Android 原生运行时状态：若探测任务仍附着在前台服务/Go runtime 上，则自动重新接入当前任务；若只剩快照与已落盘结果，则恢复结果视图并明确提示“当前不可无缝重连”。
5. 结果页在移动端优先使用窗口化列表渲染，并结合分页读取结果，而不是一次性把全量结果灌进 WebView。
6. 设置页的“异常保护”区块会展示 Android 电池优化状态，并提供“申请豁免 / 系统电池设置 / 应用详情”入口，用于引导用户把 CFST 加入厂商白名单和后台运行放行列表。
7. Android 自动调度由 `SchedulerWorker` 基于 WorkManager 注册下一次运行；触发后会拉起 `ProbeForegroundService.startScheduledIntent()`，再由 Go 侧 `runScheduledProbe` 读取保存配置并执行单次测速。受系统省电、厂商后台策略和 Doze 影响，实际触发时间可能晚于配置时间。

## Notes

- Android 配置文件实际由 app 私有运行时目录中的 `mobile-config.json` 读取；应用存储不再使用 SAF 存储镜像。
- `LoadConfig` / `SaveConfig`、配置归档导入导出和 WebDAV 备份现在都会一并携带 `pipeline_profiles`，对应文件为 app 私有运行时目录中的 `pipeline-profiles.json`。
- CSV、测速文件和调试日志通过已持久授权的 SAF 导出目录写入；未选择导出目录或权限失效时会明确失败并要求重新选择。
- Android 任务快照和分页结果缓存默认保存在 app 私有运行时目录下的 `tasks/`，用于进程重建后的恢复读取。
- 输入源文件和配置导入通过 SAF 文件选择器完成，输入源文件会复制到 app 私有 `imports/` 目录供 Go 侧读取。
- Android SAF 持久化权限只用于导出目录，不参与配置读取或应用数据持久化。
- `probe.failed` / `probe.completed` 事件会携带 `failure_stage` 与 `trace_diagnostics`，便于前端展示更接近真实原因的错误摘要；Android 原生 bridge / storage fallback 会额外写入 `Logcat`，默认 tag 为 `CfstPlugin`。
- 当前 Android 的 `StartPipeline` 已复用 `ProbeForegroundService` 承载策略管道执行与通知更新；`RunPipeline` 仍保留同步 bridge 语义，便于桌面 / WebUI / native 三端继续共用同一套接口。
- Android 调度当前固定为单次测速模式；前端会隐藏工作流定时模式并把 `scheduler.run_mode` 归一为 `probe`。桌面和 WebUI 仍支持 `pipeline` 定时工作流。
- 当前 `CancelProbe` 会在阶段边界生效，底层测速阶段运行中不会被强制中断。
- 结果页不再假定一次性加载全部结果；移动端在分页读取基础上进一步使用窗口化列表渲染，以降低大结果集导致的 WebView / JS 内存压力。
- 当前恢复能力仍以“恢复快照、结果、进度语义和暂停/运行状态”为主，还没有做到跨进程无缝重连到底层完整运行时对象；若原生 runtime 已丢失，前端会把该任务标记为 `persisted_only` 并提示重新启动。
- Android 构建要求 JDK 24；`mobile/android/build.gradle` 会强制校验当前 Gradle JVM 并将 Android 子项目 compile options 覆盖为 Java 24。
