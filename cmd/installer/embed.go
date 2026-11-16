package main

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed ui/index.html
var uiIndexHTML string

//go:embed all:payload
var payloadFS embed.FS

// payloadMarker 是仓库里常驻的占位文件；发布构建会把它换成真实主程序。
const payloadMarker = "payload/placeholder.txt"

// payloadBytes 返回内嵌的主程序字节。
// 发布构建把 cfst-gui.exe 放进 cmd/installer/payload/ 后编译，
// 开发期只有占位文件，此时返回明确错误而不是装出一个坏包。
func payloadBytes() ([]byte, error) {
	var found []byte
	err := fs.WalkDir(payloadFS, "payload", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path == payloadMarker {
			return err
		}
		data, readErr := payloadFS.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		// 多个文件时取最大的那个：主程序必然比说明文件大。
		if len(data) > len(found) {
			found = data
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, errNoPayload
	}
	return found, nil
}

// renderUI 把运行期才知道的值注入静态页。
// 走字符串替换而不是模板引擎：页面只有一个，替换点固定且可 grep。
func renderUI(targetDir string, webview2OK bool) string {
	state, text := "ok", "WEBVIEW2: DETECTED"
	if !webview2OK {
		state, text = "missing", "WEBVIEW2: MISSING"
	}
	replacer := strings.NewReplacer(
		"{{VERSION}}", version,
		"{{TARGET_DIR}}", targetDir,
		"{{WEBVIEW2_STATE}}", state,
		"{{WEBVIEW2_TEXT}}", text,
	)
	return replacer.Replace(uiIndexHTML)
}
