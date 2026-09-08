# Android 真机 Debug 包使用与日志读取

本文说明如何安装、运行和排查 Android `Debug APK`，以及如何读取应用落盘的调试日志。

## 适用范围

- 当前只支持 `arm64-v8a` 真机。
- 包名为 `io.github.axuitomo.cfstgui`。
- Debug APK 使用 Gradle debug 签名，仅用于测试和真机调试。
- Debug APK 与正式 Release APK 使用同一包名但签名不同，不能覆盖安装，也不能与正式包同时安装。
- Debug 构建不会进入 GitHub Release 正式资产；统一发布工作流会把它作为 `android-debug` Actions artifact 上传。

## 获取 Debug APK

### 从 GitHub Actions 下载

打开对应 `Release` workflow 运行，在 **Artifacts** 中下载 `android-debug`，解压得到：

```text
cfst-gui-android-arm64-v8a-debug.apk
```

可从任何预览版或正式版对应的 `Release` workflow 运行进入 Artifacts 下载。GitHub Release 页面继续只提供正式签名的 Release APK；Debug APK 应从该次 Actions 运行的 `android-debug` artifact 下载。

### 本地构建

在仓库根目录执行：

```bash
pnpm --dir frontend install
go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20260821190718-4776eadac327
gomobile init
bash scripts/build/build-android-mobile.sh
```

构建完成后，Debug APK 位于：

```text
mobile/android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk
```

如果修改了 Go、前端或 Capacitor 配置，重新运行上面的构建脚本后再安装 APK。只修改 Kotlin 或 Android 资源时，可以在 Android Studio 中直接重新构建。

## 方式一：Android Studio 调试

1. 安装 JDK 24，并把 Android Studio 的 Gradle JDK 设置为 JDK 24。
2. 安装 Android SDK Platform 37、Build Tools 37.0.0、NDK 29.0.14206865 和 Platform-Tools。
3. 按照上一节先生成 Web assets 和 `mobileapi.aar`。
4. Android Studio **只打开 `mobile/android` 目录**，不要把仓库根目录作为 Android 工程打开。
5. 等待 Gradle Sync 完成，选择 `app` 模块的 Debug 配置并连接手机；如果本地已有共享配置，也可以选择 `APP`（`mobile/android/.run/APP.run.xml`）。
6. 在手机上打开“开发者选项”和“USB 调试”，连接手机后确认：

   ```bash
   adb devices
   ```

   设备状态必须是 `device`。

7. 选择手机，点击 **Run** 或 **Debug**。

断点建议优先放在：

- `CfstPlugin.kt`：Capacitor 调用入口和异常边界
- `AndroidStorageState.kt`：运行时目录和存储迁移
- `mobileapi/`：Go 侧命令、事件和桥接追踪

## 方式二：使用 adb 安装和启动

先确认 APK 路径，然后安装：

```bash
adb install mobile/android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk
```

如果设备上已经安装的是同一份 Debug 包，可以使用：

```bash
adb install -r mobile/android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk
```

如果设备上安装的是正式 Release 包，必须先导出配置、结果和需要保留的数据，再卸载正式包。卸载会清除应用数据：

```bash
adb shell am force-stop io.github.axuitomo.cfstgui
adb uninstall io.github.axuitomo.cfstgui
adb install mobile/android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk
```

启动应用：

```bash
adb shell monkey -p io.github.axuitomo.cfstgui 1
```

确认当前安装包和版本：

```bash
adb shell dumpsys package io.github.axuitomo.cfstgui | grep -E "versionName|versionCode|debuggable"
```

Windows PowerShell 可使用：

```powershell
adb shell dumpsys package io.github.axuitomo.cfstgui | Select-String 'versionName|versionCode|debuggable'
```

## 开启调试日志

Android 新配置默认开启探测调试日志，但建议每次调试前确认一次：

1. 打开应用的 **设置**。
2. 展开 **安全与诊断**。
3. 确认 **启用调试日志** 已开启。
4. 运行一次能复现问题的探测或桥接操作。

日志只有在应用初始化并实际执行相关操作后才会产生。已有配置中明确保存的 `probe.debug: false` 会保留，此时需要手动打开开关。

## 日志文件位置

Android 默认把应用运行时目录放在 `getExternalFilesDir(null)`，通常对应：

```text
/sdcard/Android/data/io.github.axuitomo.cfstgui/files/
```

如果外部应用目录不可用，会退回应用私有 `filesDir`。运行时目录中的日志文件为：

