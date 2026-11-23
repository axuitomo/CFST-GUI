package appcore

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestEmailNotificationCancellationWhileServerStalls(t *testing.T) {
	for _, stage := range []string{"greeting", "data"} {
		t.Run(stage, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				defer close(done)
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				if stage == "data" {
					_, _ = fmt.Fprint(conn, "220 fixture\r\n")
					reader := bufio.NewReader(conn)
					for {
						line, err := reader.ReadString('\n')
						if err != nil {
							return
						}
						if strings.HasPrefix(line, "DATA") {
							break
						}
						_, _ = fmt.Fprint(conn, "250 OK\r\n")
					}
				}
				cancel()
				_, _ = io.Copy(io.Discard, conn)
			}()
			host, portText, _ := net.SplitHostPort(listener.Addr().String())
			port, _ := strconv.Atoi(portText)
			result := make(chan error, 1)
			go func() {
				result <- sendEmailNotification(ctx, EmailNotificationConfig{Host: host, Port: port, From: "from@example.invalid", To: []string{"to@example.invalid"}}, []byte("fixture"))
			}()
			select {
			case err := <-result:
				if err == nil {
					t.Fatal("cancelled SMTP send succeeded")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("SMTP ignored cancellation")
			}
			<-done
		})
	}
}

func TestEmailNotificationTLSRequirementAndDelivery(t *testing.T) {
	for _, requireTLS := range []bool{false, true} {
		t.Run(strconv.FormatBool(requireTLS), func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			body := make(chan string, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					body <- ""
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				_, _ = fmt.Fprint(conn, "220 fixture\r\n")
				reader := textproto.NewReader(bufio.NewReader(conn))
				message := ""
				defer func() { body <- message }()
				for {
					line, err := reader.ReadLine()
					if err != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "DATA"):
						_, _ = fmt.Fprint(conn, "354 send data\r\n")
						raw, _ := reader.ReadDotBytes()
						message = string(raw)
						_, _ = fmt.Fprint(conn, "250 queued\r\n")
					case strings.HasPrefix(line, "QUIT"):
						_, _ = fmt.Fprint(conn, "221 bye\r\n")
						return
					default:
						_, _ = fmt.Fprint(conn, "250 OK\r\n")
					}
				}
			}()
			host, portText, _ := net.SplitHostPort(listener.Addr().String())
			port, _ := strconv.Atoi(portText)
			snapshot := map[string]any{"notifications": map[string]any{"email": map[string]any{
				"enabled": true, "host": host, "port": port, "from": "from@example.invalid", "to": []string{"to@example.invalid"}, "use_tls": requireTLS,
			}}}
			err = SendEmailUploadNotification(context.Background(), snapshot, UploadNotification{Status: UploadNotificationStatusCompleted, Message: "fixture-body"})
			message := <-body
			if requireTLS {
				if err == nil || !strings.Contains(err.Error(), "STARTTLS") || message != "" {
					t.Fatalf("TLS requirement: err=%v message=%q", err, message)
				}
			} else if err != nil || !strings.Contains(message, "fixture-body") {
				t.Fatalf("delivery: err=%v message=%q", err, message)
			}
		})
	}
}

func TestEmailNotificationUsesImplicitTLSOn465(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:465")
	if err != nil {
		t.Skip("local SMTP TLS port unavailable:", err)
	}
	defer listener.Close()
	firstByte := make(chan byte, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			firstByte <- 0
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		var header [1]byte
		_, _ = io.ReadFull(conn, header[:])
		firstByte <- header[0]
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = sendEmailNotification(ctx, EmailNotificationConfig{Host: "127.0.0.1", Port: 465, UseTLS: true}, nil)
	if err == nil || <-firstByte != 0x16 {
		t.Fatalf("expected a TLS handshake before the SMTP greeting: %v", err)
	}
}

// CR/LF 出现在配置值里时不得拼出额外的 SMTP 头（邮件内容注入）。
func TestEmailNotificationStripsHeaderInjection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	body := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			body <- ""
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		_, _ = fmt.Fprint(conn, "220 fixture\r\n")
		reader := textproto.NewReader(bufio.NewReader(conn))
		message := ""
		defer func() { body <- message }()
		for {
			line, err := reader.ReadLine()
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "DATA"):
				_, _ = fmt.Fprint(conn, "354 send data\r\n")
				raw, _ := reader.ReadDotBytes()
				message = string(raw)
				_, _ = fmt.Fprint(conn, "250 queued\r\n")
			case strings.HasPrefix(line, "QUIT"):
				_, _ = fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				_, _ = fmt.Fprint(conn, "250 OK\r\n")
			}
		}
	}()
	host, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	snapshot := map[string]any{"notifications": map[string]any{"email": map[string]any{
		"enabled": true, "host": host, "port": port, "use_tls": false,
		"from": "from@example.invalid\r\nBcc: injected@example.invalid",
		"to":   []string{"to@example.invalid\r\nBcc: injected@example.invalid"},
	}}}
	if err := SendEmailUploadNotification(context.Background(), snapshot, UploadNotification{Status: UploadNotificationStatusCompleted, Message: "fixture-body"}); err != nil {
		t.Fatalf("delivery failed: %v", err)
	}
	message := <-body
	if strings.Contains(message, "\r\nBcc:") || strings.Contains(message, "\nBcc:") {
		t.Fatalf("header injection survived sanitization: %q", message)
	}
	if !strings.Contains(message, "From: from@example.invalidBcc:") {
		t.Fatalf("unexpected From header: %q", message)
	}
}
