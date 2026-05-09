package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"strings"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
)

type smtpSender struct {
	host       string
	port       int
	encryption string
	username   string
	password   string
	from       string
	to         []string
}

func newSMTPSender(cfg *config.ControllerSMTPNotificationConfig) (sender, error) {
	if cfg == nil {
		return nil, fmt.Errorf("smtp notification config is required")
	}
	return &smtpSender{
		host:       strings.TrimSpace(cfg.Host),
		port:       cfg.Port,
		encryption: config.NormalizeSMTPEncryption(cfg.Encryption),
		username:   strings.TrimSpace(cfg.Username),
		password:   cfg.Password,
		from:       strings.TrimSpace(cfg.From),
		to:         append([]string(nil), cfg.To...),
	}, nil
}

func (sender *smtpSender) Send(ctx context.Context, subject, body string) error {
	address := net.JoinHostPort(sender.host, fmt.Sprintf("%d", sender.port))
	client, err := sender.connect(ctx, address)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	if sender.username != "" {
		if ok, _ := client.Extension("AUTH"); !ok {
			return fmt.Errorf("smtp server %q does not support AUTH", sender.host)
		}
		if err := client.Auth(smtp.PlainAuth("", sender.username, sender.password, sender.host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(sender.from); err != nil {
		return fmt.Errorf("smtp mail from %q: %w", sender.from, err)
	}
	for _, recipient := range sender.to {
		if err := client.Rcpt(strings.TrimSpace(recipient)); err != nil {
			return fmt.Errorf("smtp rcpt to %q: %w", recipient, err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	message := buildSMTPMessage(sender.from, sender.to, subject, body)
	if _, err := io.WriteString(writer, message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("smtp write message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp close writer: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp quit: %w", err)
	}
	return nil
}

func (sender *smtpSender) connect(ctx context.Context, address string) (*smtp.Client, error) {
	if sender.encryption == config.SMTPEncryptionSSLTLS {
		dialer := &tls.Dialer{Config: &tls.Config{ServerName: sender.host, MinVersion: tls.VersionTLS12}}
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return nil, fmt.Errorf("dial smtp tls %q: %w", address, err)
		}
		client, err := smtp.NewClient(conn, sender.host)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("create smtp client: %w", err)
		}
		return client, nil
	}
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial smtp %q: %w", address, err)
	}
	client, err := smtp.NewClient(conn, sender.host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("create smtp client: %w", err)
	}
	if sender.encryption != config.SMTPEncryptionSTARTTLS {
		return client, nil
	}
	if ok, _ := client.Extension("STARTTLS"); !ok {
		_ = client.Close()
		return nil, fmt.Errorf("smtp server %q does not support STARTTLS", sender.host)
	}
	if err := client.StartTLS(&tls.Config{ServerName: sender.host, MinVersion: tls.VersionTLS12}); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("smtp starttls: %w", err)
	}
	return client, nil
}

func buildSMTPMessage(from string, to []string, subject, body string) string {
	headers := []string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", strings.Join(to, ", ")),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		body,
		"",
	}
	return strings.Join(headers, "\r\n")
}
