//go:build webui

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/runtimecleanup"
)

const defaultWebUIAddr = "0.0.0.0:34115"

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
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("CFST WebUI listening on http://%s", addr)
	return server.ListenAndServe()
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
		path := strings.TrimPrefix(filepath.Clean("/"+r.URL.Path), "/")
		if path == "." || path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(staticFS, path); err != nil {
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
		if provided != token {
			writeWebUIJSON(w, http.StatusUnauthorized, map[string]any{
				"message": "WebUI 访问令牌无效或缺失。",
				"ok":      false,
			})
			return
		}
		next.ServeHTTP(w, r)
	})
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
