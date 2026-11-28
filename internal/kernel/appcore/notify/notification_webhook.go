package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpclient"
)

type WebhookNotificationConfig struct {
	Enabled        bool              `json:"enabled"`
	URLs           []string          `json:"urls"`
	DingTalkURL    string            `json:"dingtalk_url"`
	DingTalkSecret string            `json:"dingtalk_secret"`
	WeComURLs      []string          `json:"wecom_urls"`
	UseSystemProxy bool              `json:"use_system_proxy"`
	Headers        map[string]string `json:"headers"`
	WeComCorpID    string            `json:"wecom_corp_id"`
	WeComAgentID   string            `json:"wecom_agent_id"`
	WeComSecret    string            `json:"wecom_secret"`
	WeComMobiles   []string          `json:"wecom_mobiles"`
}

func WebhookNotificationConfigFromSnapshot(snapshot map[string]any) WebhookNotificationConfig {
	notifications := mapValue(firstNonNil(snapshot["notifications"], snapshot["notification"]))
	webhook := mapValue(notifications["webhook"])
	return WebhookNotificationConfig{
		Enabled:        boolValue(webhook["enabled"], false),
		URLs:           uniqueNonEmptyStrings(stringSlice(firstNonNil(webhook["urls"], webhook["url"]))),
		DingTalkURL:    strings.TrimSpace(stringValue(webhook["dingtalk_url"], "")),
		DingTalkSecret: strings.TrimSpace(stringValue(webhook["dingtalk_secret"], "")),
		WeComURLs:      uniqueNonEmptyStrings(stringSlice(webhook["wecom_urls"])),
		Headers:        stringMap(webhook["headers"]),
		WeComCorpID:    strings.TrimSpace(stringValue(webhook["wecom_corp_id"], "")),
		WeComAgentID:   strings.TrimSpace(stringValue(webhook["wecom_agent_id"], "")),
		WeComSecret:    strings.TrimSpace(stringValue(webhook["wecom_secret"], "")),
		WeComMobiles:   uniqueNonEmptyStrings(stringSlice(webhook["wecom_mobiles"])),
		UseSystemProxy: boolValue(firstNonNil(webhook["use_system_proxy"], webhook["useSystemProxy"]), false),
	}
}

// NotificationHTTPClient 让通知渠道按各自开关决定是否跟随系统代理。关闭时
// 必须直连：http.DefaultClient 只读 HTTP_PROXY 等环境变量，会悄悄套上代理。
// 打开时显式配置的 HTTPS_PROXY 优先于系统代理，服务器上没有系统代理设置也能
// 只靠环境变量走代理。
func NotificationHTTPClient(client *http.Client, useSystemProxy bool) *http.Client {
	if client != nil && client != http.DefaultClient {
		return client
	}
	timeout := TelegramNotificationSendTimeout
	if client != nil && client.Timeout > 0 {
		timeout = client.Timeout
	}
	options := httpclient.Options{
		BlockLinkLocal: true,
		Protocol:       httpclient.ProtocolTCP,
		Timeout:        timeout,
	}
	if useSystemProxy {
		options.Proxy = httpclient.EnvFirstProxy
	} else {
		options.DisableProxy = true
	}
	return httpclient.NewClient(options)
}

func SendWebhookUploadNotification(ctx context.Context, snapshot map[string]any, notification UploadNotification, client *http.Client) error {
	cfg := WebhookNotificationConfigFromSnapshot(snapshot)
	if !cfg.Enabled {
		return nil
	}
	return sendWebhookPayload(ctx, cfg, UploadNotificationEventPayload(notification), UploadNotificationSummaryText(notification), client)
}

func SendWebhookTaskFailureNotification(ctx context.Context, snapshot map[string]any, input TaskFailureNotificationInput, client *http.Client) error {
	cfg := WebhookNotificationConfigFromSnapshot(snapshot)
	if !cfg.Enabled {
		return nil
	}
	return sendWebhookPayload(ctx, cfg, map[string]any{"event": "task_failure", "task": input}, TaskFailureNotificationText(input), client)
}

