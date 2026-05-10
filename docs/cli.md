# CLI 指令

本文档整理当前仓库可直接使用的 GUI、CLI、验证和 Release 命令。命令示例假设工作目录为仓库根目录。

## 运行模式

程序入口在 `main.go`。无参数时进入 Wails 桌面 GUI；第一个参数不是 `--gui` 时进入 CLI；第一个参数为 `--cli` 时会先移除该标记再解析 CFST 参数。

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
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
cd frontend
npm install
cd ..
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
go run . --cli -url https://speed.cloudflare.com/__down?bytes=10000000 -host cf.example.com -sni cf.example.com -ua "Mozilla/5.0 ..."
```

## CFST 兼容参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-n` | `200` | 延迟测速线程数，最大会归一化到 `1000`。 |
| `-t` | `4` | 单个 IP 延迟测速次数，最少 `2`。 |
| `-dn` | `10` | 保留参数；当前不再限制下载测速数量。 |
| `-dt` | `10` | 单个 IP 下载测速最长时间，单位秒。 |
| `-tp` | `443` | 延迟测速和下载测速端口。 |
| `-url` | `https://speed.cloudflare.com/__down?bytes=10000000` | 文件测速地址；CLI 会从该 URL 推导 `/cdn-cgi/trace` 追踪地址。 |
| `-ua` | 内置 Firefox UA | 自定义请求 User-Agent。 |
| `-host` | 空 | 强制覆盖 HTTP Host 头。 |
| `-sni` | 空 | 强制覆盖 TLS SNI。 |
| `-debug-capture` | 空 | 调试模式下把实际拨号目标改到指定地址。 |
| `-tls-insecure` | `true` | 忽略 TLS 证书校验；需要关闭时传 `-tls-insecure=false`。 |
| `-httping` | `false` | 使用 HTTPing 模式做延迟测速。 |
| `-httping-code` | `0` | HTTPing 有效状态码；`0` 表示不按状态码筛选，设置 `100-599` 才启用精确状态码过滤。 |
| `-cfcolo` | 空 | HTTPing 模式下按 IATA 机场码或地区码过滤，英文逗号分隔。 |
| `-tl` | `9999` | 平均延迟上限，单位 ms。 |
| `-tll` | `0` | 平均延迟下限，单位 ms。 |
| `-tlr` | `0.15` | 丢包率上限，范围 `0.00` 到 `1.00`。 |
| `-sl` | `0` | 下载速度下限，单位 MB/s。 |
| `-p` | `10` | 终端显示结果数量；为 `0` 时不显示结果直接退出。 |
| `-f` | `ip.txt` | IP 段数据文件路径。 |
| `-ip` | 空 | 直接指定 IP/CIDR，英文逗号分隔。 |
| `-o` | `result.csv` | CSV 输出文件；传空字符串可不写文件。 |
| `-dd` | `false` | 禁用下载测速，结果按延迟排序。 |
| `-allip` | `false` | IPv4 段内测速全部 IP，而不是每个 `/24` 随机一个。 |
| `-debug` | `false` | 输出更多调试日志，并写入 `cfip-log.txt`。 |
| `-v` | `false` | 打印版本并检查 GitHub Releases 更新。 |
| `-h` | `false` | 打印帮助。 |

## 前端与验证

前端命令在 `frontend/` 目录执行：

```bash
cd frontend
npm run typecheck
npm run build
```

Go 侧测试在仓库根目录执行：

```bash
go test ./...
```

## Release 命令

统一构建脚本位于 `scripts/build-release.sh`，默认目标是 `all`：

```bash
./scripts/build-release.sh
./scripts/build-release.sh all
```

也可以按目标单独构建：

```bash
./scripts/build-release.sh windows
./scripts/build-release.sh linux
./scripts/build-release.sh darwin-amd64
./scripts/build-release.sh darwin-arm64
./scripts/build-release.sh android
./scripts/build-release.sh manifest
```

Android Release 目标需要先提供签名环境变量，详见 [Docker 与环境变量](./docker-env.md)。
