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
		for index := range file.Agent.ControllerHeaders {
			resolved, err := resolveInlineOrFileValue(file.Agent.ControllerHeaders[index].Value, file.Agent.ControllerHeaders[index].ValueFile, fmt.Sprintf("agent.controller_headers[%q].value", file.Agent.ControllerHeaders[index].Name), false)
			if err != nil {
				return err
			}
			file.Agent.ControllerHeaders[index].Value = resolved
		}
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
	if controller.CloudflareTunnel != nil {
		resolved, err := resolveInlineOrFileValue(controller.CloudflareTunnel.APIToken, controller.CloudflareTunnel.APITokenFile, "controller.cloudflare_tunnel.api_token", false)
		if err != nil {
			return err
		}
		controller.CloudflareTunnel.APIToken = resolved
	}
	if controller.DNS != nil && controller.DNS.AliDNS != nil {
		if err := resolveAliDNSInlineOrFileConfig(controller.DNS.AliDNS); err != nil {
			return err
		}
	}
	if controller.DNS != nil && controller.DNS.DNSPod != nil {
		if err := resolveDNSPodInlineOrFileConfig(controller.DNS.DNSPod); err != nil {
			return err
		}
	}
	if controller.DNS != nil && controller.DNS.Route53 != nil {
		if err := resolveRoute53InlineOrFileConfig(controller.DNS.Route53); err != nil {
			return err
		}
	}
	if controller.DNS != nil && controller.DNS.HuaweiCloud != nil {
		if err := resolveHuaweiCloudInlineOrFileConfig(controller.DNS.HuaweiCloud); err != nil {
			return err
		}
	}
	if controller.Updates != nil && controller.Updates.ForgeAuth != nil {
		if err := resolveForgeAuthConfigs(controller.Updates.ForgeAuth.GitHub, "controller.updates.forge_auth.github"); err != nil {
			return err
		}
		if err := resolveForgeAuthConfigs(controller.Updates.ForgeAuth.GitLab, "controller.updates.forge_auth.gitlab"); err != nil {
			return err
		}
		if err := resolveForgeAuthConfigs(controller.Updates.ForgeAuth.Forgejo, "controller.updates.forge_auth.forgejo"); err != nil {
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

func resolveAliDNSInlineOrFileConfig(cfg *AliDNSConfig) error {
	var err error
	if cfg.AccessKeyID, err = resolveInlineOrFileValue(cfg.AccessKeyID, cfg.AccessKeyIDFile, "controller.dns.alidns.access_key_id", false); err != nil {
		return err
	}
	if cfg.AccessKeySecret, err = resolveInlineOrFileValue(cfg.AccessKeySecret, cfg.AccessKeySecretFile, "controller.dns.alidns.access_key_secret", false); err != nil {
		return err
	}
	if cfg.SecurityToken, err = resolveInlineOrFileValue(cfg.SecurityToken, cfg.SecurityTokenFile, "controller.dns.alidns.security_token", false); err != nil {
		return err
	}
	return nil
}

func resolveDNSPodInlineOrFileConfig(cfg *DNSPodConfig) error {
	var err error
	if cfg.SecretID, err = resolveInlineOrFileValue(cfg.SecretID, cfg.SecretIDFile, "controller.dns.dnspod.secret_id", false); err != nil {
		return err
	}
	if cfg.SecretKey, err = resolveInlineOrFileValue(cfg.SecretKey, cfg.SecretKeyFile, "controller.dns.dnspod.secret_key", false); err != nil {
		return err
	}
	if cfg.SessionToken, err = resolveInlineOrFileValue(cfg.SessionToken, cfg.SessionTokenFile, "controller.dns.dnspod.session_token", false); err != nil {
		return err
	}
	return nil
}

func resolveRoute53InlineOrFileConfig(cfg *Route53DNSConfig) error {
	var err error
	if cfg.AccessKeyID, err = resolveInlineOrFileValue(cfg.AccessKeyID, cfg.AccessKeyIDFile, "controller.dns.route53.access_key_id", false); err != nil {
		return err
	}
	if cfg.SecretAccessKey, err = resolveInlineOrFileValue(cfg.SecretAccessKey, cfg.SecretAccessKeyFile, "controller.dns.route53.secret_access_key", false); err != nil {
		return err
	}
	if cfg.SessionToken, err = resolveInlineOrFileValue(cfg.SessionToken, cfg.SessionTokenFile, "controller.dns.route53.session_token", false); err != nil {
		return err
	}
	return nil
}

func resolveHuaweiCloudInlineOrFileConfig(cfg *HuaweiCloudDNSConfig) error {
	var err error
	if cfg.AccessKeyID, err = resolveInlineOrFileValue(cfg.AccessKeyID, cfg.AccessKeyIDFile, "controller.dns.huaweicloud.access_key_id", false); err != nil {
		return err
	}
	if cfg.SecretAccessKey, err = resolveInlineOrFileValue(cfg.SecretAccessKey, cfg.SecretAccessKeyFile, "controller.dns.huaweicloud.secret_access_key", false); err != nil {
		return err
	}
	return nil
}

func resolveForgeAuthConfigs(auths ForgeAuthConfigs, fieldPath string) error {
	for index := range auths {
		resolved, err := resolveInlineOrFileValue(auths[index].Token, auths[index].TokenFile, fmt.Sprintf("%s[%d].token", fieldPath, index), false)
		if err != nil {
			return err
		}
		auths[index].Token = resolved
	}
	return nil
}

func resolveInlineOrFileValue(value, filePath, fieldPath string, required bool) (string, error) {
	value = strings.TrimSpace(value)
	filePath = strings.TrimSpace(filePath)
	if value != "" && filePath != "" {
		return "", fmt.Errorf("%s and %s_file must not both be set", fieldPath, fieldPath)
	}
	if filePath != "" {
		content, err := os.ReadFile(filePath) //nolint:gosec
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
