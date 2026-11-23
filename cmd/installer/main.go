//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// version 由构建注入：go build -ldflags "-X main.version=1.9.8"。
var version = "2.1.9"

var errNoPayload = errors.New(
	"安装包内没有主程序：发布构建需先把 cfst-gui.exe 放进 cmd/installer/payload/ 再编译")

func main() {
	opts, silent, uninstall, err := parseArgs(os.Args[1:])
	if err != nil {
		fail(err)
	}

	// 卸载必须先于 WebView2 预检：装完之后运行时可能已被移除，
	// 但卸载不能因此卡住。
	if uninstall {
		runUninstall(silent)
		return
	}

	if !webView2Installed() {
		// 界面依赖 WebView2，所以这条路径只能用原生对话框。
		if silent {
			os.Exit(66)
		}
		messageBox(webview2Missing, appName+" 安装程序", mbOK|mbIconExclamation)
		_ = openURL(webview2URL)
		os.Exit(66)
	}

	if silent {
		runSilent(opts)
		return
	}
	runGUI(opts)
}

// parseArgs 同时接受 NSIS 风格的 /S、/D= 与常规 --flag，方便沿用现有部署脚本。
func parseArgs(args []string) (Options, bool, bool, error) {
	opts := Options{Dir: defaultInstallDir(), DesktopShortcut: true, Launch: true}
	var silent, uninstall bool
	for _, arg := range args {
		switch {
		case strings.EqualFold(arg, "/S"), strings.EqualFold(arg, "/silent"), arg == "--silent":
			silent = true
			opts.Silent = true
		case arg == "--uninstall":
			uninstall = true
		case strings.HasPrefix(arg, "--dir="):
			opts.Dir = strings.TrimPrefix(arg, "--dir=")
		case strings.HasPrefix(arg, "/D="):
			opts.Dir = strings.TrimPrefix(arg, "/D=")
		case arg == "--no-desktop":
			opts.DesktopShortcut = false
		case arg == "--no-launch":
			opts.Launch = false
		case arg == "--help", arg == "/?":
			fmt.Println("CFST-GUI 安装程序\n  /S、--silent      无界面安装\n  --uninstall      卸载\n  --dir=<路径>     指定安装目录\n  --no-desktop     不创建桌面快捷方式\n  --no-launch      安装完成后不启动")
			os.Exit(0)
		default:
			return opts, false, false, fmt.Errorf("无法识别的参数：%s", arg)
		}
	}
	return opts, silent, uninstall, nil
}

func fail(err error) {
	messageBox(err.Error(), appName+" 安装程序", mbOK|mbIconError)
	os.Exit(1)
}

// noop 用于静默路径：进度没人看，但要满足 Progress 契约。
func noop(int, string) error { return nil }

func runSilent(opts Options) {
	payload, err := payloadBytes(payloadFS)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := Install(payload, selfPath(), opts, noop); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runUninstall(silent bool) {
	self := selfPath()
	dir := filepath.Dir(self)
	if !silent {
		// 卸载用原生确认框：装完之后 WebView2 可能已不在，不能再依赖界面渲染。
		if messageBox("确定要卸载 "+appName+" 吗？", appName+" 卸载", mbYesNo|mbIconQuestion) != idYes {
			return
		}
	}
	if err := Uninstall(dir, self, noop); err != nil {
		if !silent {
			messageBox(err.Error(), appName+" 卸载", mbOK|mbIconError)
		}
		os.Exit(1)
	}
	scheduleDelete(dir)
}

func selfPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

// scheduleDelete 用独立工作目录延迟删除运行中的卸载器。
func scheduleDelete(dir string) {
	comspec := os.Getenv("COMSPEC")
	if comspec == "" {
		comspec = filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	}
	cmd := exec.Command(comspec)
	// cmd.exe 不使用 Go 默认的参数转义；环境变量避免路径中的 %、& 被再次解析。
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
		CmdLine:    `"` + comspec + `" /D /V:OFF /C ping -n 3 127.0.0.1 >nul & del /f /q "%CFST_INSTALLER_REMOVE_EXE%" >nul 2>&1 & rd "%CFST_INSTALLER_REMOVE_DIR%" >nul 2>&1`,
	}
	cmd.Env = append(os.Environ(), "CFST_INSTALLER_REMOVE_DIR="+dir, "CFST_INSTALLER_REMOVE_EXE="+filepath.Join(dir, uninstName))
	cmd.Dir = os.Getenv("SystemRoot")
	_ = cmd.Start()
}

