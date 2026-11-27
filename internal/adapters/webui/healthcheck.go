package webui

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

const defaultWebUIHealthcheckAddr = "0.0.0.0:34115"

var webUIHealthcheckClient = &http.Client{Timeout: 5 * time.Second}

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
	hostName := parsed.Hostname()
	port := parsed.Port()
	if hostName == "" || hostName == "0.0.0.0" || hostName == "::" {
		hostName = "127.0.0.1"
	}
	if port != "" {
		parsed.Host = net.JoinHostPort(hostName, port)
	} else {
		parsed.Host = hostName
	}
	if parsed.Scheme == "" {
		parsed.Scheme = "http"
	}
	parsed.Path = "/api/health"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
