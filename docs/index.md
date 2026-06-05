# CFST-GUI 文档索引

README 是项目快速入口；本文档索引 `docs/` 下的深入说明，用于查找运行、部署、配置、接口和移动端细节。

## 快速入口

| 场景 | 文档 |
| --- | --- |
| 快速了解项目能力、运行形态和数据文件 | [README](../README.md) |
| 面向普通用户了解产品定位、发行资产和安装建议 | [产品介绍](../介绍产品.md) |
| 理解仓库分层、代码归位和跨端契约约束 | [架构约束](./architecture-constraints.md) |
| 查看 GUI、CLI、验证和 Release 命令 | [CLI 指令](./cli.md) |
| 准备开发环境、构建桌面端、WebUI、Android 和 Release | [部署与构建](./deployment.md) |
| 理解配置目录、字段默认值、旧配置兼容和字段净化 | [配置详解](./configuration.md) |
| 配置 Cloudflare DNS 读取/推送 API Token 最小权限和排错 | [Cloudflare API Token 权限设置教程](./cloudflare-api-token.md) |
| 配置 GitHub 结果导出的 PAT 最小权限和排错 | [GitHub PAT 权限设置教程](./github-pat.md) |
| 查看 WebUI、Docker、Android、Actions 环境变量 | [Docker 与环境变量](./docker-env.md) |
| 理解 Android 架构、SAF 文件访问、构建输出和桥接机制 | [Android Mobile Architecture](./android-mobile.md) |
| 查看统一上传筛选、Cloudflare/GitHub 结果上传设计 | [上传工作流设计](./upload-workflow-design.md) |
| 查看功能链路、Wails/WebUI/Android API、事件和源码定位 | [功能与相关接口文档](./功能与相关接口文档.md) |
| 查看 v1.7.6 变更摘要、验证命令和发行资产 | [v1.7.6 发布说明](./release-notes/v1.7.6.md) |

## 最短启动

桌面开发推荐先安装 Wails 和前端依赖：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
cd frontend
npm install
cd ..
wails dev
```

无参数运行时默认进入桌面 GUI，也可以显式使用 `--gui`：

```bash
go run .
go run . --gui
```

CLI 兼容 CFST 参数，推荐显式带 `--cli`：

```bash
go run . --cli -f ip.txt -o result.csv
```

Linux WebUI bundle 由统一 Release 脚本生成，既可用于 Docker Compose，也可直接本地运行：

```bash
./scripts/build-release.sh linux
./scripts/build-release.sh linux-amd64
./scripts/build-release.sh linux-arm64
```

## 文档地图

`docs/cli.md` 说明运行模式判定、CLI 兼容参数、前端验证命令、Go 测试命令和 Release 构建入口。

`docs/architecture-constraints.md` 说明仓库分层、Go/前端/Android 代码归位、跨端契约和验证入口约束。

`docs/deployment.md` 说明本地开发环境、桌面构建、Android Debug/Release、Linux WebUI 的 Docker Compose / 本地运行、升级、备份、回滚、GitHub Release 和 GHCR 镜像发布。

`docs/configuration.md` 说明 `storage.json`、`desktop-config.json`、`mobile-config.json`、`source-profiles.json`、`cfip-log.txt`、主要配置字段、默认值、旧配置兼容和字段净化时机。

`docs/cloudflare-api-token.md` 说明 Cloudflare DNS 读取/推送需要的 API Token 最小权限、Zone ID 获取、应用内填写位置、常见报错和安全边界。

`docs/github-pat.md` 说明 GitHub 结果导出需要的 fine-grained PAT 最小权限、应用内填写位置、常见报错和维护者发布权限边界。

`docs/docker-env.md` 集中列出 `CFST_WEBUI_*`、`CFST_GUI_PORTABLE_ROOT`、`CFST_VERSION`、Android toolchain/signing 和 GitHub Actions Secret。

`docs/android-mobile.md` 说明 Android Capacitor + gomobile 架构、SAF 文件选择、APK 构建输出、在线更新和移动端桥接注意事项。

`docs/upload-workflow-design.md` 说明当前统一上传筛选、Cloudflare/GitHub 目标 Top N、测速后自动推送、兼容字段和后续扩展方向。

`docs/功能与相关接口文档.md` 说明功能链路、三端 bridge 能力矩阵、WebUI `/api/*`、`desktop:probe` 事件、配置归档、WebDAV、Cloudflare DNS 和源码定位。

`docs/release-notes/v1.7.6.md` 说明 v1.7.6 的 DNS 读取拆分、Cloudflare/GitHub 自动推送、配置结构迁移、结果页分页筛选和发行资产。

## 事实来源

这些文档基于当前源码整理，主要来源包括 `main.go`、`resources.go`、`internal/app/run.go`、`internal/app/app.go`、`internal/app/app_archive.go`、`internal/app/webui.go`、`internal/app/storage.go`、`frontend/src/lib/bridge.ts`、`mobileapi/`、`mobile/android/app/src/main/java/io/github/axuitomo/cfstgui/CfstPlugin.java`、`scripts/build-release.sh`、`scripts/build-android-mobile.sh`、`.github/workflows/release.yml`、`.github/workflows/container.yml`、`mobile/android/app/build.gradle` 和 `frontend/package.json`。