// ── 界面 ──────────────────────────────────────────────────────────────

type server struct {
	wailsApp  *application.App
	opts      Options
	payload   []byte
	self      string
	legalFS   fs.FS
	webviewOK bool
	cancelled atomic.Bool
	running   atomic.Bool
	quit      func()
	// startDrag 由窗口创建后注入：提权窗口的 WebView2 无法可靠转发
	// wails:drag 消息，所以标题栏按下时由 Go 端直接触发原生拖动。
	startDrag func()
}

const installerAutoCloseDelay = 1200 * time.Millisecond

func runGUI(opts Options) {
	payload, err := payloadBytes(payloadFS)
	if err != nil {
		fail(err)
	}
	srv := &server{opts: opts, payload: payload, self: selfPath(), legalFS: payloadFS, webviewOK: true}

	mux := newInstallerMux(srv)

	app := application.New(application.Options{
		Name:        appName + " 安装程序",
		Description: "CFST-GUI 安装程序",
		Assets: application.AssetOptions{
			Handler: mux,
		},
	})
	srv.wailsApp = app
	srv.quit = func() { app.Quit() }

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "installer",
		Title:     appName + " 安装程序",
		Frameless: true,
		Width:     560,
		Height:    380,
		URL:       "/",
	})
	srv.startDrag = func() { window.HandleMessage("wails:drag") }

	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if srv.running.Load() {
			srv.cancelled.Store(true)
			event.Cancel()
		}
	})

	if err := app.Run(); err != nil {
		fail(err)
	}
}

// newInstallerMux 组装安装器的全部 HTTP 路由：页面、拆分后的静态资源与 /api/*。
func newInstallerMux(srv *server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleIndex)
	mux.HandleFunc("/installer.css", handleUIAsset("installer.css", "text/css; charset=utf-8"))
	mux.HandleFunc("/installer.js", handleUIAsset("installer.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("/api/browse", srv.handleBrowse)
	mux.HandleFunc("/api/install", srv.handleInstall)
	mux.HandleFunc("/api/cancel", srv.handleCancel)
	mux.HandleFunc("/api/close", srv.handleClose)
	mux.HandleFunc("/api/legal", srv.handleLegal)
	mux.HandleFunc("/api/running", srv.handleRunning)
	mux.HandleFunc("/api/close-app", srv.handleCloseApp)
	mux.HandleFunc("/api/drag", srv.handleDrag)
	return mux
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = fmt.Fprint(w, renderUI(s.opts, s.webviewOK))
}

// handleLegal 只读暴露安装包内嵌的许可证文本，安装前后都可查看。
func (s *server) handleLegal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "请求方法不受支持", http.StatusMethodNotAllowed)
		return
	}
	name := r.URL.Query().Get("name")
	allowed := false
	for _, candidate := range installerLegalFileNames {
		if candidate == name {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "未知的许可证文件", http.StatusNotFound)
		return
	}
	source := s.legalFS
	if source == nil {
		source = payloadFS
	}
	files, err := installerLegalFiles(source)
	if err != nil {
		http.Error(w, "安装包中没有内嵌许可证文本", http.StatusServiceUnavailable)
		return
	}
	for _, file := range files {
		if file.name != name {
			continue
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(file.data)
		return
	}
	http.Error(w, "未知的许可证文件", http.StatusNotFound)
}

func (s *server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	dir, err := pickFolder(s.opts.Dir)
	if err != nil || dir == "" {
		writeJSON(w, map[string]string{"dir": "", "error": errString(err)})
		return
	}
	writeJSON(w, map[string]string{"dir": dir})
}

// handleRunning 供界面在开始安装前判断目标程序是否正在运行。
func (s *server) handleRunning(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "请求方法不受支持", http.StatusMethodNotAllowed)
		return
	}
	dir := r.URL.Query().Get("dir")
	if dir == "" {
		dir = s.opts.Dir
	}
	pids, err := runningProcessIDs(filepath.Join(dir, appExeName))
	if err != nil {
		writeJSON(w, map[string]any{"running": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"running": len(pids) > 0, "count": len(pids)})
}

