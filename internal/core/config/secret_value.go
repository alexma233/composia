package config

import (
	"fmt"
	"os"
	"strings"
)

func resolveInlineOrFileConfig(file *File) error {
	if file == nil {
		return nil
	}
	if file.Controller != nil {
		if err := resolveControllerInlineOrFileConfig(file.Controller); err != nil {
			return err
		}
	}
	if file.Agent != nil {
		resolved, err := resolveInlineOrFileValue(file.Agent.Token, file.Agent.TokenFile, "agent.token", true)
		if err != nil {
			return err
		}
		file.Agent.Token = resolved
	}
	return nil
}

func resolveControllerInlineOrFileConfig(controller *ControllerConfig) error {
	for index := range controller.Nodes {
		resolved, err := resolveInlineOrFileValue(controller.Nodes[index].Token, controller.Nodes[index].TokenFile, fmt.Sprintf("controller.nodes[%q].token", controller.Nodes[index].ID), true)
		if err != nil {
			return err
		}
		controller.Nodes[index].Token = resolved
	}
	for index := range controller.AccessTokens {
		resolved, err := resolveInlineOrFileValue(controller.AccessTokens[index].Token, controller.AccessTokens[index].TokenFile, fmt.Sprintf("controller.access_tokens[%q].token", controller.AccessTokens[index].Name), true)
		if err != nil {
			return err
		}
		controller.AccessTokens[index].Token = resolved
	}
	if controller.Git != nil && controller.Git.Auth != nil {
		resolved, err := resolveInlineOrFileValue(controller.Git.Auth.Token, controller.Git.Auth.TokenFile, "controller.git.auth.token", false)
		if err != nil {
			return err
		}
		controller.Git.Auth.Token = resolved
	}
	if controller.DNS != nil && controller.DNS.Cloudflare != nil {
		resolved, err := resolveInlineOrFileValue(controller.DNS.Cloudflare.APIToken, controller.DNS.Cloudflare.APITokenFile, "controller.dns.cloudflare.api_token", false)
		if err != nil {
			return err
		}
		controller.DNS.Cloudflare.APIToken = resolved
	}
	if controller.Updates != nil && controller.Updates.ForgeAuth != nil {
		if err := resolveForgeAuthConfig(controller.Updates.ForgeAuth.GitHub, "controller.updates.forge_auth.github.token"); err != nil {
			return err
		}
		if err := resolveForgeAuthConfig(controller.Updates.ForgeAuth.GitLab, "controller.updates.forge_auth.gitlab.token"); err != nil {
			return err
		}
		if err := resolveForgeAuthConfig(controller.Updates.ForgeAuth.Forgejo, "controller.updates.forge_auth.forgejo.token"); err != nil {
			return err
		}
	}
	if controller.Notifications != nil {
		if controller.Notifications.SMTP != nil {
			resolved, err := resolveInlineOrFileValue(controller.Notifications.SMTP.Password, controller.Notifications.SMTP.PasswordFile, "controller.notifications.smtp.password", false)
			if err != nil {
				return err
			}
			controller.Notifications.SMTP.Password = resolved
		}
		if controller.Notifications.Telegram != nil {
			resolved, err := resolveInlineOrFileValue(controller.Notifications.Telegram.BotToken, controller.Notifications.Telegram.BotTokenFile, "controller.notifications.telegram.bot_token", false)
			if err != nil {
				return err
			}
			controller.Notifications.Telegram.BotToken = resolved
		}
	}
	return nil
}

func resolveForgeAuthConfig(auth *ForgeAuthConfig, fieldPath string) error {
	if auth == nil {
		return nil
	}
	resolved, err := resolveInlineOrFileValue(auth.Token, auth.TokenFile, fieldPath, false)
	if err != nil {
		return err
	}
	auth.Token = resolved
	return nil
}

func resolveInlineOrFileValue(value, filePath, fieldPath string, required bool) (string, error) {
	value = strings.TrimSpace(value)
	filePath = strings.TrimSpace(filePath)
	if value != "" && filePath != "" {
		return "", fmt.Errorf("%s and %s_file must not both be set", fieldPath, fieldPath)
	}
	if filePath != "" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("read %s_file %q: %w", fieldPath, filePath, err)
		}
		value = strings.TrimSpace(string(content))
		if value == "" {
			return "", fmt.Errorf("%s_file %q is empty", fieldPath, filePath)
		}
	}
	if required && value == "" {
		return "", fmt.Errorf("%s or %s_file is required", fieldPath, fieldPath)
	}
	return value, nil
}
