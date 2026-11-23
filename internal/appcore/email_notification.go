package appcore

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// 邮件头字段必须是单行：配置值里的 CR/LF 会拼出额外的 SMTP 头，等于邮件内容注入。
func emailHeaderValue(value string) string {
	return strings.NewReplacer("\r", "", "\n", "", "\x00", "").Replace(strings.TrimSpace(value))
}

func emailHeaderList(values []string) []string {
	sanitized := make([]string, 0, len(values))
	for _, value := range values {
		if clean := emailHeaderValue(value); clean != "" {
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
	if err := client.Mail(emailHeaderValue(cfg.From)); err != nil {
		return err
	}
	for _, recipient := range cfg.To {
		if err := client.Rcpt(emailHeaderValue(recipient)); err != nil {
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
