package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var version = "2.1.5"

const defaultWebUIHealthcheckAddr = "0.0.0.0:34115"

var webUIHealthcheckClient = &http.Client{Timeout: 5 * time.Second}

// Run 只保留两个入口：WebUI 容器的健康检查，以及 GUI 启动。
func Run(args []string, resources Resources) {
	// 放在最前面：GUI 与健康检查共用，profile 覆盖整个进程生命周期。
	stopCPUProfile := startCPUProfileFromEnv()
	defer stopCPUProfile()
	setResources(resources)
	if shouldRunWebUIHealthcheck(args) {
		os.Exit(runWebUIHealthcheck(context.Background(), os.Getenv("CFST_WEBUI_ADDR"), webUIHealthcheckClient))
	}
	if shouldRefuseRemovedCLI(args) {
		if dropped := droppedSourceFiles(args); len(dropped) > 0 {
			startupDroppedSources = dropped
		} else {
			refuseRemovedCLI("CFST-GUI 已移除 CLI 模式，请启动图形界面，或使用 Linux WebUI 与 Android 应用。")
			os.Exit(2)
		}
	}
	runGUI()
}

func shouldRunWebUIHealthcheck(args []string) bool {
	return len(args) == 1 && args[0] == "--healthcheck"
}

func runWebUIHealthcheck(ctx context.Context, addr string, client *http.Client) int {
	healthURL := webUIHealthcheckURL(addr)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "webui healthcheck request failed: %v\n", err)
		return 1
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "webui healthcheck failed: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "webui healthcheck returned HTTP %d\n", resp.StatusCode)
		return 1
	}
	return 0
}

func webUIHealthcheckURL(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = defaultWebUIHealthcheckAddr
	}
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}
	parsed, err := url.Parse(addr)
	if err != nil || parsed.Host == "" {
		parsed = &url.URL{Scheme: "http", Host: defaultWebUIHealthcheckAddr}
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	if port != "" {
		parsed.Host = net.JoinHostPort(host, port)
	} else {
		parsed.Host = host
	}
	if parsed.Scheme == "" {
		parsed.Scheme = "http"
	}
	parsed.Path = "/api/health"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func shouldRefuseRemovedCLI(args []string) bool {
	if len(args) == 0 {
		return false
	}
	// 保留 --gui 作为显式 GUI 启动方式，其余参数一律视为已移除的 CLI 调用。
	return args[0] != "--gui"
}
