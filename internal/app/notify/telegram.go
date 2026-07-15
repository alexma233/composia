package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
)

type telegramSender struct {
	botToken string
	chatID   string
	client   *http.Client
}

type telegramSendMessageRequest struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

type telegramSendMessageResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func newTelegramSender(cfg *config.ControllerTelegramNotificationConfig) (sender, error) {
	if cfg == nil {
		return nil, errors.New("telegram notification config is required")
	}
	return &telegramSender{
		botToken: strings.TrimSpace(cfg.BotToken),
		chatID:   strings.TrimSpace(cfg.ChatID),
		client:   &http.Client{},
	}, nil
}

func (sender *telegramSender) Send(ctx context.Context, subject, body string) error {
	payload, err := json.Marshal(telegramSendMessageRequest{
		ChatID: sender.chatID,
		Text:   subject + "\n\n" + body,
	})
	if err != nil {
		return fmt.Errorf("marshal telegram message: %w", err)
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", sender.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return sender.redactedErrorf("create telegram request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := sender.client.Do(req)
	if err != nil {
		return sender.redactedErrorf("send telegram request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return sender.redactedErrorf("read telegram response: %v", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return sender.redactedErrorf("telegram send failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var response telegramSendMessageResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return sender.redactedErrorf("decode telegram response: %v", err)
	}
	if !response.OK {
		return sender.redactedErrorf("telegram send rejected: %s", strings.TrimSpace(response.Description))
	}
	return nil
}

func (sender *telegramSender) redactedErrorf(format string, args ...any) error {
	return errors.New(redactTelegramToken(fmt.Sprintf(format, args...), sender.botToken))
}

func redactTelegramToken(text, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return text
	}
	return strings.ReplaceAll(text, token, "[REDACTED]")
}
