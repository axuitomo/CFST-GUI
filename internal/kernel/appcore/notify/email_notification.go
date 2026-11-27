package notify

import (
	"context"
	"crypto/tls"
	"errors"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type EmailNotificationConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       []string
	UseTLS   bool
}

func EmailNotificationConfigFromSnapshot(snapshot map[string]any) EmailNotificationConfig {
	notifications := mapValue(firstNonNil(snapshot["notifications"], snapshot["notification"]))
	email := mapValue(notifications["email"])
	return EmailNotificationConfig{
		Enabled:  boolValue(email["enabled"], false),
		Host:     strings.TrimSpace(stringValue(email["host"], "")),
		Port:     intValue(email["port"], 587),
		Username: strings.TrimSpace(stringValue(email["username"], "")),
		Password: stringValue(email["password"], ""),
		From:     strings.TrimSpace(stringValue(email["from"], "")),
		To:       uniqueNonEmptyStrings(stringSlice(email["to"])),
		UseTLS:   boolValue(email["use_tls"], true),
	}
}

func SendEmailUploadNotification(ctx context.Context, snapshot map[string]any, notification UploadNotification) error {
	cfg := EmailNotificationConfigFromSnapshot(snapshot)
	if !cfg.Enabled {
		return nil
	}
	if cfg.Host == "" || cfg.From == "" || len(cfg.To) == 0 {
		return errors.New("邮件通知配置不完整")
	}
	subject := "CFST 上传结论：" + UploadNotificationStatusLabel(notification.Status)
	return sendEmailNotification(ctx, cfg, emailNotificationMessage(cfg, subject, UploadNotificationText(notification)))
}

func SendEmailTaskFailureNotification(ctx context.Context, snapshot map[string]any, input TaskFailureNotificationInput) error {
	cfg := EmailNotificationConfigFromSnapshot(snapshot)
	if !cfg.Enabled {
		return nil
	}
	return SendEmailUploadNotification(ctx, snapshot, UploadNotification{
		Status:    UploadNotificationStatusFailed,
		Message:   input.Message,
		TaskID:    input.TaskID,
		CreatedAt: input.CreatedAt.Format(time.RFC3339),
	})
}

// emailNotificationMessage 按 RFC 5322/2045 拼出邮件：头部只出现 ASCII
// （非 ASCII 头值交给 RFC 2047 编码），正文声明 UTF-8 与 8bit 传输编码。
func emailNotificationMessage(cfg EmailNotificationConfig, subject, body string) []byte {
	headers := []string{
		"From: " + emailAddressHeader(cfg.From),
		"To: " + strings.Join(emailHeaderList(cfg.To), ","),
		"Subject: " + emailEncodedHeader(subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		body,
	}
	return []byte(strings.Join(headers, "\r\n"))
}

// 邮件头字段必须是单行：配置值里的 CR/LF 会拼出额外的 SMTP 头，等于邮件内容注入。
func emailHeaderValue(value string) string {
	return strings.NewReplacer("\r", "", "\n", "", "\x00", "").Replace(strings.TrimSpace(value))
}

// emailEncodedHeader 把含非 ASCII 的头值按 RFC 2047 编成 encoded-word。
// 头部字段只允许 US-ASCII：原始 UTF-8（中文、emoji）在严格服务器上会被拒，
// 在被宽松接受的服务器上也常被客户端按 Latin-1 解码成乱码。
// 纯 ASCII 值原样返回，不给本来就合规的内容加噪音。
func emailEncodedHeader(value string) string {
	cleaned := emailHeaderValue(value)
	if isASCIIString(cleaned) {
		return cleaned
	}
	return mime.BEncoding.Encode("UTF-8", cleaned)
}

func isASCIIString(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] >= 0x80 {
			return false
		}
	}
	return true
}

// emailAddressHeader 输出地址头（From/To）：显示名里的非 ASCII 由 net/mail
// 编成 encoded-word，地址本身保持 <a@b> 形式，不会被整体编码掉。
func emailAddressHeader(value string) string {
	cleaned := emailHeaderValue(value)
	if addr, err := mail.ParseAddress(cleaned); err == nil {
		// 没有显示名时保留裸地址写法，避免 From： <a@b> 这种多余尖括号。
		if addr.Name == "" {
			return addr.Address
		}
		return addr.String()
	}
	return emailEncodedHeader(cleaned)
}

// emailEnvelopeAddress 取出 SMTP 信封要用的裸地址：MAIL FROM / RCPT TO 只接受
// 地址本身，带显示名会被服务器拒收（RFC 5321）。解析失败时回退到清理后的原值。
func emailEnvelopeAddress(value string) string {
	cleaned := emailHeaderValue(value)
	if addr, err := mail.ParseAddress(cleaned); err == nil {
		return addr.Address
	}
	return cleaned
}

func emailHeaderList(values []string) []string {
	sanitized := make([]string, 0, len(values))
	for _, value := range values {
		if clean := emailAddressHeader(value); clean != "" {
			sanitized = append(sanitized, clean)
		}
	}
	return sanitized
}

func sendEmailNotification(ctx context.Context, cfg EmailNotificationConfig, message []byte) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)))
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}
	tlsConfig := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}
	transport := conn
	if cfg.UseTLS && cfg.Port == 465 {
		secure := tls.Client(conn, tlsConfig)
		if err := secure.HandshakeContext(ctx); err != nil {
			return err
		}
		transport = secure
	}
	client, err := smtp.NewClient(transport, cfg.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	if cfg.UseTLS && cfg.Port != 465 {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("SMTP server does not support required STARTTLS")
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return err
		}
	}
	if cfg.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			return err
		}
	}
	if err := client.Mail(emailEnvelopeAddress(cfg.From)); err != nil {
		return err
	}
	for _, recipient := range cfg.To {
		if err := client.Rcpt(emailEnvelopeAddress(recipient)); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
