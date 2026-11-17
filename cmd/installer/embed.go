//go:build windows

package main

import (
	"embed"
	"fmt"
	"html"
	"io/fs"
	"strings"
)

//go:embed ui/index.html
var uiIndexHTML string

//go:embed all:payload
var payloadFS embed.FS

var installerLegalFileNames = [...]string{"LICENSE", "THIRD-PARTY-NOTICES.md"}

type installerLegalFile struct {
	name string
	data []byte
}

func installerLegalFiles(source fs.FS) ([]installerLegalFile, error) {
	files := make([]installerLegalFile, 0, len(installerLegalFileNames))
	for _, name := range installerLegalFileNames {
		data, err := fs.ReadFile(source, "payload/"+name)
		if err != nil {
			return nil, fmt.Errorf("embedded legal file %s: %w", name, err)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("embedded legal file %s is empty", name)
		}
		files = append(files, installerLegalFile{name: name, data: data})
	}
	return files, nil
}

// payloadBytes 只接受发布脚本写入的固定载荷，不能把遗留文件当作主程序。
func payloadBytes(source fs.FS) ([]byte, error) {
	data, err := fs.ReadFile(source, "payload/cfst-gui.exe")
	if err != nil || len(data) == 0 {
		return nil, errNoPayload
	}
	return data, nil
}

// renderUI 把运行期才知道的值注入静态页。
// 走字符串替换而不是模板引擎：页面只有一个，替换点固定且可 grep。
func renderUI(opts Options, webview2OK bool) string {
	state, text := "ok", "WEBVIEW2: DETECTED"
	if !webview2OK {
		state, text = "missing", "WEBVIEW2: MISSING"
	}
	checked := func(enabled bool) string {
		if enabled {
			return "checked"
		}
		return ""
	}
	replacer := strings.NewReplacer(
		"{{VERSION}}", html.EscapeString(version),
		"{{TARGET_DIR}}", html.EscapeString(opts.Dir),
		"{{DESKTOP_CHECKED}}", checked(opts.DesktopShortcut),
		"{{LAUNCH_CHECKED}}", checked(opts.Launch),
		"{{WEBVIEW2_STATE}}", state,
		"{{WEBVIEW2_TEXT}}", text,
	)
	return replacer.Replace(uiIndexHTML)
}
