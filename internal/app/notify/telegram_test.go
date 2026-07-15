package notify

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
)

func TestNewTelegramSenderNormalizesConfig(t *testing.T) {
	t.Parallel()

	rawSender, err := newTelegramSender(&config.ControllerTelegramNotificationConfig{BotToken: " token ", ChatID: " chat "})
	if err != nil {
		t.Fatalf("newTelegramSender returned error: %v", err)
	}
	sender, ok := rawSender.(*telegramSender)
	if !ok {
		t.Fatalf("sender type = %T", rawSender)
	}
	if sender.botToken != "token" || sender.chatID != "chat" || sender.client == nil {
		t.Fatalf("unexpected sender config: %+v", sender)
	}
}

func TestNewTelegramSenderRequiresConfig(t *testing.T) {
	t.Parallel()

	_, err := newTelegramSender(nil)
	if err == nil || !strings.Contains(err.Error(), "config is required") {
		t.Fatalf("expected config error, got %v", err)
	}
}

func TestTelegramSenderSendPostsJSON(t *testing.T) {
	t.Parallel()

	transport := &recordingRoundTripper{status: http.StatusOK, body: `{"ok":true}`}
	sender := &telegramSender{botToken: "token", chatID: "chat", client: &http.Client{Transport: transport}}

	if err := sender.Send(context.Background(), "Subject", "Body"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if transport.method != http.MethodPost || transport.url != "https://api.telegram.org/bottoken/sendMessage" {
		t.Fatalf("request = %s %s", transport.method, transport.url)
	}
	if transport.contentType != "application/json" {
		t.Fatalf("Content-Type = %q", transport.contentType)
	}
	if !strings.Contains(transport.requestBody, `"chat_id":"chat"`) || !strings.Contains(transport.requestBody, `"text":"Subject\n\nBody"`) {
		t.Fatalf("unexpected request body: %s", transport.requestBody)
	}
}

func TestTelegramSenderSendRedactsTokenFromErrors(t *testing.T) {
	t.Parallel()

	token := "123456:secret-token"
	sender := &telegramSender{botToken: token, chatID: "chat", client: &http.Client{Transport: errorRoundTripper{err: io.ErrUnexpectedEOF}}}
	err := sender.Send(context.Background(), "Subject", "Body")
	if err == nil {
		t.Fatal("expected telegram send error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("telegram error leaked token: %v", err)
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("telegram error did not include redaction marker: %v", err)
	}
}

func TestTelegramSenderSendHandlesRejectedResponse(t *testing.T) {
	t.Parallel()

	sender := &telegramSender{botToken: "token", chatID: "chat", client: &http.Client{Transport: &recordingRoundTripper{status: http.StatusOK, body: `{"ok":false,"description":"chat not found"}`}}}
	err := sender.Send(context.Background(), "Subject", "Body")
	if err == nil || !strings.Contains(err.Error(), "chat not found") {
		t.Fatalf("expected rejected response error, got %v", err)
	}
}

type errorRoundTripper struct {
	err error
}

func (transport errorRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, &urlError{text: req.URL.String(), err: transport.err}
}

type urlError struct {
	text string
	err  error
}

func (err *urlError) Error() string { return "post " + err.text + ": " + err.err.Error() }

func (err *urlError) Unwrap() error { return err.err }

type recordingRoundTripper struct {
	status      int
	body        string
	method      string
	url         string
	contentType string
	requestBody string
}

func (transport *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	content, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	transport.method = req.Method
	transport.url = req.URL.String()
	transport.contentType = req.Header.Get("Content-Type")
	transport.requestBody = string(content)
	return &http.Response{
		StatusCode: transport.status,
		Body:       io.NopCloser(strings.NewReader(transport.body)),
		Header:     http.Header{},
	}, nil
}
