# CFST-GUI 文档索引

本文档是仓库内中文文档入口。它只索引和补充 `docs/` 下的内容，仓库根目录 README 保持不变。

## 快速入口

| 场景 | 文档 |
| --- | --- |
| 查看 GUI、CLI、验证和 Release 命令 | [CLI 指令](./cli.md) |
| 准备开发环境、构建桌面端、WebUI、Android 和 Release | [部署与构建](./deployment.md) |
| 理解配置目录、配置文件、字段含义和默认值 | [配置详解](./configuration.md) |
| 查看 WebUI、Docker、Android、Actions 环境变量 | [Docker 与环境变量](./docker-env.md) |
| 理解 Android 架构、构建输出和桥接机制 | [Android Mobile Architecture](./android-mobile.md) |
| 查看功能链路、Wails/WebUI API、事件和源码定位 | [功能与相关接口文档](./功能与相关接口文档.md) |

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

Linux WebUI 和 Docker Compose 包由统一 Release 脚本生成：

```bash
./scripts/build-release.sh linux
```

## 文档地图

`docs/cli.md` 说明运行模式判定、所有 CLI 兼容参数、前端验证命令、Go 测试命令和 Release 构建入口。

`docs/deployment.md` 说明本地开发环境、桌面构建、Android Debug/Release、Linux WebUI Docker Compose、升级、备份、回滚、GitHub Release 和 GHCR 镜像发布。

`docs/configuration.md` 说明 `storage.json`、`desktop-config.json`、`config.json`、`profiles.json`、`source-profiles.json`、`cfip-log.txt` 和主要配置字段。

`docs/docker-env.md` 集中列出 `CFST_WEBUI_*`、`CFST_GUI_PORTABLE_ROOT`、`CFST_VERSION`、Android toolchain/signing 和 GitHub Actions Secret。

## 事实来源

这些文档基于当前源码整理，主要来源包括 `main.go`、`app.go`、`webui.go`、`storage.go`、`scripts/build-release.sh`、`scripts/build-android-mobile.sh`、`.github/workflows/release.yml`、`.github/workflows/container.yml`、`mobile/android/app/build.gradle` 和 `frontend/package.json`。
