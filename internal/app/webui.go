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
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
	mux.Handle("/api/app/", app.webUIAuth(http.HandlerFunc(app.handleWebUIAppMethod)))
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

func (a *App) handleWebUIAppMethod(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	method := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/app/"), "/")
	payload, raw, err := readWebUIPayload(r)
	if err != nil {
		writeWebUIError(w, http.StatusBadRequest, err)
		return
	}

	result, err := a.invokeWebUIAppMethod(method, payload, raw)
	if err != nil {
		writeWebUIError(w, http.StatusBadRequest, err)
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

func (a *App) invokeWebUIAppMethod(method string, payload map[string]any, raw []byte) (any, error) {
	switch method {
	case "LoadDesktopConfig":
		return a.LoadDesktopConfig(), nil
	case "GetAppInfo":
		return a.GetAppInfo(), nil
	case "CheckForUpdates":
		return a.CheckForUpdates(payload), nil
	case "DownloadAndInstallUpdate":
		return a.DownloadAndInstallUpdate(payload), nil
	case "OpenReleasePage":
		return desktopCommandResult("RELEASE_OPENED", map[string]any{"release_url": releasePageURL}, "已准备打开发行页。", true, nil, nil), nil
	case "ListCloudflareDNSRecords":
		return a.ListCloudflareDNSRecords(payload), nil
	case "LoadSchedulerStatus":
		return a.LoadSchedulerStatus(), nil
	case "TestGitHubExport":
		return a.TestGitHubExport(payload), nil
	case "ExportResultsCSV":
		return a.ExportResultsCSV(payload), nil
	case "ExportResultsToGitHub":
		return a.ExportResultsToGitHub(payload), nil
	case "SaveDesktopConfig":
		return a.SaveDesktopConfig(payload), nil
	case "LoadDesktopDraft":
		return a.LoadDesktopDraft(), nil
	case "SaveDesktopDraft":
		return a.SaveDesktopDraft(payload), nil
	case "DiscardDesktopDraft":
		return a.DiscardDesktopDraft(payload), nil
	case "SetStorageDirectory":
		return a.SetStorageDirectory(payload), nil
	case "CheckStorageHealth":
		return a.CheckStorageHealth(payload), nil
	case "ExportConfig":
		return a.ExportConfig(payload), nil
	case "ExportConfigArchive":
		return a.ExportConfigArchive(payload), nil
	case "ImportConfigArchive":
		return a.ImportConfigArchive(payload), nil
	case "TestWebDAV":
		return a.TestWebDAV(payload), nil
	case "BackupConfigToWebDAV":
		return a.BackupConfigToWebDAV(payload), nil
	case "RestoreConfigFromWebDAV":
		return a.RestoreConfigFromWebDAV(payload), nil
	case "BackupCurrentConfig":
		return a.BackupCurrentConfig(payload), nil
	case "LoadProfiles":
		return a.LoadProfiles(), nil
	case "LoadSourceProfiles":
		return a.LoadSourceProfiles(), nil
	case "SaveCurrentProfile":
		return a.SaveCurrentProfile(payload), nil
	case "UpdateCurrentProfile":
		return a.UpdateCurrentProfile(payload), nil
	case "SaveSourceProfile":
		return a.SaveSourceProfile(payload), nil
	case "UpdateCurrentSourceProfile":
		return a.UpdateCurrentSourceProfile(payload), nil
	case "SaveSourceProfileStore":
		return a.SaveSourceProfileStore(payload), nil
	case "SwitchProfile":
		return a.SwitchProfile(payload), nil
	case "SwitchSourceProfile":
		return a.SwitchSourceProfile(payload), nil
	case "DeleteProfile":
		return a.DeleteProfile(payload), nil
	case "DeleteSourceProfile":
		return a.DeleteSourceProfile(payload), nil
	case "PreviewDesktopSource":
		var typed DesktopSourcePreviewPayload
		if err := json.Unmarshal(raw, &typed); err != nil {
			return nil, err
		}
		return a.PreviewDesktopSource(typed), nil
	case "FetchDesktopSource":
		var typed DesktopSourcePreviewPayload
		if err := json.Unmarshal(raw, &typed); err != nil {
			return nil, err
		}
		return a.FetchDesktopSource(typed), nil
	case "LoadColoDictionaryStatus":
		return a.LoadColoDictionaryStatus(), nil
	case "UpdateColoDictionary":
		return a.UpdateColoDictionary(payload), nil
	case "ProcessColoDictionary":
		return a.ProcessColoDictionary(payload), nil
	case "PushCloudflareDNSRecords":
		return a.PushCloudflareDNSRecords(payload), nil
	case "RunDesktopProbe":
		var typed DesktopProbePayload
		if err := json.Unmarshal(raw, &typed); err != nil {
			return nil, err
		}
		return a.RunDesktopProbe(typed)
	case "CancelProbe":
		return a.CancelProbe(payload), nil
	case "ResumeProbe":
		return a.ResumeProbe(payload), nil
	case "ListResultFile":
		return a.ListResultFile(payload), nil
	case "OpenPath":
		target := strings.TrimSpace(stringValue(firstNonNil(payload["target_path"], payload["targetPath"], payload["path"]), ""))
		if target == "" {
			target = strings.TrimSpace(stringValue(payload["value"], ""))
		}
		return desktopCommandResult("PATH_OPEN_READY", map[string]any{"path": target}, "路径已准备由浏览器打开。", true, nil, nil), nil
	default:
		return nil, fmt.Errorf("unknown app method: %s", method)
	}
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

	ch, unsubscribe := a.eventHub.subscribe()
	defer unsubscribe()
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			raw, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", raw)
			flusher.Flush()
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
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
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
