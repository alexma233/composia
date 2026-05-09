package config

import (
	"fmt"
	"strings"

	corenotify "forgejo.alexma.top/alexma233/composia/internal/core/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

const (
	SMTPEncryptionNone     = "none"
	SMTPEncryptionSTARTTLS = "starttls"
	SMTPEncryptionSSLTLS   = "ssl_tls"
)

type ControllerNotificationsConfig struct {
	SMTP     *ControllerSMTPNotificationConfig     `yaml:"smtp"`
	Telegram *ControllerTelegramNotificationConfig `yaml:"telegram"`
}

type ControllerSMTPNotificationConfig struct {
	Enabled      *bool    `yaml:"enabled"`
	Host         string   `yaml:"host"`
	Port         int      `yaml:"port"`
	Encryption   string   `yaml:"encryption"`
	Username     string   `yaml:"username"`
	Password     string   `yaml:"password"`
	PasswordFile string   `yaml:"password_file"`
	From         string   `yaml:"from"`
	To           []string `yaml:"to"`
	On           []string `yaml:"on"`
	TaskSources  []string `yaml:"task_sources"`
}

type ControllerTelegramNotificationConfig struct {
	Enabled      *bool    `yaml:"enabled"`
	BotToken     string   `yaml:"bot_token"`
	BotTokenFile string   `yaml:"bot_token_file"`
	ChatID       string   `yaml:"chat_id"`
	On           []string `yaml:"on"`
	TaskSources  []string `yaml:"task_sources"`
}

func (cfg *ControllerSMTPNotificationConfig) IsEnabled() bool {
	return cfg != nil && (cfg.Enabled == nil || *cfg.Enabled)
}

func (cfg *ControllerTelegramNotificationConfig) IsEnabled() bool {
	return cfg != nil && (cfg.Enabled == nil || *cfg.Enabled)
}

func validateControllerNotifications(cfg *ControllerNotificationsConfig) error {
	if cfg == nil {
		return nil
	}
	if err := validateSMTPNotifications(cfg.SMTP); err != nil {
		return err
	}
	if err := validateTelegramNotifications(cfg.Telegram); err != nil {
		return err
	}
	return nil
}

func validateSMTPNotifications(cfg *ControllerSMTPNotificationConfig) error {
	if cfg == nil {
		return nil
	}
	if err := validateNotificationEvents("controller.notifications.smtp.on", cfg.On); err != nil {
		return err
	}
	if err := validateNotificationTaskSources("controller.notifications.smtp.task_sources", cfg.TaskSources); err != nil {
		return err
	}
	if !cfg.IsEnabled() {
		return nil
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("controller.notifications.smtp.host is required")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("controller.notifications.smtp.port must be between 1 and 65535")
	}
	switch NormalizeSMTPEncryption(cfg.Encryption) {
	case SMTPEncryptionNone, SMTPEncryptionSTARTTLS, SMTPEncryptionSSLTLS:
	default:
		return fmt.Errorf("controller.notifications.smtp.encryption must be one of none, starttls, or ssl_tls")
	}
	if strings.TrimSpace(cfg.From) == "" {
		return fmt.Errorf("controller.notifications.smtp.from is required")
	}
	if len(cfg.To) == 0 {
		return fmt.Errorf("controller.notifications.smtp.to must contain at least one recipient")
	}
	for index, recipient := range cfg.To {
		if strings.TrimSpace(recipient) == "" {
			return fmt.Errorf("controller.notifications.smtp.to[%d] must not be empty", index)
		}
	}
	return nil
}

func validateTelegramNotifications(cfg *ControllerTelegramNotificationConfig) error {
	if cfg == nil {
		return nil
	}
	if err := validateNotificationEvents("controller.notifications.telegram.on", cfg.On); err != nil {
		return err
	}
	if err := validateNotificationTaskSources("controller.notifications.telegram.task_sources", cfg.TaskSources); err != nil {
		return err
	}
	if !cfg.IsEnabled() {
		return nil
	}
	if strings.TrimSpace(cfg.BotToken) == "" {
		return fmt.Errorf("controller.notifications.telegram.bot_token is required")
	}
	if strings.TrimSpace(cfg.ChatID) == "" {
		return fmt.Errorf("controller.notifications.telegram.chat_id is required")
	}
	return nil
}

func NormalizeSMTPEncryption(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return SMTPEncryptionSTARTTLS
	}
	return value
}

func validateNotificationEvents(path string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(strings.ToLower(value))
		if normalized == "" {
			return fmt.Errorf("%s[] must not be empty", path)
		}
		if !corenotify.IsValidEventType(normalized) {
			return fmt.Errorf("%s[%q] is not a supported notification event", path, normalized)
		}
		if _, exists := seen[normalized]; exists {
			return fmt.Errorf("%s[%q] is duplicated", path, normalized)
		}
		seen[normalized] = struct{}{}
	}
	return nil
}

func validateNotificationTaskSources(path string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(strings.ToLower(value))
		if normalized == "" {
			return fmt.Errorf("%s[] must not be empty", path)
		}
		source := task.Source(normalized)
		switch source {
		case task.SourceWeb, task.SourceCLI, task.SourceOthers, task.SourceSchedule, task.SourceSystem:
		default:
			return fmt.Errorf("%s[%q] is not a supported task source", path, normalized)
		}
		if _, exists := seen[normalized]; exists {
			return fmt.Errorf("%s[%q] is duplicated", path, normalized)
		}
		seen[normalized] = struct{}{}
	}
	return nil
}