func sendWebhookPayload(ctx context.Context, cfg WebhookNotificationConfig, payload any, text string, client *http.Client) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := contextWithTelegramNotificationTimeout(ctx)
	defer cancel()
	client = notificationHTTPClient(client, cfg.UseSystemProxy)
	if cfg.WeComCorpID != "" && cfg.WeComAgentID != "" && cfg.WeComSecret != "" && len(cfg.WeComMobiles) > 0 {
		if err := sendWeComMobileMessage(ctx, cfg, text, client); err != nil {
			return err
		}
	}
	type target struct {
		name, endpoint string
		payload        any
	}
	targets := make([]target, 0)
	for _, endpoint := range cfg.URLs {
		targets = append(targets, target{"Generic", endpoint, payload})
	}
	for _, endpoint := range cfg.WeComURLs {
		targets = append(targets, target{"企业微信", endpoint, map[string]any{"msgtype": "text", "text": map[string]string{"content": text}}})
	}
	if cfg.DingTalkURL != "" {
		endpoint := cfg.DingTalkURL
		if cfg.DingTalkSecret != "" {
			parsed, err := url.Parse(endpoint)
			if err != nil {
				return errors.New("钉钉 Webhook 地址无效")
			}
			timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
			mac := hmac.New(sha256.New, []byte(cfg.DingTalkSecret))
			_, _ = mac.Write([]byte(timestamp + "\n" + cfg.DingTalkSecret))
			query := parsed.Query()
			query.Set("timestamp", timestamp)
			query.Set("sign", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
			parsed.RawQuery = query.Encode()
			endpoint = parsed.String()
		}
		targets = append(targets, target{"钉钉", endpoint, map[string]any{"msgtype": "text", "text": map[string]string{"content": text}}})
	}
	failures := make([]string, 0)
	for index, item := range targets {
		parsed, err := httpclient.ValidateEndpointURL(item.endpoint, true)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s target %d: %v", item.name, index+1, err))
			continue
		}
		body, err := json.Marshal(item.payload)
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, parsed.String(), bytes.NewReader(body))
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s target %d: 请求构造失败", item.name, index+1))
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		for key, value := range cfg.Headers {
			req.Header.Set(key, value)
		}
		res, err := client.Do(req)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s target %d: %v", item.name, index+1, redactWebhookError(err, item.endpoint)))
			continue
		}
		var response struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		decodeErr := json.NewDecoder(io.LimitReader(res.Body, 2048)).Decode(&response)
		res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			failures = append(failures, fmt.Sprintf("%s target %d: HTTP %d", item.name, index+1, res.StatusCode))
		} else if (item.name == "钉钉" || item.name == "企业微信") && decodeErr == nil && response.ErrCode != 0 {
			failures = append(failures, fmt.Sprintf("%s target %d: errcode %d", item.name, index+1, response.ErrCode))
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "；"))
	}
	return nil
}

func sendWeComMobileMessage(ctx context.Context, cfg WebhookNotificationConfig, text string, client *http.Client) error {
	tokenReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid="+url.QueryEscape(cfg.WeComCorpID)+"&corpsecret="+url.QueryEscape(cfg.WeComSecret), nil)
	tokenRes, err := client.Do(tokenReq)
	if err != nil {
		return err
	}
	defer tokenRes.Body.Close()
	var token struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.NewDecoder(io.LimitReader(tokenRes.Body, 4096)).Decode(&token); err != nil {
		return err
	}
	if token.ErrCode != 0 || token.AccessToken == "" {
		return fmt.Errorf("企业微信 token 失败：%d", token.ErrCode)
	}
	for _, mobile := range cfg.WeComMobiles {
		mobileBody, err := json.Marshal(map[string]string{"mobile": mobile})
		if err != nil {
			return err
		}
		lookup, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://qyapi.weixin.qq.com/cgi-bin/user/getuserid?access_token="+url.QueryEscape(token.AccessToken), bytes.NewReader(mobileBody))
		lookup.Header.Set("Content-Type", "application/json")
		lookupRes, err := client.Do(lookup)
		if err != nil {
			return err
		}
		var user struct {
			UserID  string `json:"userid"`
			ErrCode int    `json:"errcode"`
		}
		_ = json.NewDecoder(io.LimitReader(lookupRes.Body, 4096)).Decode(&user)
		lookupRes.Body.Close()
		if err != nil || user.ErrCode != 0 || user.UserID == "" {
			return fmt.Errorf("企业微信手机号无对应成员：%s", mobile)
		}
		body, _ := json.Marshal(map[string]any{"touser": user.UserID, "msgtype": "text", "agentid": cfg.WeComAgentID, "text": map[string]string{"content": text}})
		send, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token="+url.QueryEscape(token.AccessToken), bytes.NewReader(body))
		send.Header.Set("Content-Type", "application/json")
		sendRes, err := client.Do(send)
		if err != nil {
			return err
		}
		sendRes.Body.Close()
		if sendRes.StatusCode < 200 || sendRes.StatusCode >= 300 {
			return fmt.Errorf("企业微信消息发送失败：HTTP %d", sendRes.StatusCode)
		}
	}
	return nil
}

func redactWebhookError(err error, endpoint string) error {
	return errors.New(strings.ReplaceAll(err.Error(), endpoint, "<redacted webhook URL>"))
}