```text
logs/cfip-log.txt          # 探测调试日志
logs/error-log.txt         # 错误日志
logs/bridge-debug.log      # Vue / Kotlin / Go 桥接追踪，JSONL 格式
logs/bridge-debug.*.log    # 桥接追踪轮转归档
```

桥接追踪日志的限制是：当前文件达到 4 MiB 时轮转，最多保留 3 个归档文件。桥接追踪写入采用 `Flush` + `Sync`，但它仍是诊断辅助机制，不能替代业务数据持久化。

`导出调试日志` 当前导出的只有 `cfip-log.txt`，`导出诊断包` 包含 `cfip-log.txt`、`error-log.txt` 和运行状态，但两者都不包含 `bridge-debug.log`。需要桥接追踪时，使用后文的 adb 或 Android Studio Device Explorer 直接读取该文件。

## 推荐方式：通过应用导出日志

Android 受分区存储限制，直接从 `/sdcard/Android/data/...` 读取日志在部分系统上会被拒绝。最稳定的方式是使用应用内 SAF 导出：

1. 打开 **设置**。
2. 展开 **安全与诊断**。
3. 点击 **导出调试日志**。
4. 在系统文件选择器中选择一个可写目录，例如 Download 下的专用目录。
5. 返回应用，确认提示“调试日志已导出”。
6. 在电脑上通过 USB、文件管理器或 Android Studio Device Explorer 取出导出的 `.txt` 文件。

导出的默认文件名类似：

```text
cfip-log-20260908-121455.txt
```

应用导出调试日志前会对敏感内容执行脱敏。发送给开发者前仍应人工检查文件，不要连同配置压缩包一起公开，因为配置导出可能包含完整 Token 和凭据。

如需一次性收集更多信息，可点击 **导出诊断包**。诊断包通常包含：

```text
logs/cfip-log.txt
logs/error-log.txt
status/runtime.json
status/scheduler.json
config/config-summary.json
```

诊断包会进行脱敏，但仍应只发送给可信的排查人员。

## 直接读取或拉取日志

如果设备系统允许访问应用外部文件目录，可以尝试：

```bash
adb shell ls -l /sdcard/Android/data/io.github.axuitomo.cfstgui/files/logs/
adb pull /sdcard/Android/data/io.github.axuitomo.cfstgui/files/logs/bridge-debug.log .
adb pull /sdcard/Android/data/io.github.axuitomo.cfstgui/files/logs/cfip-log.txt .
```

如果返回 `Permission denied`、目录不存在或读取到旧目录，不要反复修改权限，改用上面的 **导出调试日志** 或 **导出诊断包**。应用启动时可能已经完成旧目录到当前运行时目录的迁移，最终路径以应用状态和导出结果为准。

## 查看 Logcat

桥接追踪主要写入文件；Android 原生异常和插件边界错误还会写入 Logcat，默认 tag 是 `CfstPlugin`。

先清除旧日志，再启动实时查看：

```bash
adb logcat -c
adb logcat -v threadtime -s CfstPlugin:V AndroidRuntime:E
```

将 Logcat 保存到电脑：

```bash
adb logcat -v threadtime -s CfstPlugin:V AndroidRuntime:E > android-logcat.txt
```

PowerShell 中可以使用：

```powershell
adb logcat -v threadtime -s CfstPlugin:V AndroidRuntime:E | Tee-Object -FilePath android-logcat.txt
```

复现问题后按 `Ctrl+C` 停止采集。若只想查看最近 500 行：

```bash
adb logcat -d -v threadtime -t 500 -s CfstPlugin:V AndroidRuntime:E
```

## 一次完整取证流程

```text
清除 Logcat
   ↓
启动 Debug APK
   ↓
确认“安全与诊断 → 启用调试日志”
   ↓
开始 Logcat 采集
   ↓
只复现一次问题
   ↓
停止 Logcat 采集
   ↓
应用内导出调试日志或诊断包
   ↓
同时保存：APK 版本、设备型号、Android 版本、复现步骤、Logcat、导出文件
```

提交问题时至少提供：

- Debug APK 版本或 Git commit
- 手机型号和 Android 版本
- `adb devices` 中的设备状态
- 简短、可重复的复现步骤
- 导出的调试日志或诊断包
- 对应时间段的 `android-logcat.txt`

不要提供：

- `mobile-config.json` 原文件
- 未检查的完整配置压缩包
- API Token、Bot Token、WebDAV 密码
- 带有认证参数的 URL
