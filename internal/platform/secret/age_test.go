package secret

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
)

func TestEncryptDecryptRoundTripWithDerivedRecipient(t *testing.T) {
	t.Parallel()

	cfg := writeAgeTestConfig(t)
	encrypted, err := Encrypt("plain secret", cfg)
	if err != nil {
		t.Fatalf("Encrypt returned error: %v", err)
	}
	if !bytes.Contains(encrypted, []byte("-----BEGIN AGE ENCRYPTED FILE-----")) {
		t.Fatalf("expected armored age payload, got %q", string(encrypted))
	}

	decrypted, err := Decrypt(encrypted, cfg)
	if err != nil {
		t.Fatalf("Decrypt returned error: %v", err)
	}
	if decrypted != "plain secret" {
		t.Fatalf("decrypted = %q", decrypted)
	}
}

func TestEncryptDecryptRoundTripWithRecipientFileAndBinaryArmor(t *testing.T) {
	t.Parallel()

	cfg := writeAgeTestConfig(t)
	identity := readAgeIdentity(t, cfg.IdentityFile)
	recipientFile := filepath.Join(t.TempDir(), "recipients.txt")
	writeFile(t, recipientFile, "# comment\n\n"+identity.Recipient().String()+"\n")
	armor := false
	cfg.RecipientFile = recipientFile
	cfg.Armor = &armor

	encrypted, err := Encrypt("binary secret", cfg)
	if err != nil {
		t.Fatalf("Encrypt returned error: %v", err)
	}
	if bytes.Contains(encrypted, []byte("-----BEGIN AGE ENCRYPTED FILE-----")) {
		t.Fatalf("expected binary age payload")
	}

	decrypted, err := Decrypt(encrypted, cfg)
	if err != nil {
		t.Fatalf("Decrypt returned error: %v", err)
	}
	if decrypted != "binary secret" {
		t.Fatalf("decrypted = %q", decrypted)
	}
}

func TestDecryptFileReadsEncryptedPayload(t *testing.T) {
	t.Parallel()

	cfg := writeAgeTestConfig(t)
	encrypted, err := Encrypt("file secret", cfg)
	if err != nil {
		t.Fatalf("Encrypt returned error: %v", err)
	}
	path := filepath.Join(t.TempDir(), "secret.age")
	writeFile(t, path, string(encrypted))

	decrypted, err := DecryptFile(path, cfg)
	if err != nil {
		t.Fatalf("DecryptFile returned error: %v", err)
	}
	if decrypted != "file secret" {
		t.Fatalf("decrypted = %q", decrypted)
	}
}

func TestLoadRecipientsRejectsEmptyRecipientFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "recipients.txt")
	writeFile(t, path, "# no recipients\n\n")
	_, err := loadRecipients(path)
	if err == nil || !strings.Contains(err.Error(), "did not contain any recipients") {
		t.Fatalf("expected empty recipient error, got %v", err)
	}
}

func writeAgeTestConfig(t *testing.T) *config.ControllerSecretsConfig {
	t.Helper()

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate age identity: %v", err)
	}
	identityFile := filepath.Join(t.TempDir(), "identity.txt")
	writeFile(t, identityFile, identity.String()+"\n")
	return &config.ControllerSecretsConfig{Provider: "age", IdentityFile: identityFile}
}

func readAgeIdentity(t *testing.T, path string) *age.X25519Identity {
	t.Helper()

	content, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		t.Fatalf("read identity: %v", err)
	}
	identity, err := age.ParseX25519Identity(strings.TrimSpace(string(content)))
	if err != nil {
		t.Fatalf("parse identity: %v", err)
	}
	return identity
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
