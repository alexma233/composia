package repo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeBundleExtraPathRejectsEscapingPaths(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", ".", "../secret", `/abs/secret`, `..\secret`} {
		if _, err := normalizeBundleExtraPath(name); err == nil {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
}

func TestNormalizeBundleExtraPathCleansSafePath(t *testing.T) {
	t.Parallel()

	got, err := normalizeBundleExtraPath("./demo/../demo/.composia-backup.json")
	if err != nil {
		t.Fatalf("normalize safe path: %v", err)
	}
	if got != "demo/.composia-backup.json" {
		t.Fatalf("unexpected normalized path %q", got)
	}
}

func TestStreamServiceBundleWithExtras(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	gitRun(t, repoDir, "init")
	serviceDir := filepath.Join(repoDir, "app")
	if err := os.MkdirAll(serviceDir, 0o750); err != nil {
		t.Fatalf("create service dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serviceDir, "compose.yaml"), []byte("services: {}\n"), 0o600); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-m", "service")
	revision, err := CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("current revision: %v", err)
	}

	var bundle bytes.Buffer
	if err := StreamServiceBundleWithExtras(context.Background(), repoDir, revision, "app", map[string]string{"app/.composia-extra.json": `{"ok":true}`}, &bundle); err != nil {
		t.Fatalf("stream bundle: %v", err)
	}
	files := readGzipTarFiles(t, bundle.Bytes())
	if got := files["app/compose.yaml"]; got != "services: {}\n" {
		t.Fatalf("compose.yaml = %q", got)
	}
	if got := files["app/.composia-extra.json"]; got != `{"ok":true}` {
		t.Fatalf("extra file = %q", got)
	}
}

func TestStreamServiceBundleWithExtrasRejectsUnsafeExtraPath(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	gitRun(t, repoDir, "init")
	if err := os.MkdirAll(filepath.Join(repoDir, "app"), 0o750); err != nil {
		t.Fatalf("create service dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "app", "compose.yaml"), []byte("services: {}\n"), 0o600); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-m", "service")
	revision, err := CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("current revision: %v", err)
	}

	var bundle bytes.Buffer
	if err := StreamServiceBundleWithExtras(context.Background(), repoDir, revision, "app", map[string]string{"../secret": "nope"}, &bundle); err == nil {
		t.Fatalf("expected unsafe extra path error")
	}
}

func readGzipTarFiles(t *testing.T, data []byte) map[string]string {
	t.Helper()

	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open gzip reader: %v", err)
	}
	defer func() {
		if err := gzipReader.Close(); err != nil {
			t.Fatalf("close gzip reader: %v", err)
		}
	}()

	files := map[string]string{}
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read tar entry: %v", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		body, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("read tar file %q: %v", header.Name, err)
		}
		files[header.Name] = string(body)
	}
	return files
}
