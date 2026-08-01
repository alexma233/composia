package repo

import (
	"errors"
	"testing"
)

func TestEncryptedFilePaths(t *testing.T) {
	if !IsEncryptedFilePath("config/.env.enc") || IsEncryptedFilePath("config/.env.ENC") {
		t.Fatal("encrypted file suffix must be exact")
	}
	if got := RuntimeFilePath("config/.env.enc"); got != "config/.env" {
		t.Fatalf("runtime path = %q", got)
	}
	if got := RuntimeFilePath("config/.env"); got != "config/.env" {
		t.Fatalf("plain runtime path = %q", got)
	}
	if IsValidEncryptedFilePath("config/.enc") || !HasEncryptedParent("config.enc/file") {
		t.Fatal("invalid encrypted path was accepted")
	}
	if got, err := NormalizePath(" config/.env.enc/. "); err != nil || got != "config/.env.enc" {
		t.Fatalf("normalized path = %q/%v", got, err)
	}
}

func TestValidateEncryptedFilePathsRejectsRuntimePairs(t *testing.T) {
	if err := ValidateEncryptedFilePaths([]string{"config/.env", "config/.env.enc"}); !errors.Is(err, ErrEncryptedFileConflict) {
		t.Fatalf("collision error = %v", err)
	}
	if err := ValidateEncryptedFilePaths([]string{"config/.env.enc", "config/.env"}); !errors.Is(err, ErrEncryptedFileConflict) {
		t.Fatalf("reverse collision error = %v", err)
	}
	if err := ValidateEncryptedFilePaths([]string{"config/.env.enc", "config/.env/file"}); !errors.Is(err, ErrEncryptedFileConflict) {
		t.Fatalf("runtime parent collision error = %v", err)
	}
	if err := ValidateEncryptedFilePaths([]string{"config.enc/file"}); !errors.Is(err, ErrRepoPathInvalid) {
		t.Fatalf("encrypted parent error = %v", err)
	}
}
