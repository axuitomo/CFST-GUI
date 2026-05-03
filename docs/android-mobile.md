# Android Mobile Architecture

桌面端继续使用 Wails。Android 端使用 Vue + Capacitor + gomobile AAR，并通过 `mobileapi` 包复用 Go 探测核心。

## Build

```bash
go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20260410095206-2cfb76559b7b
gomobile init
./scripts/build-android-mobile.sh
```

调试构建脚本会执行：

1. `frontend` 生产构建。
2. `npx cap sync android` 同步 Web assets 到 `mobile/android`。
3. `gomobile bind -target=android/arm64,android/arm` 生成 `mobile/android/app/libs/mobileapi.aar`。
4. `mobile/android/gradlew assembleDebug` 输出 ABI split APK。

Release 发行包由仓库根目录的统一脚本生成：

```bash
export CFST_ANDROID_KEYSTORE=/absolute/path/release.jks
export CFST_ANDROID_KEYSTORE_PASSWORD=...
export CFST_ANDROID_KEY_ALIAS=...
export CFST_ANDROID_KEY_PASSWORD=...
./scripts/build-release.sh
```

也可以只构建 Android 资产：`./scripts/build-release.sh android`。完整 GitHub Release 由 `.github/workflows/release.yml` 分平台构建桌面和 Android 资产，再集中生成 `cfst-gui-update-manifest.json`。

Release 签名只从环境变量读取，不把 keystore 或密码写入仓库。

## Outputs

Debug APK 输出在：

- `mobile/android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk`
- `mobile/android/app/build/outputs/apk/debug/app-armeabi-v7a-debug.apk`
- `mobile/android/app/build/outputs/apk/debug/app-universal-debug.apk`

发行版最终只保留一个 Android APK，并参与统一更新 manifest：

- `build/release/android/cfst-gui-android-release.apk`
- `build/release/cfst-gui-update-manifest.json`

`arm64-v8a` 是 Android 发布必选 ABI，`armeabi-v7a` 用于兼容旧设备。

Android 在线更新通过 GitHub Releases latest + `cfst-gui-update-manifest.json` 找到 `cfst-gui-android-release.apk`，下载到 app 私有 `updates/` 目录后使用 `FileProvider` 拉起系统安装确认。新旧 APK 必须使用同一签名证书。

## Bridge

前端统一调用 `frontend/src/lib/bridge.ts`：

- Wails bridge 存在时走桌面端 `window.go.main.App`。
- Android native 环境且无 Wails bridge 时走 Capacitor `Cfst` plugin。

Android plugin 位于 `mobile/android/app/src/main/java/io/github/axuitomo/cfstgui/CfstPlugin.java`，通过 gomobile 生成的 `mobileapi.Service` 调 Go，并把 probe 事件通过 `desktop:probe` 回传给前端。

## Notes

- Android 配置文件写入 app 私有目录 `mobile-config.json`。
- CSV 默认导出到 app 私有目录下的 `exports/`；用户选择系统导出文件时通过 SAF `ACTION_CREATE_DOCUMENT` 写入目标 URI。
- 输入源文件和配置导入通过 SAF 文件选择器完成，输入源文件会复制到 app 私有 `imports/` 目录供 Go 侧读取。
- 当前 `CancelProbe` 会在阶段边界生效，底层测速阶段运行中不会被强制中断。
- Android 构建要求 JDK 24；`mobile/android/build.gradle` 会强制校验当前 Gradle JVM 并将 Android 子项目 compile options 覆盖为 Java 24。
