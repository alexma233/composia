package notify

import (
	"bufio"
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
)

func TestNewSMTPSenderNormalizesConfig(t *testing.T) {
	t.Parallel()

	rawSender, err := newSMTPSender(&config.ControllerSMTPNotificationConfig{
		Host:       " smtp.example.com ",
		Port:       587,
		Encryption: "STARTTLS",
		Username:   " alex ",
		Password:   "secret",
		From:       " composia@example.com ",
		To:         []string{"ops@example.com"},
	})
	if err != nil {
		t.Fatalf("newSMTPSender returned error: %v", err)
	}
	sender, ok := rawSender.(*smtpSender)
	if !ok {
		t.Fatalf("sender type = %T", rawSender)
	}
	if sender.host != "smtp.example.com" || sender.encryption != config.SMTPEncryptionSTARTTLS || sender.username != "alex" || sender.from != "composia@example.com" {
		t.Fatalf("unexpected sender config: %+v", sender)
	}
}

func TestNewSMTPSenderRequiresConfig(t *testing.T) {
	t.Parallel()

	_, err := newSMTPSender(nil)
	if err == nil || !strings.Contains(err.Error(), "config is required") {
		t.Fatalf("expected config error, got %v", err)
	}
}

func TestBuildSMTPMessage(t *testing.T) {
	t.Parallel()

	message := buildSMTPMessage("composia@example.com", []string{"ops@example.com", "dev@example.com"}, "Subject", "Body")
	for _, want := range []string{
		"From: composia@example.com\r\n",
		"To: ops@example.com, dev@example.com\r\n",
		"Subject: Subject\r\n",
		"Content-Type: text/plain; charset=UTF-8\r\n",
		"\r\nBody\r\n",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("message missing %q:\n%s", want, message)
		}
	}
}

func TestSMTPSenderSendBoundsPostConnectIOWithContext(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen smtp: %v", err)
	}
	defer func() { _ = listener.Close() }()
	accepted := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		close(accepted)
		_, _ = conn.Write([]byte("220 fake smtp\r\n"))
		time.Sleep(time.Second)
	}()

	host, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	sender := &smtpSender{host: host, port: port, encryption: config.SMTPEncryptionNone, from: "composia@example.com", to: []string{"ops@example.com"}}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = sender.Send(ctx, "Subject", "Body")
	if err == nil {
		t.Fatal("expected stalled smtp server to fail")
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("smtp send was not bounded by context: %s", elapsed)
	}
	select {
	case <-accepted:
	default:
		t.Fatal("smtp server did not accept connection")
	}
}

func TestSMTPSenderSendUsesSMTPEnvelopeAndMessage(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen smtp: %v", err)
	}
	defer func() { _ = listener.Close() }()
	result := make(chan fakeSMTPResult, 1)
	go serveFakeSMTP(listener, result)

	host, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	sender := &smtpSender{host: host, port: port, encryption: config.SMTPEncryptionNone, from: "composia@example.com", to: []string{"ops@example.com"}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sender.Send(ctx, "Subject", "Body"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	select {
	case got := <-result:
		if got.mailFrom != "MAIL FROM:<composia@example.com>" {
			t.Fatalf("mail from = %q", got.mailFrom)
		}
		if got.rcptTo != "RCPT TO:<ops@example.com>" {
			t.Fatalf("rcpt to = %q", got.rcptTo)
		}
		if !strings.Contains(got.message, "Subject: Subject") || !strings.Contains(got.message, "\r\nBody\r\n") {
			t.Fatalf("unexpected smtp message:\n%s", got.message)
		}
	case <-ctx.Done():
		t.Fatalf("timed out waiting for smtp result")
	}
}

type fakeSMTPResult struct {
	mailFrom string
	rcptTo   string
	message  string
}

func serveFakeSMTP(listener net.Listener, result chan<- fakeSMTPResult) {
	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()
	reader := bufio.NewReader(conn)
	writeLine := func(line string) bool {
		_, err := conn.Write([]byte(line + "\r\n"))
		return err == nil
	}
	if !writeLine("220 fake smtp") {
		return
	}
	var got fakeSMTPResult
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		command := strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(command, "EHLO ") || strings.HasPrefix(command, "HELO "):
			if !writeLine("250-fake") || !writeLine("250 OK") {
				return
			}
		case strings.HasPrefix(command, "MAIL FROM:"):
			got.mailFrom = command
			if !writeLine("250 OK") {
				return
			}
		case strings.HasPrefix(command, "RCPT TO:"):
			got.rcptTo = command
			if !writeLine("250 OK") {
				return
			}
		case command == "DATA":
			if !writeLine("354 End data with <CR><LF>.<CR><LF>") {
				return
			}
			var builder strings.Builder
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if dataLine == ".\r\n" {
					break
				}
				builder.WriteString(dataLine)
			}
			got.message = builder.String()
			if !writeLine("250 OK") {
				return
			}
		case command == "QUIT":
			_ = writeLine("221 Bye")
			result <- got
			return
		default:
			if !writeLine("250 OK") {
				return
			}
		}
	}
}