// handleCloseApp 只在用户确认关闭应用后调用：结束目标安装目录里的主程序。
func (s *server) handleCloseApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "请求方法不受支持", http.StatusMethodNotAllowed)
		return
	}
	dir := r.URL.Query().Get("dir")
	if dir == "" {
		dir = s.opts.Dir
	}
	target := filepath.Join(dir, appExeName)
	pids, err := runningProcessIDs(target)
	if err != nil {
		writeJSON(w, map[string]any{"closed": false, "error": "检测应用运行状态失败：" + err.Error()})
		return
	}
	if len(pids) == 0 {
		writeJSON(w, map[string]any{"closed": true})
		return
	}
	if err := closeProcesses(pids); err != nil {
		writeJSON(w, map[string]any{"closed": false, "error": err.Error()})
		return
	}
	// 进程结束后系统还需要一点时间释放 exe 句柄，等它彻底可替换再返回。
	var remaining []uint32
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		remaining, _ = runningProcessIDs(target)
		if len(remaining) == 0 {
			break
		}
	}
	if len(remaining) > 0 {
		writeJSON(w, map[string]any{"closed": false, "error": "应用仍在运行，请手动退出后重试"})
		return
	}
	writeJSON(w, map[string]any{"closed": true})
}

// handleDrag 由标题栏的 mousedown 调用，交给 Wails 走原生窗口拖动。
// 不依赖 WebView2 的运行时消息转发，提权运行时同样有效。
func (s *server) handleDrag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "请求方法不受支持", http.StatusMethodNotAllowed)
		return
	}
	if s.startDrag == nil {
		http.Error(w, "窗口尚未就绪", http.StatusServiceUnavailable)
		return
	}
	s.startDrag()
	writeJSON(w, map[string]bool{"ok": true})
}

// handleInstall 用 SSE 推进度：EventSource 只支持 GET，所以选项走查询参数。
func (s *server) handleInstall(w http.ResponseWriter, r *http.Request) {
	if !s.running.CompareAndSwap(false, true) {
		http.Error(w, "安装已在进行中", http.StatusConflict)
		return
	}
	defer s.running.Store(false)
	s.cancelled.Store(false)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "当前环境不支持进度推送", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")

	opts := s.opts
	if dir := r.URL.Query().Get("dir"); dir != "" {
		opts.Dir = dir
	}
	opts.DesktopShortcut = queryBool(r, "desktop", opts.DesktopShortcut)
	opts.Launch = queryBool(r, "launch", opts.Launch)

	send := func(v any) {
		data, err := json.Marshal(v)
		if err != nil {
			return
		}
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	emit := func(percent int, line string) error {
		if s.cancelled.Load() {
			return ErrCancelled
		}
		send(map[string]any{"p": percent, "line": line})
		return nil
	}

	err := Install(s.payload, s.self, opts, emit)
	switch {
	case errors.Is(err, ErrCancelled):
		send(map[string]any{"p": 0, "line": "已取消", "done": true, "cancelled": true})
	default:
		if err != nil {
			send(map[string]any{"line": err.Error(), "done": true, "error": err.Error()})
			return
		}
		send(map[string]any{"p": 100, "line": "安装完成", "done": true, "ok": true})
		// 成功安装后自动关闭安装器；保留短暂停留让完成日志可见。
		// 失败和取消不走这里，窗口继续保留给用户排查。
		s.scheduleQuit(installerAutoCloseDelay)
	}
}

func (s *server) scheduleQuit(delay time.Duration) *time.Timer {
	if s.quit == nil {
		return nil
	}
	return time.AfterFunc(delay, s.quit)
}

func (s *server) handleCancel(w http.ResponseWriter, r *http.Request) {
	s.cancelled.Store(true)
	writeJSON(w, map[string]bool{"cancelled": true})
}

func (s *server) handleClose(w http.ResponseWriter, r *http.Request) {
	if s.running.Load() {
		s.cancelled.Store(true)
		writeJSON(w, map[string]bool{"closing": false, "cancelled": true})
		return
	}
	writeJSON(w, map[string]bool{"closing": true})
	go s.wailsApp.Quit()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func queryBool(r *http.Request, key string, fallback bool) bool {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
