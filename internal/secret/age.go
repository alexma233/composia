package secret

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
	"forgejo.alexma.top/alexma233/composia/internal/config"
)

func DecryptFile(filePath string, cfg *config.ControllerSecretsConfig) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read encrypted secret file %q: %w", filePath, err)
	}
	return Decrypt(content, cfg)
}

func Decrypt(content []byte, cfg *config.ControllerSecretsConfig) (string, error) {
	identities, err := loadIdentities(cfg.IdentityFile)
	if err != nil {
		return "", err
	}
	reader, err := openAgeReader(content)
	if err != nil {
		return "", err
	}
	decrypted, err := age.Decrypt(reader, identities...)
	if err != nil {
		return "", fmt.Errorf("decrypt secret payload: %w", err)
	}
	plaintext, err := io.ReadAll(decrypted)
	if err != nil {
		return "", fmt.Errorf("read decrypted secret payload: %w", err)
	}
	return string(plaintext), nil
}

func Encrypt(content string, cfg *config.ControllerSecretsConfig) ([]byte, error) {
	recipients, err := loadRecipients(cfg.RecipientFile)
	if err != nil {
		return nil, err
	}
	buffer := bytes.Buffer{}
	writer := io.Writer(&buffer)
	var armorWriter io.WriteCloser
	if cfg.Armor == nil || *cfg.Armor {
		armorWriter = armor.NewWriter(&buffer)
		writer = armorWriter
	}
	encryptedWriter, err := age.Encrypt(writer, recipients...)
	if err != nil {
		if armorWriter != nil {
			_ = armorWriter.Close()
		}
		return nil, fmt.Errorf("create age encrypt writer: %w", err)
	}
	if _, err := io.WriteString(encryptedWriter, content); err != nil {
		_ = encryptedWriter.Close()
		if armorWriter != nil {
			_ = armorWriter.Close()
		}
		return nil, fmt.Errorf("write encrypted secret payload: %w", err)
	}
	if err := encryptedWriter.Close(); err != nil {
		if armorWriter != nil {
			_ = armorWriter.Close()
		}
		return nil, fmt.Errorf("close age encrypt writer: %w", err)
	}
	if armorWriter != nil {
		if err := armorWriter.Close(); err != nil {
			return nil, fmt.Errorf("close age armor writer: %w", err)
		}
	}
	return buffer.Bytes(), nil
}

func openAgeReader(content []byte) (io.Reader, error) {
	trimmed := bytes.TrimSpace(content)
	if bytes.HasPrefix(trimmed, []byte("-----BEGIN AGE ENCRYPTED FILE-----")) {
		return armor.NewReader(bytes.NewReader(content)), nil
	}
	return bytes.NewReader(content), nil
}

func loadIdentities(filePath string) ([]age.Identity, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read age identity file %q: %w", filePath, err)
	}
	identities, err := age.ParseIdentities(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("parse age identities from %q: %w", filePath, err)
	}
	return identities, nil
}

func loadRecipients(filePath string) ([]age.Recipient, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read age recipient file %q: %w", filePath, err)
	}
	lines := strings.Split(string(content), "\n")
	recipients := make([]age.Recipient, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		recipient, err := age.ParseX25519Recipient(line)
		if err != nil {
			return nil, fmt.Errorf("parse age recipient %q from %q: %w", line, filePath, err)
		}
		recipients = append(recipients, recipient)
	}
	if len(recipients) == 0 {
		return nil, fmt.Errorf("age recipient file %q did not contain any recipients", filePath)
	}
	return recipients, nil
}
