//go:build webui

package app

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/runtimecleanup"
)

// defaultWebUIAddr 默认只绑定回环地址：WebUI 未设置 CFST_WEBUI_TOKEN 时不做鉴权，
// 因此绝不能在未鉴权的情况下默认对外暴露。需要局域网/容器访问时显式设置
// CFST_WEBUI_ADDR（例如 0.0.0.0:34115），此时必须同时提供令牌。
const defaultWebUIAddr = "127.0.0.1:34115"

type webUIFileEntry struct {
	IsDir   bool   `json:"is_dir"`
	ModTime string `json:"mod_time"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
}

func runGUI() {
	if err := runWebUI(); err != nil {
		log.Fatal(err)
	}
}

func runWebUI() error {
	app := NewApp()
	ctx := context.Background()
	app.startup(ctx)

	addr := strings.TrimSpace(os.Getenv("CFST_WEBUI_ADDR"))
	if addr == "" {
		addr = defaultWebUIAddr
	}

	// 非回环地址上不接受“无令牌”运行：空令牌等同于关闭鉴权，一旦绑定 0.0.0.0
	// 就等于把配置接口、文件接口和诊断接口全部开放给网络，因此直接拒绝启动。
	if strings.TrimSpace(os.Getenv("CFST_WEBUI_TOKEN")) == "" && !webUIHostIsLoopback(addr) {
		return fmt.Errorf("CFST_WEBUI_ADDR=%s 绑定非回环地址时必须设置 CFST_WEBUI_TOKEN（可用 openssl rand -hex 24 生成）", addr)
	}

	if runtimeResources.FrontendAssets == nil {
		return fmt.Errorf("frontend assets not configured")
	}
	staticFS, err := fs.Sub(runtimeResources.FrontendAssets, "frontend/dist")
	if err != nil {
		return fmt.Errorf("frontend assets not found: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", app.handleWebUIHealth)
	mux.Handle("/api/platform/", app.webUIAuth(http.HandlerFunc(app.handleWebUIPlatformCommand)))
	mux.Handle("/api/command/", app.webUIAuth(http.HandlerFunc(app.handleWebUICommand)))
	mux.Handle("/api/events/probe", app.webUIAuth(http.HandlerFunc(app.handleWebUIProbeEvents)))
	mux.Handle("/api/files/list", app.webUIAuth(http.HandlerFunc(app.handleWebUIFileList)))
	mux.Handle("/api/files/download", app.webUIAuth(http.HandlerFunc(app.handleWebUIFileDownload)))
	mux.Handle("/", webUISPAHandler(staticFS))

	server := &http.Server{
		Addr:              addr,
		Handler:           webUIRequestLog(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("CFST WebUI listening on http://%s", addr)
	return server.ListenAndServe()
}

// webUIRequestLog 记录每个 WebUI 请求的方法、路径与响应状态，用于诊断客户端
// 调用错误（如 method not allowed 405）与服务问题。只记录方法、路径与状态码，
// 不读取请求体或查询串，避免把令牌等敏感配置写进日志。
func webUIRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &webUIStatusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if rec.status >= 400 || r.URL.Path == "/" {
			log.Printf("webui %s %s -> %d", r.Method, r.URL.Path, rec.status)
		}
	})
}

type webUIStatusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *webUIStatusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Flush 透传 http.Flusher，保证 /api/events/probe 的 SSE 流式响应不受影响。
func (w *webUIStatusRecorder) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (a *App) handleWebUICommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	command := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/command/"), "/")
	if command == "" {
		writeWebUIError(w, http.StatusBadRequest, errors.New("missing command"))
		return
	}
	if command == "runtime.status" && !webUIRuntimeDiagnosticsAllowed(r) {
		result := appcore.NewCommandResult("RUNTIME_DIAGNOSTICS_LOCAL_ONLY", map[string]any{
			"diagnostics_enabled": runtimecleanup.DiagnosticsEnabled(),
			"remote_enabled":      runtimecleanup.DiagnosticsRemoteEnabled(),
			"token_required":      strings.TrimSpace(os.Getenv("CFST_WEBUI_TOKEN")) == "",
		}, "运行时诊断默认只允许本机访问。", false, nil, nil)
		writeWebUIJSON(w, http.StatusOK, result)
		return
	}
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		writeWebUIError(w, http.StatusBadRequest, err)
		return
	}
	result := a.Invoke(command, string(raw))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, result)
}

func webUISPAHandler(staticFS fs.FS) http.Handler {
	files := http.FileServer(http.FS(staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// path.Clean（而非 filepath.Clean）：fs.FS 始终使用 "/" 分隔，
		// Windows 下 filepath.Clean 会产生 "\" 导致静态资源全部 fallback 到 index.html。
		cleanPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if cleanPath == "." || cleanPath == "" {
			cleanPath = "index.html"
		}
		if _, err := fs.Stat(staticFS, cleanPath); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		files.ServeHTTP(w, r)
	})
}

func (a *App) handleWebUIHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeWebUIJSON(w, http.StatusOK, map[string]any{
		"auth_required": strings.TrimSpace(os.Getenv("CFST_WEBUI_TOKEN")) != "",
		"ok":            true,
		"service":       "cfst-webui",
		"version":       appVersion(),
	})
}

func (a *App) webUIAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(os.Getenv("CFST_WEBUI_TOKEN"))
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		provided := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if provided == "" {
			provided = strings.TrimSpace(r.URL.Query().Get("token"))
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			writeWebUIJSON(w, http.StatusUnauthorized, map[string]any{
				"message": "WebUI 访问令牌无效或缺失。",
				"ok":      false,
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// webUIHostIsLoopback 判断监听地址是否只面向本机。空主机名（如 ":34115"）等价于
// 绑定全部网卡，按非回环处理。
func webUIHostIsLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func webUIRequestFromLoopback(r *http.Request) bool {
	if r == nil {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	return ip != nil && ip.IsLoopback()
}

func webUIRuntimeDiagnosticsAllowed(r *http.Request) bool {
	if webUIRequestFromLoopback(r) {
		return true
	}
	return runtimecleanup.DiagnosticsRemoteEnabled() && strings.TrimSpace(os.Getenv("CFST_WEBUI_TOKEN")) != ""
}

func (a *App) handleWebUIPlatformCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	command := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/platform/"), "/")
	payload, _, err := readWebUIPayload(r)
	if err != nil {
		writeWebUIError(w, http.StatusBadRequest, err)
		return
	}
	var result any
	switch command {
	case "GetAppInfo":
		result = a.GetAppInfo()
	case "CheckForUpdates":
		result = a.CheckForUpdates(payload)
	case "DownloadAndInstallUpdate":
		result = a.DownloadAndInstallUpdate(payload)
	case "OpenReleasePage":
		result = appcore.NewCommandResult("RELEASE_OPENED", map[string]any{"release_url": releasePageURL}, "已准备打开发行页。", true, nil, nil)
	case "OpenLogDirectory":
		result = a.OpenLogDirectory(payload)
	default:
		writeWebUIJSON(w, http.StatusNotFound, appcore.NewCommandResult("PLATFORM_COMMAND_UNKNOWN", nil, fmt.Sprintf("unknown platform command: %s", command), false, nil, nil))
		return
	}
	writeWebUIJSON(w, http.StatusOK, result)
}

func readWebUIPayload(r *http.Request) (map[string]any, []byte, error) {
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		return nil, nil, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]any{}, raw, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, raw, err
	}
	return payload, raw, nil
}

func (a *App) handleWebUIProbeEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	lastEventID := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	if lastEventID == "" {
		lastEventID = strings.TrimSpace(r.URL.Query().Get("last_event_id"))
	}
	ch, replay, unsubscribe := a.eventHub.subscribeAfter(lastEventID)
	defer unsubscribe()
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	writeEvent := func(event appcore.ProbeEvent) bool {
		raw, err := json.Marshal(event)
		if err != nil {
			return true
		}
		if _, err := fmt.Fprintf(w, "id: %s\ndata: %s\n\n", encodeWebUIEventID(event), raw); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	for _, event := range replay {
		if !writeEvent(event) {
			return
		}
	}

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			if !writeEvent(event) {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (a *App) handleWebUIFileList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	target, err := webUIAllowedPath(r.URL.Query().Get("path"))
	if err != nil {
		writeWebUIError(w, http.StatusBadRequest, err)
		return
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		writeWebUIError(w, http.StatusBadRequest, err)
		return
	}
	files := make([]webUIFileEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, webUIFileEntry{
			IsDir:   entry.IsDir(),
			ModTime: info.ModTime().Format(time.RFC3339),
			Name:    entry.Name(),
			Path:    filepath.Join(target, entry.Name()),
			Size:    info.Size(),
		})
	}
	slices.SortFunc(files, func(a, b webUIFileEntry) int {
		if a.IsDir != b.IsDir {
			if a.IsDir {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	writeWebUIJSON(w, http.StatusOK, map[string]any{
		"entries": files,
		"path":    target,
		"roots":   webUIAllowedRoots(),
	})
}

func (a *App) handleWebUIFileDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	target, err := webUIAllowedPath(r.URL.Query().Get("path"))
	if err != nil {
		writeWebUIError(w, http.StatusBadRequest, err)
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		writeWebUIError(w, http.StatusBadRequest, err)
		return
	}
	if info.IsDir() {
		writeWebUIError(w, http.StatusBadRequest, errors.New("不能下载目录"))
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(target)))
	http.ServeFile(w, r, target)
}

func webUIAllowedPath(rawPath string) (string, error) {
	roots := webUIAllowedRoots()
	if len(roots) == 0 {
		return "", errors.New("未配置 WebUI 可访问目录")
	}
	if strings.TrimSpace(rawPath) == "" {
		return roots[0], nil
	}
	target, err := filepath.Abs(filepath.Clean(rawPath))
	if err != nil {
		return "", err
	}
	for _, root := range roots {
		rel, err := filepath.Rel(root, target)
		if err == nil && (rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")) {
			return target, nil
		}
	}
	return "", fmt.Errorf("路径不在 WebUI 允许访问范围内: %s", rawPath)
}

func webUIAllowedRoots() []string {
	values := []string{"/data", storageRoot()}
	for _, raw := range strings.FieldsFunc(os.Getenv("CFST_WEBUI_ALLOWED_ROOTS"), func(r rune) bool {
		return r == ',' || r == ':'
	}) {
		values = append(values, raw)
	}
	seen := make(map[string]struct{})
	roots := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		abs, err := filepath.Abs(filepath.Clean(value))
		if err != nil {
			continue
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		roots = append(roots, abs)
	}
	return roots
}

func writeWebUIError(w http.ResponseWriter, status int, err error) {
	writeWebUIJSON(w, status, map[string]any{
		"message": err.Error(),
		"ok":      false,
	})
}

func writeWebUIJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
