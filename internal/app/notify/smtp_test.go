package notify

import (
	"strings"
	"testing"

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
