package controller

import (
	"os"
	"path/filepath"
	"testing"

	"forgejo.alexma.top/alexma233/composia/internal/store"
)

func openControllerTestDB(t *testing.T) *store.DB {
	t.Helper()

	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}
