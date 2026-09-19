package app

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

// embeddedDistRoot 是根包 //go:embed frontend/dist 在 embed.FS 内的根目录。
const embeddedDistRoot = "frontend/dist"

// frontendCachePolicy 返回前端静态资源的 Cache-Control 指令，返回空串表示不覆盖默认行为。
//
// Vite 给 /assets/* 使用内容哈希命名：内容一变文件名必变，因此可以 immutable 长缓存；
// 但入口 index.html（请求路径 "/" 或 *.html）绝不能被 WebView2/浏览器缓存，否则在线覆盖
// 安装新版本后仍会读到旧的 index.html，再去拉旧哈希的 JS/CSS——表现就是「升级之后还是旧前端」。
// embed.FS 的 ModTime 为零，Wails/net/http 默认不会为 index.html 下发任何缓存协商头，
// 因此这里显式补齐策略。/wails/* 运行时与 /api/* 等其它路径一律透传、不加干预。
func frontendCachePolicy(urlPath string) string {
	switch {
	case urlPath == "/" || strings.HasSuffix(urlPath, ".html"):
		return "no-cache"
	case strings.HasPrefix(urlPath, "/assets/"):
		return "public, max-age=31536000, immutable"
	default:
		return ""
	}
}

// withFrontendCacheHeaders 在交给下一个 handler 之前，为前端静态资源响应补上缓存策略。
func withFrontendCacheHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if policy := frontendCachePolicy(r.URL.Path); policy != "" {
			w.Header().Set("Cache-Control", policy)
		}
		next.ServeHTTP(w, r)
	})
}

// verifyEmbeddedFrontend 确认内嵌的 frontend/dist 含入口 index.html。
//
// 全新克隆若只编译 Go 而没先 `pnpm build`，dist 里只有占位用的 .gitkeep。这里给出明确、
// 可操作的修复指引，而不是让 Wails 抛一个含义不清的 “no index.html” 或让窗口停在旧页面。
func verifyEmbeddedFrontend(assets fs.FS) error {
	if assets == nil {
		return fmt.Errorf("frontend assets are not configured; run `pnpm --dir frontend install && pnpm --dir frontend build` then rebuild")
	}
	if _, err := fs.Stat(assets, embeddedDistRoot+"/index.html"); err != nil {
		return fmt.Errorf("embedded frontend/dist is missing index.html; run `pnpm --dir frontend build` and rebuild the Go binary: %w", err)
	}
	return nil
}

// frontendUsesDevServer 判断当前进程是否由 wails3 dev 拉起（存在 FRONTEND_DEVSERVER_URL）。
func frontendUsesDevServer() bool {
	return strings.TrimSpace(os.Getenv("FRONTEND_DEVSERVER_URL")) != ""
}

// logFrontendSource 标注本次进程实际使用的前端来源，避免把「内嵌回退」误当成 Vite 实时源码：
// wails3 dev 注入 FRONTEND_DEVSERVER_URL 时反代 Vite，否则使用二进制内嵌的 frontend/dist 快照。
func logFrontendSource() {
	if devURL := strings.TrimSpace(os.Getenv("FRONTEND_DEVSERVER_URL")); devURL != "" {
		fmt.Printf("[frontend] proxying live Vite dev server at %s (HMR, latest source)\n", devURL)
		return
	}
	fmt.Println("[frontend] serving embedded frontend/dist snapshot (not live Vite); rebuild the frontend if you expected the latest UI")
}
