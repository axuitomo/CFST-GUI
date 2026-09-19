# 发布说明索引

本目录按版本归档 CFST-GUI 的发布说明。每个文件包含对应版本的变更摘要、验证方式和发行资产；最新版本为 [v2.0.0](./v2.0.0.md)。

| 版本 | 变更摘要 |
| --- | --- |
| [v2.0.0](./v2.0.0.md) | 设置页新增分区帮助说明并收紧布局；前端静态资源缓存策略统一（入口 `index.html` no-cache、带哈希资产 immutable）；启动时标注前端来源、缺失前端产物时给出明确修复指引；Windows 桌面/CLI 启动模式拆分随 2.0.0 正式纳入发行资产；新增 vapor-mode 与 frontend-boundary 质量门禁；`wails3 dev` 默认桌面开发配置与孤儿 Vite 端口清理；`docs/reference` 拆分为五个聚焦文档。 |
| [v1.9.8](./v1.9.8.md) | Windows 发行形态改为 NSIS 安装器（桌面版与 CLI 版拆分、代码签名、PE 版本资源）；WebUI 默认只绑定回环地址并强制令牌；`wails3 dev` 默认拉起原生桌面窗口；修复 WebUI 误走桌面 IPC 通道；Android 前台服务修复；工具链基线统一升级（JDK 25、AGP 9.3.2、NDK r30、Wails beta.20、pnpm 12.3.4、vite 8.3.0）。 |
| [v1.9.7](./v1.9.7.md) | Android 在线更新改用安全版本比较；更新响应要求 GitHub Release 提供 `tag_name`；前端 bridge 归一化非对象响应；发布构建目录和脚本按 build/checks/dev 分类。 |
| [v1.9.6](./v1.9.6.md) | 强化代码质量与静态检查体系：新增 golangci-lint、actionlint、stylelint、markdownlint、根目录 ESLint 以及 Android ktlint/detekt 检查，配套补充 `.editorconfig`、`.golangci.yml`、`.markdownlint-cli2.jsonc`、`eslint.config.mjs`、`detekt-baseline.xml` 等配置文件。 |
| [v1.9.5](./v1.9.5.md) | 修复 Android 文件选择、目录授权与持久化存储访问，并收紧更新文件的 FileProvider 暴露范围。 |
| [v1.9.4](./v1.9.4.md) | 迁移桌面端 Wails 集成至 Wails v3 beta.16。 |
| [v1.9.3](./v1.9.3.md) | 修复 Go 间接依赖版本并刷新前端锁文件。 |
| [v1.9.2](./v1.9.2.md) | 优化 MCIS 候选结果复用与探测流程，减少重复 TCP 测量并保留 MCIS 诊断数据。 |
| [v1.9.1](./v1.9.1.md) | MCIS 探针流程支持独立的阶段限制、并发执行与实时进度展示，前端任务状态和暂停、恢复、取消交互同步完善。 |
| [v1.9.0](./v1.9.0.md) | MCIS 搜索探针改用 `GET` 获取 Cloudflare trace，并恢复 TLS 证书校验、`TraceURL` 目标选择和 COLO allow/deny 过滤，失败探针不再进入 Top N。 |
| [v1.8.9](./v1.8.9.md) | Cloudflare DNS 推送区分“无可上传 IP”“全部目标失败”和“部分完成”：分流或组合推送全部目标失败时返回失败状态，部分目标成功时保留 `partial`，定时任务状态、上传通知和 Telegram 通知不再把这些结果改写为普通完成。 |
| [v1.8.8](./v1.8.8.md) | 新增上传结果 Telegram 通知：手动上传、测速后自动上传、定时任务和策略管道自动上传会生成统一通知摘要，支持在设置页配置 Bot Token / Chat ID 并发送测试通知。 |
| [v1.8.7](./v1.8.7.md) | 修复 Windows 安装前 WebView2 Runtime 检测：缺失运行时时先提示并安装依赖，避免安装后首次启动直接失败。 |
| [v1.8.6](./v1.8.6.md) | 修复 Android 前台探测服务重复启动路径：重复手动启动或定时启动不再停止正在运行的前台服务，任务结束时会使用最新 `startId` 正确收尾，避免误杀当前任务或留下残留服务。 |
| [v1.8.5](./v1.8.5.md) | 设置页与输入源页改为自动保存：交互、切页和关闭页面前会尝试静默保存，输入组切换/删除当前输入组前会先补齐当前变更，减少手动保存丢失配置的概率。 |
| [v1.8.4](./v1.8.4.md) | 前端包管理迁移到 pnpm workspace：新增根目录 `package.json`、`pnpm-workspace.yaml`、`.npmrc` 和 `pnpm-lock.yaml`，移除 `frontend/package-lock.json`，CI、脚本、文档和 Release workflow 全部改用 pnpm。 |
| [v1.8.3](./v1.8.3.md) | 前端工具链同步到 Vite 8、Tailwind CSS 4、TypeScript 6、`@vitejs/plugin-vue` 6 和 `vue-tsc` 3；Tailwind 改由 `@tailwindcss/vite` 接入，PostCSS 配置只保留 Autoprefixer，并刷新 `frontend/dist` hashed assets。 |
| [v1.8.2](./v1.8.2.md) | 修复 Android WebView 中输入框被软键盘遮挡的问题：Activity 使用 `adjustResize`，前端按 `visualViewport` 同步可视高度，并在键盘打开时隐藏底部导航。 |
| [v1.8.1](./v1.8.1.md) | 全端应用图标替换为蓝底白色闪电图标，覆盖桌面主图标、托盘图标、Windows 安装器图标、前端 favicon 和 Android 启动器图标。 |
| [v1.8.0](./v1.8.0.md) | 新增桌面、WebUI、Android 共用的运行时清理链路，周期性释放 idle 连接、过期缓存和内存快照，并避免在任务运行中执行重清理。 |
| [v1.7.9](./v1.7.9.md) | 在线更新检查改为只读取 GitHub Releases latest 版本信息，发现新版本后不再在检查阶段拉取更新 manifest 和匹配安装包，降低设置页“检查更新”的等待时间。 |
| [v1.7.8](./v1.7.8.md) | 移除 Go 1.24+ 下已 deprecated 且无效的 `rand.Seed` 初始化，依赖 Go 1.20+ `math/rand` 顶层随机源自动播种，保持现有 IP 取样逻辑不变。 |
| [v1.7.7](./v1.7.7.md) | 工作流运行时按职责拆分 DAG、节点执行、payload 解析和 runtime 状态文件，降低策略管道维护成本。 |
| [v1.7.6](./v1.7.6.md) | DNS 页面拆分为只读记录查询入口，支持按 Zone、当前配置记录名或自定义名称读取 Cloudflare A/AAAA 记录，避免误触发线上记录修改。 |
| [v1.7.5](./v1.7.5.md) | 工作流画布补齐输入源组配置：可从当前绑定配置或已有输入组档案选择输入源，并支持“全部启用输入源”和自定义勾选两种模式。 |
| [v1.7.4](./v1.7.4.md) | 聚焦发行阻塞收口与三端发布面同步，确保桌面端、Android 手机端、Linux WebUI / Docker Compose bundle 和 GHCR 多架构镜像统一发布到 `1.7.4`。 |
| [v1.7.3](./v1.7.3.md) | 聚焦发行前维护体系、质量门禁和 Docker/GHCR 发布链路收口，确保桌面端、Linux WebUI、Android 与容器镜像的默认版本面统一到 `1.7.3`。 |
| [v1.7.2](./v1.7.2.md) | 主要聚焦桌面端与 WebUI 的异步任务链路收口、配置时间显示与保存体验修复，以及发布面默认版本同步。 |
| [v1.7.1](./v1.7.1.md) | 主要聚焦 Android 启动探测闪退修复、在线更新 APK 选择链路加固，以及发布面版本同步。 |
| [v1.7](./v1.7.md) | 以 `v1.6` 为基准，重点收敛 Android 长任务稳定性、任务恢复/结果落盘链路、桌面端与移动端设置页信息架构，以及 Linux WebUI / Docker 与多架构发布面的统一。 |
| [v1.6](./v1.6.md) | 以 `v1.5` 为基准，重点收敛统一上传筛选、输入源档案与共享核心，增强更新下载链路，并把桌面端、Linux WebUI/Docker Compose 与 Android 的公开版本面统一提升到 `1.6`。 |
| [v1.5](./v1.5.md) | 以 `v1.4` 为基准，重点补齐桌面端、Linux WebUI/Docker 和 Android 三端发布链路，新增自动调度、GitHub 结果导出、CSV 编码控制、配置兼容净化和更完整的文档说明。 |
| [v1.4](./v1.4.md) | 以 `v1.3` 为基准，重点修复 Android 版本号显示异常，并补齐桌面端与安卓端共用的测速参数、设置页和发布元数据一致性。 |
| [v1.3](./v1.3.md) | 相比 `v1.2` 重点提升测速准确性、配置迁移和移动端体验：文件测速新增 HTTP Range 分片并发、自动续连、HTTP/3 优先与协议回退，输入源支持 IP/CIDR/域名混合解析、注释清洗、MICS 抽样和输入源档案；新增配置压缩包导入导出、本地备份/还原、WebDAV 备份/还原和储存目录健康检查。 |

> 说明：本索引由 `docs/release-notes/` 下的版本文件整理而成，按版本号从新到旧排列；完整变更、验证命令和发行资产见各版本文件。
