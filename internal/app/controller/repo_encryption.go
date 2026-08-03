package controller

import (
	"errors"
	"fmt"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	secretutil "forgejo.alexma.top/alexma233/composia/internal/platform/secret"
)

var errSecretsNotConfigured = errors.New("controller secrets are not configured")

var errInvalidEncryptedPath = errors.New(".enc is reserved for encrypted files")

func repoFileContent(cfg *config.ControllerConfig, path, stored string) (string, error) {
	if repo.HasEncryptedParent(path) || repo.IsEncryptedFilePath(path) && !repo.IsValidEncryptedFilePath(path) {
		return "", errInvalidEncryptedPath
	}
	if !repo.IsEncryptedFilePath(path) {
		return stored, nil
	}
	if cfg == nil || cfg.Secrets == nil {
		return "", errSecretsNotConfigured
	}
	plaintext, err := secretutil.Decrypt([]byte(stored), cfg.Secrets)
	if err != nil {
		return "", fmt.Errorf("decrypt repo file %q: %w", path, err)
	}
	return plaintext, nil
}

func storedRepoFileContent(cfg *config.ControllerConfig, path, content string) (string, error) {
	normalized, err := repo.NormalizePath(path)
	if err != nil || normalized == "" || repo.HasEncryptedParent(normalized) || repo.IsEncryptedFilePath(normalized) && !repo.IsValidEncryptedFilePath(normalized) {
		return "", errInvalidEncryptedPath
	}
	path = normalized
	if !repo.IsEncryptedFilePath(path) {
		return content, nil
	}
	if cfg == nil || cfg.Secrets == nil {
		return "", errSecretsNotConfigured
	}
	ciphertext, err := secretutil.Encrypt(content, cfg.Secrets)
	if err != nil {
		return "", fmt.Errorf("encrypt repo file %q: %w", path, err)
	}
	return string(ciphertext), nil
}
