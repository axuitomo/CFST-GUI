package archivecore

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/configvalue"
)

const (
	ConfigArchiveEntryName      = "cfst-gui-config.json"
	DefaultConfigArchiveName    = "cfst-gui-config.zip"
	DefaultWebDAVTimeoutSeconds = 30
)

type WebDAVConfig struct {
	Enabled        bool
	Password       string
	RemotePath     string
	ServerURL      string
	TimeoutSeconds int
	Username       string
}

type PayloadOptions struct {
	AllowPathRead bool
}

func ZipSingleFile(name string, raw []byte, modTime ...time.Time) ([]byte, error) {
	if strings.TrimSpace(name) == "" {
		name = ConfigArchiveEntryName
	}
	timestamp := time.Now()
	if len(modTime) > 0 && !modTime[0].IsZero() {
		timestamp = modTime[0]
	}
	buffer := bytes.NewBuffer(nil)
	writer := zip.NewWriter(buffer)
	header := &zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	}
	header.SetModTime(timestamp)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		_ = writer.Close()
		return nil, err
	}
	if _, err := entry.Write(raw); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func ParseConfigArchive(raw []byte) (map[string]any, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("配置文件内容为空")
	}
	if bytes.HasPrefix(trimmed, []byte("{")) {
		return ParseConfigArchiveJSON(trimmed)
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	var fallback *zip.File
	for _, file := range reader.File {
		if file.Name == ConfigArchiveEntryName {
			return readArchiveJSONFile(file)
		}
		if fallback == nil && strings.HasSuffix(strings.ToLower(file.Name), ".json") {
			fallback = file
		}
	}
	if fallback != nil {
		return readArchiveJSONFile(fallback)
	}
	return nil, fmt.Errorf("配置压缩包缺少 %s", ConfigArchiveEntryName)
}

func ParseConfigArchiveJSON(raw []byte) (map[string]any, error) {
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	return body, nil
}

func ArchivePayloadBytes(payload map[string]any, options ...PayloadOptions) ([]byte, string, error) {
	allowPathRead := true
	if len(options) > 0 {
		allowPathRead = options[0].AllowPathRead
	}
	if encoded := strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(payload["content_base64"], payload["contentBase64"]), "")); encoded != "" {
		raw, err := base64.StdEncoding.DecodeString(encoded)
		return raw, DefaultConfigArchiveName, err
	}
	if content := configvalue.String(payload["content"], ""); strings.TrimSpace(content) != "" {
		return []byte(content), ConfigArchiveEntryName, nil
	}
	if targetPath := strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(payload["path"], payload["target_path"], payload["targetPath"], payload["source_path"], payload["sourcePath"]), "")); targetPath != "" {
		if !allowPathRead || strings.HasPrefix(targetPath, "content://") {
			return nil, "", fmt.Errorf("缺少配置压缩包内容或路径")
		}
		raw, err := os.ReadFile(targetPath)
		return raw, filepath.Base(targetPath), err
	}
	return nil, "", fmt.Errorf("缺少配置压缩包内容或路径")
}

func ParseWebDAVConfig(raw map[string]any) (WebDAVConfig, error) {
	cfg := WebDAVConfig{
		Enabled:        configvalue.Bool(raw["enabled"], false),
		Password:       configvalue.String(raw["password"], ""),
		RemotePath:     strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(raw["remote_path"], raw["remotePath"]), DefaultConfigArchiveName)),
		ServerURL:      strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(raw["server_url"], raw["serverUrl"], raw["url"]), "")),
		TimeoutSeconds: configvalue.Int(configvalue.FirstNonNil(raw["timeout_seconds"], raw["timeoutSeconds"]), DefaultWebDAVTimeoutSeconds),
		Username:       configvalue.String(raw["username"], ""),
	}
	if cfg.RemotePath == "" {
		cfg.RemotePath = DefaultConfigArchiveName
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = DefaultWebDAVTimeoutSeconds
	}
	if cfg.ServerURL == "" {
		return WebDAVConfig{}, fmt.Errorf("缺少 WebDAV 地址")
	}
	if err := validateWebDAVRemotePath(cfg.RemotePath); err != nil {
		return WebDAVConfig{}, err
	}
	return cfg, nil
}

func WebDAVTargetURL(cfg WebDAVConfig) (string, error) {
	if err := validateWebDAVRemotePath(cfg.RemotePath); err != nil {
		return "", err
	}
	base, err := url.Parse(cfg.ServerURL)
	if err != nil {
		return "", err
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return "", fmt.Errorf("WebDAV 地址必须以 http:// 或 https:// 开头")
	}
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	remotePath := strings.TrimLeft(cfg.RemotePath, "/")
	base.Path = path.Join(base.Path, remotePath)
	if strings.HasSuffix(remotePath, "/") {
		base.Path += "/"
	}
	return base.String(), nil
}

func WebDAVRequest(ctx context.Context, cfg WebDAVConfig, method, targetURL string, body []byte, userAgent string) (int, []byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "CFST-GUI"
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	client := &http.Client{Timeout: timeout}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, targetURL, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/zip")
	}
	if cfg.Username != "" || cfg.Password != "" {
		req.SetBasicAuth(cfg.Username, cfg.Password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024))
	return resp.StatusCode, raw, nil
}

func WebDAVHTTPErrorMessage(prefix string, status int, body []byte) string {
	detail := strings.TrimSpace(string(body))
	if detail == "" {
		return fmt.Sprintf("%s：HTTP %d", prefix, status)
	}
	if len(detail) > 240 {
		detail = detail[:240] + "..."
	}
	return fmt.Sprintf("%s：HTTP %d，%s", prefix, status, detail)
}

func SetWebDAVTimestamp(snapshot map[string]any, key string, value string) map[string]any {
	backup := configvalue.Map(snapshot["backup"])
	webdav := configvalue.Map(backup["webdav"])
	webdav[key] = value
	backup["webdav"] = webdav
	snapshot["backup"] = backup
	return snapshot
}

func SensitiveArchiveWarnings() []string {
	return []string{"配置压缩包包含完整 Cloudflare Token 和 WebDAV 凭据，请只保存到可信位置。"}
}

func validateWebDAVRemotePath(remotePath string) error {
	remotePath = strings.TrimSpace(remotePath)
	parsed, err := url.Parse(remotePath)
	if err != nil {
		return fmt.Errorf("WebDAV 远端路径无效：%w", err)
	}
	if parsed.Scheme != "" || parsed.Host != "" {
		return fmt.Errorf("WebDAV 远端路径必须是相对路径，不能填写完整 URL")
	}
	return nil
}

func readArchiveJSONFile(file *zip.File) (map[string]any, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return ParseConfigArchiveJSON(raw)
}
